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

## 11) Current Implementation Notes (cfg/*)

This is a summary of what the code currently does (not the full paper design):

- **CFG construction** (`cfg/cfg.go`): splits bytecode into basic blocks, tracks `validDestMap` per INVALID-delimited partition, and connects linear fallthrough via `next`.
- **Interpreter** (`cfg/interpreter.go`): executes a *path* (sequence of blocks), building `Reg` nodes, taking a snapshot before each block, and evaluating only a small set of opcodes concretely (`PUSH`, `DUP`, `ADD`, `SUB`, `SHL/SHR`, `AND`, `SLOAD`). Unknown ops yield `nil` values.
- **StateMap (Jump Vector)**: deduplicates exploration by `(block, stack hash, dest)` where the stack hash is **value-only** (no provenance). This is intentional and keeps the abstraction coarse.
- **D-path handling** (`executeDPath`):
  - If the jump dest does **not** depend on input stack (liveness flag false), run the **any-stack test** to confirm the dest is constant, then mark as `MustJump` and add to `FixedD`.
  - If it **does** depend on input stack, record **liveness slots** backward along the path and add to `SumD`.
- **Liveness** (`cfg/live.go`): each block can hold multiple `live` masks (different input slots lead to different D-blocks). Matching is position-based (top-of-stack height) using instruction identity strings from snapshots.
- **Worklist driver** (`build`):
  - Always prunes **leading halt**, **liveness-matched paths**, and some loop paths (if header has no live mask).
  - When the immediate path list is empty, it **only picks from `SumD`** currently.
  - `Liveness.PrunedPaths()` and `LoopPruned` are stored but replay is not yet wired (commented out).

---

## 12) Evaluation Summary (Paper Draft)

Evaluation focuses on:
- **Block coverage**: fraction of minimal basic blocks recovered.
- **Probe coverage**: function entry/exit instrumentation to verify realistic execution paths.
- **Scalability**: near-linear growth vs CFG size in practice.

---

## 13) Key Takeaways

- CFG recovery must be **context aware**, but full context is intractable.
- Track **only stack elements that influence dynamic jumps**, per D-Block.
- Use **per-D-Block masks + pruning + recovery** to balance completeness and termination.
- Exploration is a **fixpoint over abstract states**, with guaranteed convergence.

---

## 14) Open Problems / Next Steps (Implementation)

The paper-mask mechanism is **not fully implemented** yet. Today, liveness is the practical stand-in, and the core challenge is **how to replay pruned paths** without exploding search.

Desired replay sources (future work):

1) **SumD** (context-aware paths).
2) **Liveness-pruned paths** (masked equivalence).
3) **Loop-pruned paths** (paths cut due to loop handling).

Loop handling remains the hardest part: we want to avoid **magic numbers** (e.g., fixed loop depth). The current fixpoint window is itself a magic number. The target is a principled replay strategy that removes this tuning and still converges.

Notes from `TestStagnation` (comments in `cfg/builder_test.go`):
- Without picking from pruned liveness, coverage drops sharply even though runtime is very fast.
- Picking from pruned liveness improves coverage substantially; **larger window size increases coverage** (at some time cost), while still far faster than the baseline.

---

## 15) Formal View: Liveness, Masks, and Loops (Design Lens)

This section provides a formal lens for current liveness behavior and a direction for a principled replay strategy.

### 15.1 Definitions

Let CFG = (B, E), entry block `b0`.

- A **path** `p = <b0, b1, …, bk>` is a sequence of blocks.
- The interpreter defines a **stack transformer**:
  - `Exec(p, S_in) -> (S_out, lastReg)`
  - `lastReg` is the final executed reg; for JUMP/JUMPI it is the jump-dest reg (`params[0]`).
- For a D-block `D`, define `Dest_D(S_in)` as the resolved jump destination under input stack `S_in`.

### 15.2 Liveness as a Dependency Mask

`Snapshot.Live(lastReg)` computes a **dependency set** of stack slots (top-based height) that influence `lastReg`.

We can view each liveness instance as a **mask** `M` over stack slots:
- `π_M(S)` is the projection of stack `S` to the slots in `M`.
- In current code, `live.slots` stores `(pos -> instruction string)` which is a *signature* for `π_M(S)`.

**Sound pruning criterion (for a given D-block):**
If two input stacks agree on `π_M(S)` then they are equivalent for resolving `Dest_D`.

### 15.3 Pruning as Abstract-State Fixpoint

Define an **abstract state** at block `b`:
```
A = (b, π_M(S_in))
```

Exploration is a monotone process over abstract states. A fixpoint is reached when no new abstract state appears. This removes the need for a window-size heuristic once replay is driven by abstraction changes.

### 15.4 Replay via Mask Refinement

When a new liveness mask `M'` refines a previous mask `M` (i.e., adds new slots), previously pruned paths may become distinguishable.

**Replay rule:**
Replay any pruned path whose stored signature `π_M(S_in)` matched before but whose refined signature `π_M'(S_in)` is now new.

This produces a **deterministic, finite** replay sequence because masks only grow.

### 15.5 Loop Handling Without Magic Depth

Treat a loop header (or SCC entry) `H` as a join point with a set of abstract inputs:
```
AbstractIn(H) = { π_M(S_in) }
```

When a path re-enters `H`:
- If its `π_M(S_in)` is already in `AbstractIn(H)`, prune (loop does not add a new abstract state).
- Otherwise, record and continue.

As masks refine, loop entries can be revisited only when their abstract signatures change. This eliminates the need for a fixed loop-depth limit.

### 15.6 Replay Sources as One Mechanism

The three replay sources (SumD, liveness-pruned, loop-pruned) can be unified:
- Store pruned paths with their abstract signatures.
- Re-enqueue only when the relevant signature becomes distinguishable under refined masks.

This aligns the implementation with the core idea: **enumerate only meaningful future paths** while avoiding magic numbers.

---

## 16) Terms / Glossary

- **D-Block**: a block whose successors depend on the input stack (dynamic jump targets).
- **Mask (M)**: a subset of stack indices relevant to a particular D-Block’s target resolution.
- **Masked equivalence**: two states are equivalent if masked indices match.
- **Recovery**: replay of pruned paths when later D-Blocks require additional dependencies.
