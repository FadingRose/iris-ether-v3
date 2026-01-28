# iris-ether: CFG Recovery Summary (LLM/Agent Intro)

## Purpose
This document is a single, concise summary of the paper draft and design ideas behind **iris-ether**, intended as onboarding for agents working in this codebase.

---

## 1) Task Definition: EVM Bytecode CFG Recovery

**Input**: EVM bytecode (typically the runtime bytecode; if bytecode contains partitions split by `INVALID`, each partition is treated as an independent memory layout with its own entry at PC=0).

**Output**: A **complete** Control Flow Graph (CFG):
- all reachable basic blocks, and
- all valid edges between blocks,
- **no missing reachable paths** and **no spurious edges**.

**Core difficulty**: EVM jump targets (`JUMP`, `JUMPI`) are **stack values** computed at runtime, so control flow is **path/context dependent**.

Formally:

- `E(B)`: Entry Path Set, all valid paths from entry to `B`.
- `Σ(B)`: Future Path Set, all valid continuations from `B` under all contexts in `E(B)`.
- The key problem is to recover `Σ(B)` for **context-dependent blocks** (D-Blocks).

---

## 2) Key Challenges

1) **Context-Aware Dependency**
- A block’s valid successors depend on the stack values produced by the exact path taken to reach it.
- Ignoring context creates spurious edges that pollute downstream CFG regions.

2) **Unstructured Control Flow / Loops**
- EVM bytecode often forms irreducible or overlapping loops.
- No reliable loop invariants; stack height/content is inconsistent across entries.
- Naive exploration explodes or never converges.

3) **Limits of Traditional Symbolic Execution**
- Practical SE uses local predicates and over-approximates.
- This introduces false edges and unreachable subgraphs.

---

## 3) Core Insight

CFG recovery is **path enumeration in a cyclic graph**, but the exploration must be **context aware** and terminate. The solution is an explicit-state exploration with a **stack abstraction** that only preserves information needed to resolve future jumps.

---

## 4) Stack Model (Symbolic but Minimal)

We use a simplified symbolic stack for control-flow only (not full EVM state).

- **Stack shape**: LIFO, max depth 1024. `S#0` is top.
- **Value domain**: each element is either
  - **Concrete 256-bit value**, or
  - **⊤ (unknown)** for external/undecidable values.
- Each symbolic element also retains a **source trace** and can expand to an **expression tree**.

Example:
```
PUSH1 0x01
PUSH1 0x02
ADD
```
The top of stack is a symbolic item `#0` with expression `0x01 + 0x02 = 0x03` (concrete).

This expression tree is used to identify which prior stack slots actually influence a dynamic jump.

---

## 5) Block Taxonomy (Resolution Frontier)

We classify basic blocks by how their jump targets are resolved:

- **I-Block (Immediate)**: target is syntactically obvious (e.g., `PUSH <addr>; JUMP`).
- **A-Block (Semantically Constant)**: target is constant in practice despite not being syntactically immediate.
- **D-Block (Context Dependent)**: target depends on input stack state and path.

**Resolution Frontier** = set of all D-Blocks. These are the only blocks requiring deep context tracking.

---

## 6) Per-D-Block Mask (M) and Dependency Derivation

**Mask is per D-Block.**

When a path first reaches a D-Block `D1`, we execute the path with a symbolic interpreter and compute the jump target expression. From the expression tree, we derive a **dependency set** of stack slots that influence the target.

Example path:
```
0 -> A -> B -> C -> D1
```
Suppose the target expression expands to:
```
0x123 == (A#1) AND (B#3)
```
and `(B#3)` depends on `C#5`.

Dependency set for `D1`: `{A#1, B#3, C#5}`. The **mask for D1** is the projection of these influencing indices.

---

## 7) Pruning Rule (Per D-Block)

When revisiting an earlier block (e.g., `A`) on another path:
- If the stack matches the **mask-relevant indices** for `D1`, we **can try to prune** this path as equivalent for resolving `D1`.
- This pruning is local to D1 and safe for D1’s target resolution, but may hide information needed for **other D-Blocks** downstream.

---

## 8) Recovery (Conflict-Driven Refinement)

Pruning can lose precision for later D-Blocks. Recovery strategy:

1) **Log pruned paths** with their masked state and context.
2) If a later D-Block `D2` remains unresolved or missing expected targets, check whether any pruned paths could reach `D2`.
3) **Re-explore** those pruned paths with a refined mask that includes the newly required dependency indices.

This is a **lazy refinement** strategy: start small, then expand masks only where needed.

---

## 9) Algorithm Outline (High Level)

1) Start at entry block (PC=0).
2) DFS/worklist explore reachable paths.
3) For each block:
   - If I-Block or A-Block: resolve successors directly.
   - If D-Block: execute path, compute target expression, derive mask, resolve successor.
4) Use per-D-Block mask equivalence to prune repeated contexts.
5) If resolution stalls or targets remain missing, trigger recovery by replaying pruned paths with refined masks.
6) Iterate to fixpoint.

Termination is guaranteed because:
- The bytecode is finite.
- Stack depth is bounded.
- Mask refinements are finite and monotonic.

---

## 10) Loop Handling Philosophy

- Loops are treated as cyclic path enumeration; no reliance on classical loop invariants.
- Pruning controls infinite exploration by collapsing contexts under masks.
- Recovery restores precision only where pruning proved too aggressive.

---

## 11) Evaluation Summary (Paper Draft)

Evaluation focuses on:
- **Block coverage**: fraction of minimal basic blocks recovered.
- **Probe coverage**: function entry/exit instrumentation to verify realistic execution paths.
- **Scalability**: near-linear growth vs CFG size in practice.

---

## 12) Key Takeaways

- CFG recovery must be **context aware**, but full context is intractable.
- Track **only stack elements that influence dynamic jumps**, per D-Block.
- Use **per-D-Block masks + pruning + recovery** to balance completeness and termination.
- Exploration is a **fixpoint over abstract states**, with guaranteed convergence.

---

## 13) Terms / Glossary

- **D-Block**: a block whose successors depend on the input stack (dynamic jump targets).
- **Mask (M)**: a subset of stack indices relevant to a particular D-Block’s target resolution.
- **Masked equivalence**: two states are equivalent if masked indices match.
- **Recovery**: replay of pruned paths when later D-Blocks require additional dependencies.

