## 基本块分类

我们将形式化定义 **块分类体系** 以及核心集合 **$\mathcal{D}_{Set}$**。

为了在理论上严谨地描述这些定义，我们需要引入几个辅助符号：
*   $\mathbb{S}$：所有可能的具体栈状态（Concrete Stack States）的集合。
*   $Exec(B, S)$：在栈状态 $S$ 下执行块 $B$ 后得到的跳转目标地址集合（对于 JUMP 为单元素，对于 JUMPI 可能为双元素，对于 STOP/RETURN 为空）。

以下是 **Live_D** 体系的形式化定义：

---

### 1. 块分类定义 (Block Taxonomy)

我们将所有控制流相关的基本块 $\mathbb{B}$ 划分为三个互斥的集合：$\mathbf{I}$ (Immediate), $\mathbf{A}$ (Any-Test), $\mathbf{D}$ (Dependent)。

#### 定义 1.1：I 块 (Syntactic Immediate Block)
**I 块**是指那些仅通过语法分析即可确定跳转目标的块，不依赖于任何栈输入。
$$ B \in \mathbf{I} \iff \text{Opcode}(B)_{last} \in \{\text{PUSH} \cdot \text{JUMP}, \ \text{PUSH} \cdot \text{PUSH} \cdot \text{JUMPI}\} $$
*这意味着跳转目标是作为立即数硬编码在指令流中的。*

#### 定义 1.2：A 块 (Any-Test / Semantically Constant Block)
**A 块**是指虽然语法上不是 I 块（可能包含 POP, SWAP, ADD 等操作），但在语义上其控制流输出具有**状态无关性（State Independence）**。即无论输入栈是什么，跳转目标恒定。

$$ B \in \mathbf{A} \iff (B \notin \mathbf{I}) \land (\exists T, \forall S \in \mathbb{S}, Exec(B, S) = \{T\}) $$
*注：对于 JUMPI，这意味着条件恒为真或恒为假，或者跳转目标恒定。在符号执行中，这意味着所有栈变量被消去，只剩下常量。*

#### 定义 1.3：D 块 (Locally Unsolvable / Dependent Block)
**D 块**是指其控制流后继强依赖于输入栈状态的具体值的块。这是分析的难点，也是我们需要应用 Masking 策略的地方。

$$ B \in \mathbf{D} \iff B \notin \mathbf{I} \land B \notin \mathbf{A} $$
等价地，我们可以说：
$$ B \in \mathbf{D} \iff \exists S_1, S_2 \in \mathbb{S}, \text{ s.t. } Exec(B, S_1) \neq Exec(B, S_2) $$
*即：存在至少两种不同的栈状态，导致块 $B$ 跳转到不同的地方。*

---

### 2. 形式化集合 $\mathcal{D}_{Set}$

基于上述分类，我们定义您提到的 **$\mathcal{D}_{Set}$**：

$$ \mathcal{D}_{Set} = \{ B \in \mathbb{B}_{discovered} \mid B \in \mathbf{D} \} = \{ D_1, D_2, \dots, D_n \} $$

这是一个动态集合，随着 CFG 探索的进行，可能会发现新的 D 块。


### 3. Live Masking 下的 D 块解析

为了配合我们之前的讨论（通过 Masking 解决鸡生蛋问题），我们需要定义 D 块在特定 Mask 下的行为。

定义 **局部活性掩码 (Local Live Mask)** 为 $M$。
定义 **抽象求值函数** $Eval(D_i, M)$。

对于任意 $D_i \in \mathcal{D}_{Set}$：
1.  **可消解 (Resolvable)**: 如果在掩码 $M$ 的约束下，$D_i$ 的跳转目标变为常量（即 Mask 保留了所有决定控制流的变量），则称 $D_i$ 在 $M$ 下退化为 A 块。
2.  **不可消解 (Unresolvable)**: 如果在掩码 $M$ 的约束下（即忽略了某些栈位），$D_i$ 的跳转目标变为 $\top$（无法确定），则称 $D_i$ 仍处于“未决状态”。

**发现 S 的充要条件**：
要通过 $D_i$ 发现目标块 $S$，必须寻找一个掩码 $M$，使得：
$$ Exec(D_i, S_{in} \land M) \to S $$
其中 $S_{in}$ 是到达 $D_i$ 的路径栈。


在这个形式化体系中：
*   **I 块** 和 **A 块** 是“平凡的”，它们不会阻碍路径探索，可以直接扩展。
*   **D 块 ($\mathcal{D}_{Set}$)** 是“前沿（Frontier）”上的阻碍点。
*   我们的算法目标就是：对于 $\mathcal{D}_{Set}$ 中的每个 $D_i$，找到一个最小的 Mask $M$，使其变为**可消解**，并指向新的未知区域（包含 $S$）。
