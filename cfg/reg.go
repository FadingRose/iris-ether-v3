package cfg

import (
	"strings"

	"github.com/holiman/uint256"
)

// Reg represents a concrete value
type Reg struct {
	inst   *instruction
	params []*Reg

	dupFrom *Reg

	exec executor

	val *uint256.Int

	cfg *CFG
}

func (r *Reg) IsConcrete() bool {
	return true
}

func (r *Reg) value() *uint256.Int {
	if r.exec == nil {
		return nil
	}
	return r.exec(r)
}

func (r *Reg) Expand() string {
	return r.expand("")
}

func (r *Reg) expand(indent string) string {
	var sb strings.Builder
	sb.WriteString(r.inst.String() + "\n")
	if r.dupFrom != nil {
		return (r.dupFrom).expand(indent)
	}
	for _, param := range r.params {
		sb.WriteString(indent + "├── " + param.expand(indent+"│   "))
	}
	return sb.String()
}

func (r *Reg) dependency() []*Reg {
	var deps []*Reg
	if r.dupFrom != nil {
		return r.dupFrom.dependency()
	}

	for _, param := range r.params {
		deps = append(deps, param.dependency()...)
	}

	return append(deps, r)
}

type AbstractReg struct {
	// depth int

	// each AbstractReg belonds to a BasicBlock A, assume there are B->A, C->A
	// so after stack mapping,
	// instances[B]->Reg
	// instances[A]->Reg
	instances map[int32]Reg

	cfg *CFG
}

func (ar *AbstractReg) IsConcrete() bool {
	return false
}
