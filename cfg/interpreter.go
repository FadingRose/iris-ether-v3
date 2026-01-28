package cfg

import (
	"fadingrose/iris-ether-v3/vm"
)

type Interpreter struct {
	st   *Stack
	path *Path

	maxStackSize int
}

func NewInterpreter(path *Path, maxStackSize int) *Interpreter {
	if maxStackSize <= 0 || maxStackSize > 1024 {
		maxStackSize = 1024
	}

	return &Interpreter{
		st:           NewStack(),
		path:         path,
		maxStackSize: maxStackSize,
	}
}

func (i *Interpreter) SetStack(st *Stack) {
	i.st = st
}

func (i *Interpreter) Run() (*Reg, *Stack, map[*BasicBlock]*Snapshot, error) {
	var last *Reg
	var err error
	snapshots := make(map[*BasicBlock]*Snapshot)
	for _, bb := range i.path.data {
		// Snapshot before executing the block for liveness analysis.
		snapshots[bb] = i.st.Snapshot()

		for _, reg := range bb.regs() {
			if err != nil {
				return nil, nil, nil, err
			}

			last = reg

			if reg.inst.op == vm.POP {
				_, err = i.st.Pop()
				continue
			}
			if reg.inst.op.IsDup() {
				// Record dup source so dependency/liveness can trace through duplicates.
				dupFrom, _ := i.st.GetDupN(reg.inst.op.DupNum())
				reg.dupFrom = dupFrom
				i.st.Push(reg)
				continue
			}
			if reg.inst.op.IsSwap() {
				err = i.st.SwapN(reg.inst.op.SwapNum())
				continue
			}

			reg.params, err = i.st.PopN(reg.inst.paramSize)
			reg.val = reg.value()

			if reg.inst.op == vm.JUMP || reg.inst.op == vm.JUMPI {
				// JUMP/JUMPI consume params but do not push results.
				continue
			}
			i.st.PushN(reg, reg.inst.pushbackSize)
		}
	}

	if len(i.st.items) > i.maxStackSize {
		return nil, nil, nil, ErrStackOverflow
	}

	if err != nil {
		return nil, nil, nil, err
	}

	if last.inst.op.IsHalt() {
		return nil, i.st, snapshots, nil
	}

	if last.inst.op.IsPop() {
		if len(i.st.items) == 0 {
			return nil, nil, nil, ErrStackUnderflow
		}
		last = i.st.items[len(i.st.items)-1]
	}

	return last, i.st, snapshots, nil
}
