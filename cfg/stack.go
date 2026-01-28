package cfg

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"fadingrose/iris-ether-v3/vm"
)

// Stack is a generic LIFO stack.
type Stack struct {
	items []*Reg
}

type Snapshot struct {
	items []*AbstractReg
}

func (s *Snapshot) Live(last *Reg) (map[int]string, string, bool) {
	if len(s.items) == 0 {
		return nil, "Snapshot: []", false
	}
	for last.dupFrom != nil {
		last = last.dupFrom
	}

	liveness := make(map[int]string, 0)

	var sb strings.Builder
	sb.WriteString("Snapshot:\n")
	flag := false
	for i := len(s.items) - 1; i >= 0; i-- {
		height := len(s.items) - 1 - i
		reg := s.items[i].instance
		for reg.dupFrom != nil {
			reg = reg.dupFrom
		}
		instruction := ""
		if reg.inst == nil {
			fmt.Fprintf(&sb, "[%d] nil\n", height)
			continue
		} else {
			instruction = reg.inst.String()
		}

		if reg.inst.String() == last.inst.String() {
			fmt.Fprintf(&sb, "[%d]* %s\n", height, instruction)
			liveness[height] = instruction
			flag = true
		} else {
			fmt.Fprintf(&sb, "[%d] %s\n", height, instruction)
		}
	}
	return liveness, sb.String(), flag
}

func (s *Snapshot) String(last *Reg) string {
	if len(s.items) == 0 {
		return "Snapshot: []"
	}
	for last.dupFrom != nil {
		last = last.dupFrom
	}
	var sb strings.Builder
	sb.WriteString("Snapshot:\n")
	for i := len(s.items) - 1; i >= 0; i-- {
		height := len(s.items) - 1 - i
		reg := s.items[i].instance
		opName := "nil"
		if reg.inst != nil {
			opName = reg.expand("    ")
		}
		// if reg.Equals(last) {

		// for reg.dupFrom != nil {
		// 	reg = reg.dupFrom
		// 	opName = reg.inst.String()
		// }

		// fmt.Println(reg.inst.String(), " ", last.inst.String())
		if reg.inst == nil {
			fmt.Fprintf(&sb, "[%d] nil\n", height)
			continue
		}

		if reg.inst.String() == last.inst.String() {
			fmt.Fprintf(&sb, "[%d]* %s\n", height, opName)
		} else {
			fmt.Fprintf(&sb, "[%d] %s\n", height, opName)
		}
	}
	return sb.String()
}

var (
	ErrStackUnderflow = fmt.Errorf("stack underflow")
	ErrStackOverflow  = fmt.Errorf("stack overflow")
)

// NewStack creates and returns a new Stack.
func NewStack() *Stack {
	return &Stack{}
}

func NewAnyStack(cfg *CFG) *Stack {
	stack := NewStack()
	// Seed with abstract (nil inst/val) items to represent unknown input stack.
	for range 16 {
		stack.Push(&Reg{
			inst:    nil,
			params:  nil,
			dupFrom: nil,
			exec:    nil,
			val:     nil,
			cfg:     cfg,
		})
	}
	return stack
}

func (s *Stack) ToAnyStack() *Stack {
	anystack := NewStack()
	for _, item := range s.items {
		anystack.items = append(anystack.items, &Reg{
			inst:    item.inst,
			params:  nil,
			dupFrom: nil,
			exec:    nil,
			val:     nil,
			cfg:     item.cfg,
		})
	}
	return anystack
}

func (s *Stack) Snapshot() *Snapshot {
	snapshot := &Snapshot{
		items: make([]*AbstractReg, len(s.items)),
	}
	for i, item := range s.items {
		snapshot.items[i] = item.snapshot()
	}
	return snapshot
}

func (s *Stack) GetAtPosition(pos int) *Reg {
	if len(s.items) == 0 || pos >= len(s.items) || pos < 0 {
		return nil
	}
	// pos is counted from top of stack: 0 = top.
	return s.items[len(s.items)-1-pos]
}

// Push adds an item to the top of the stack.
func (s *Stack) Push(item *Reg) {
	s.items = append(s.items, item)
}

// Pop removes and returns the top item from the stack.
// It returns an error if the stack is empty.
func (s *Stack) Pop() (*Reg, error) {
	if len(s.items) == 0 {
		var zero *Reg
		return zero, errors.New("stack is empty")
	}
	index := len(s.items) - 1
	item := s.items[index]
	s.items = s.items[:index]
	return item, nil
}

// Peek returns the top item from the stack without removing it.
// It returns an error if the stack is empty.
func (s *Stack) Peek() (*Reg, error) {
	if len(s.items) == 0 {
		var zero *Reg
		return zero, errors.New("stack is empty")
	}
	return s.items[len(s.items)-1], nil
}

// IsEmpty checks if the stack is empty.
func (s *Stack) IsEmpty() bool {
	return len(s.items) == 0
}

func (s *Stack) GetDupN(n int) (*Reg, error) {
	if len(s.items) < n {
		return nil, ErrStackUnderflow
	}
	return s.items[len(s.items)-n], nil
}

func (s *Stack) SwapN(n int) error {
	n++
	if len(s.items) < n {
		return ErrStackUnderflow
	}
	s.items[len(s.items)-1], s.items[len(s.items)-n] = s.items[len(s.items)-n], s.items[len(s.items)-1]
	return nil
}

// popN make sure the return value is in the order of the stack
func (s *Stack) PopN(n int) ([]*Reg, error) {
	if len(s.items) < n {
		return nil, ErrStackUnderflow
	}
	ret := append([]*Reg{}, s.items[len(s.items)-n:]...)
	s.items = s.items[:len(s.items)-n]
	return ret, nil
}

func (s *Stack) PushN(r *Reg, n int) {
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		s.Push(r)
	}
}

func (s *Stack) Hash() string {
	h := sha256.New()
	for _, reg := range s.items {
		// Hash is value-based only; unknown values are encoded as "nil".
		if reg.value() != nil {
			h.Write([]byte(reg.value().String()))
		} else {
			h.Write([]byte("nil"))
		}
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Stack) String() string {
	ops := "   ["
	// locs := "   ["

	for i, reg := range s.items {
		value := ""
		if reg.value() != nil {
			value = fmt.Sprintf("=%s", reg.value().Hex())
		} else {
			value = "=nil"
		}

		if reg.inst != nil {
			if tracedStackOps[reg.inst.op] || reg.inst.op.IsPush() {
				ops += fmt.Sprintf("%s(0x%x)%s", reg.inst.op.String(), reg.inst.pc, value)
			} else {
				ops += fmt.Sprintf("%s%s", reg.inst.op.String(), value)
			}
			// locs += fmt.Sprintf("%d", -(len(s.items) - 1 - i)) // reverse the index
			if i < len(s.items)-1 {
				ops += ","
				// locs += strings.Repeat(" ", len(reg.inst.op.String())+6)
			}
		}

	}
	// return ops + "\n" + locs + "\n"
	return ops
}

var tracedStackOps = map[vm.OpCode]bool{
	vm.SHR:    true,
	vm.SHL:    true,
	vm.AND:    true,
	vm.SLOAD:  true,
	vm.SSTORE: true,
	vm.MSTORE: true,
}

// var ignoredOps = map[vm.OpCode]bool{
// 	vm.SSTORE: true,
// 	vm.SLOAD:  true,
// }
