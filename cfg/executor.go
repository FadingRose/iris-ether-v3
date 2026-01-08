package cfg

import (
	"fadingrose/iris-ether-v3/vm"

	"github.com/holiman/uint256"
)

type executor func(me *Reg) *uint256.Int

func Executor(op vm.OpCode) executor {
	return execute(op)
}

func execute(op vm.OpCode) executor {
	if op.IsPush() {
		return func(me *Reg) *uint256.Int {
			return me.inst.Value()
		}
	}

	if op.IsDup() {
		return func(me *Reg) *uint256.Int {
			if me.dupFrom == nil {
				return nil
			}
			return me.dupFrom.value()
		}
	}

	switch op {
	case vm.ADD:
		return func(me *Reg) *uint256.Int {
			if len(me.params) < 2 {
				return nil
			}
			left, right := me.params[1].value(), me.params[0].value()
			if left == nil || right == nil {
				return nil
			}
			ret := uint256.NewInt(0)
			ret.Add(left, right)
			return ret
		}
	case vm.SUB:
		return func(me *Reg) *uint256.Int {
			// return nil
			left, right := me.params[1].value(), me.params[0].value()
			if left == nil || right == nil {
				return nil
			}
			ret := uint256.NewInt(0)
			ret.Sub(left, right)
			return ret
		}
	case vm.SHR:
		return func(me *Reg) *uint256.Int {
			// if me.params[0] == nil || me.params[1] == nil {
			// 	return nil
			// }
			shift, value := me.params[1].value(), me.params[0].value()
			if shift == nil || value == nil {
				return nil
			}
			ret := uint256.NewInt(0)
			if shift.LtUint64(256) {
				ret.Lsh(value, uint(shift.Uint64()))
			} else {
				ret.Clear()
			}
			return ret
		}
	case vm.SHL:
		return func(me *Reg) *uint256.Int {
			// if me.params[0] == nil || me.params[1] == nil {
			// 	return nil
			// }

			shift, value := me.params[1].value(), me.params[0].value()
			if shift == nil || value == nil {
				return nil
			}
			ret := uint256.NewInt(0)
			if shift.LtUint64(256) {
				ret.Lsh(value, uint(shift.Uint64()))
			} else {
				ret.Clear()
			}
			return ret
		}
	case vm.AND:
		return func(me *Reg) *uint256.Int {
			// if me.params[0] == nil || me.params[1] == nil {
			// 	return nil
			// }

			ret := uint256.NewInt(0)
			if me.params[0].value() == nil || me.params[1].value() == nil {
				return nil
			}
			ret.And(me.params[0].value(), me.params[1].value())
			return ret
		}
	case vm.SLOAD:
		return func(me *Reg) *uint256.Int {
			ret := uint256.NewInt(0)
			if me.params[0].value() == nil {
				return nil
			}
			ret.Set(me.params[0].value())
			return ret
		}

	default:
		return func(me *Reg) *uint256.Int {
			return nil
		}
	}
}

// we will use this function to check whether the params are ConcreteReg
func validParamsCheck(n int, params []*Reg) bool {
	if len(params) < n {
		return false
	}
	for _, param := range params {
		if param == nil {
			return false
		}
	}
	return true
}
