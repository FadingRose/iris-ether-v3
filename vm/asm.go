package vm

import (
	"encoding/hex"
	"fmt"
)

type instructionIterator struct {
	code    []byte
	pc      uint64
	arg     []byte
	op      OpCode
	error   error
	started bool
}

func NewInstructionIterator(code []byte) *instructionIterator {
	return &instructionIterator{code: code}
}

// Next returns true if there is another instruction to read and move on.
func (it *instructionIterator) Next() bool {
	if it.error != nil || uint64(len(it.code)) <= it.pc {
		// The function returns false if there is an error or the length of the code is less than the program counter.
		return false
	}

	if it.started {
		// If the iterator has started, it will read the next instruction.
		if it.arg != nil {
			it.pc += uint64(len(it.arg))
			it.arg = nil
		}
		it.pc++
	} else {
		// If the iterator has not started, it will read the first instruction.
		it.started = true
	}

	if uint64(len(it.code)) <= it.pc {
		// We reach the end.
		return false
	}

	it.op = OpCode(it.code[it.pc])
	if it.op.IsPush() {
		// If the instruction is a push instruction, it will read the argument.
		a := uint64(it.op) - uint64(PUSH0)
		u := it.pc + 1 + a
		if uint64(len(it.code)) <= it.pc || uint64(len(it.code)) < u {
			it.error = fmt.Errorf("incomplete push instruction at %v", it.pc)
			return false
		}
		it.arg = it.code[it.pc+1 : u]
	} else {
		// If the instruction is not a push instruction, the argument is nil.
		it.arg = nil
	}
	return true
}

func (it *instructionIterator) ToStop() {
	for it.Next() {
		if it.op == STOP {
			break
		}
	}
}

// Error returns any error that may have been encountered.
func (it *instructionIterator) Error() error {
	return it.error
}

// PC returns the PC of the current instruction.
func (it *instructionIterator) PC() uint64 {
	return it.pc
}

// Op returns the opcode of the current instruction.
func (it *instructionIterator) Op() OpCode {
	return it.op
}

// Arg returns the argument of the current instruction.
func (it *instructionIterator) Arg() []byte {
	return it.arg
}

// PrintDisassembled pretty-print all disassembled EVM instructions to stdout.
func PrintDisassembled(code string) error {
	script, err := hex.DecodeString(code)
	if err != nil {
		return err
	}

	it := NewInstructionIterator(script)
	for it.Next() {
		if it.Arg() != nil && 0 < len(it.Arg()) {
			fmt.Printf("%05x: %v %#x\n", it.PC(), it.Op(), it.Arg())
		} else {
			fmt.Printf("%05x: %v\n", it.PC(), it.Op())
		}
	}
	return it.Error()
}

// // Disassemble returns all disassembled EVM instructions in human-readable format.
// func Disassemble(script []byte) ([]string, error) {
// 	instrs := make([]string, 0)
//
// 	it := NewInstructionIterator(script)
// 	for it.Next() {
// 		if it.Arg() != nil && 0 < len(it.Arg()) {
// 			instrs = append(instrs, fmt.Sprintf("%05x: %v %#x\n", it.PC(), it.Op(), it.Arg()))
// 		} else {
// 			instrs = append(instrs, fmt.Sprintf("%05x: %v\n", it.PC(), it.Op()))
// 		}
// 	}
// 	if err := it.Error(); err != nil {
// 		return nil, err
// 	}
// 	return instrs, nil
// }
