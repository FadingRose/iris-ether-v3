package cfg

import (
	"fadingrose/iris-ether-v3/vm"
	"fmt"

	"github.com/holiman/uint256"
)

type instructions []*instruction

func (inss *instructions) Iterator() *InstructionIterator {
	return &InstructionIterator{
		inss:  *inss,
		index: -1,
		error: nil,
	}
}

func (inss *instructions) ReverseIterator() *InstructionIterator {
	return &InstructionIterator{
		inss:  *inss,
		index: len(*inss),
		error: nil,
	}
}

type instruction struct {
	pc  uint64
	arg []byte
	op  vm.OpCode

	offset uint64

	paramSize    int
	pushbackSize int
}

func (i *instruction) PC() uint64 {
	return i.pc
}

func (i *instruction) Value() *uint256.Int {
	return uint256.NewInt(0).SetBytes(i.arg)
}

func (i *instruction) String() string {
	argStr := ""
	if i.arg != nil {
		argStr = fmt.Sprintf("0x%x", i.arg)
	}
	if i.op == vm.PUSH0 {
		argStr = ""
	}
	coverStr := ""

	return fmt.Sprintf("0x%x(%d) %s %s %s", i.pc, i.pc, i.op.String(), argStr, coverStr)
}

type InstructionIterator struct {
	inss  []*instruction
	index int
	error error
}

func (ii *InstructionIterator) Next() bool {
	if ii.index >= len(ii.inss) {
		return false
	}
	if ii.error != nil {
		return false
	}
	ii.index++
	return true
}

func (ii *InstructionIterator) Prev() bool {
	if ii.index < 0 {
		return false
	}
	if ii.error != nil {
		return false
	}
	ii.index--
	return true
}

func (ii *InstructionIterator) Values() (pc uint64, arg []byte, op vm.OpCode, err error) {
	if ii.index >= len(ii.inss) {
		return 0, nil, 0, fmt.Errorf("iterator broken")
	}
	if ii.index == -1 {
		return 0, nil, 0, fmt.Errorf("iterator not started, by using Next() first")
	}
	return ii.inss[ii.index].pc, ii.inss[ii.index].arg, ii.inss[ii.index].op, nil
}

func (ii *InstructionIterator) OpCode() (vm.OpCode, error) {
	if ii.index >= len(ii.inss) {
		return 0, fmt.Errorf("iterator broken")
	}
	if ii.index == -1 {
		return 0, fmt.Errorf("iterator not started, by using Next() first")
	}
	return ii.inss[ii.index].op, nil
}

func (ii *InstructionIterator) StackSize() (paramSize int, pushbackSize int) {
	if ii.index >= len(ii.inss) {
		return 0, 0
	}
	if ii.index == -1 {
		return 0, 0
	}

	_, _, op, _ := ii.Values()
	return vm.GetStackSize(op)
}
