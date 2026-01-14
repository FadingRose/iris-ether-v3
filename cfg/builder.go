package cfg

import (
	"fmt"
	"sync"

	"fadingrose/iris-ether-v3/vm"
)

type (
	Hash [32]byte
)

type CFGBuilder struct {
	cfg *CFG
	// worklist []*Path
	// worklist *Worklist

	// stateMap   map[*BasicBlock]map[string]map[uint64][]*Path // map[block]map[stack_hash]map[dest]
	StateMap
	SumD
	FixedD
	Liveness    *Liveness
	stateMutex  sync.RWMutex
	forceStop   bool
	leadingHalt map[*BasicBlock]bool
	fixedD      map[*BasicBlock]bool
}

type StateMap map[*BasicBlock]map[string]map[uint64][]*Path // map[block]map[stack_hash]map[dest]

func (sm *StateMap) Insert(basicBlock *BasicBlock, stackHash string, dest uint64, path *Path) (duplicated bool) {
	if _, ok := (*sm)[basicBlock]; !ok {
		(*sm)[basicBlock] = make(map[string]map[uint64][]*Path)
	}
	if _, ok := (*sm)[basicBlock][stackHash]; !ok {
		(*sm)[basicBlock][stackHash] = make(map[uint64][]*Path)
	}
	if _, ok := (*sm)[basicBlock][stackHash][dest]; !ok {
		(*sm)[basicBlock][stackHash][dest] = make([]*Path, 0)
	}

	if len((*sm)[basicBlock][stackHash][dest]) == 0 {
		// First time for (basicblock, stack hash, dest)
		(*sm)[basicBlock][stackHash][dest] = append((*sm)[basicBlock][stackHash][dest], path)
		duplicated = false
	} else {
		// Already exists
		// (*sm)[basicBlock][stackHash][dest] = append((*sm)[basicBlock][stackHash][dest], path)
		duplicated = true
	}
	return
}

type (
	SumD   map[*BasicBlock][]*Path
	FixedD map[*BasicBlock][]*Path
)

// Insert adds a path to the SumD set, D is the end of the path
func (sd *SumD) Insert(path *Path) bool {
	basicBlock := path.last()
	if _, ok := (*sd)[basicBlock]; !ok {
		(*sd)[basicBlock] = make([]*Path, 0)
	}
	(*sd)[basicBlock] = append((*sd)[basicBlock], path)
	return true
}

func (fd *FixedD) Insert(path *Path) bool {
	basicBlock := path.last()
	if _, ok := (*fd)[basicBlock]; !ok {
		(*fd)[basicBlock] = make([]*Path, 0)
	}
	(*fd)[basicBlock] = append((*fd)[basicBlock], path)
	return true
}

func NewCFGBuilder(cfg *CFG) *CFGBuilder {
	return &CFGBuilder{
		cfg:         cfg,
		StateMap:    make(StateMap),
		SumD:        make(SumD),
		FixedD:      make(FixedD),
		Liveness:    NewLiveness(),
		stateMutex:  sync.RWMutex{},
		forceStop:   false,
		leadingHalt: make(map[*BasicBlock]bool),
		fixedD:      make(map[*BasicBlock]bool),
	}
}

var (
	ErrLastIsNil          = fmt.Errorf("last is nil")
	ErrDuplicated         = fmt.Errorf("duplicated")
	ErrUnreachable        = fmt.Errorf("unreachable")
	ErrJumpDestUnsolved   = fmt.Errorf("JUMP's dest unsolved")
	ErrJumpiTrueUnsolved  = fmt.Errorf("JUMPI's true branch unsolved")
	ErrJumpiFalseUnsolved = fmt.Errorf("JUMPI's false branch unsolved")
	ErrPathNextIsNil      = fmt.Errorf("path.last().next is nil")
)

// execute executes the path and returns the next basic block, check duplicated internal
// if the path is duplicated, return nil, nil; otherwise,
//   - JUMP: return the next basic block, nil
//   - JUMPI: return the next basic block and fallthrough, nil; consider the condition value
//   - STOP/RETURN/REVERT/INVALID/SELFDESTRUCT: return nil, nil
//   - other: return the fallthrough basic block, nil
func (cb *CFGBuilder) execute(path *Path) ([]*BasicBlock, error) {
	if path.last().isContextAware() {
		return cb.executeDPath(path)
	}

	i := NewInterpreter(path, 1024)
	last, st, _, err := i.Run()
	if err != nil {
		return nil, err
	}
	if last == nil {
		return nil, ErrLastIsNil
	}
	switch last.inst.op {
	case vm.STOP, vm.RETURN, vm.REVERT, vm.INVALID, vm.SELFDESTRUCT:
		// terminated
		return nil, nil
	case vm.JUMP:
		if last.params[0].value() == nil {
			return nil, fmt.Errorf("%w: dest is nil", ErrJumpDestUnsolved)
		}
		destU64 := last.params[0].value().Uint64()
		dest := cb.cfg.GetValidDestBasicBlock(uint64(0), destU64)
		if dest == nil {
			return nil, fmt.Errorf("%w: dest is nil", ErrJumpDestUnsolved)
		}
		duplicated := cb.StateMap.Insert(dest, st.Hash(), destU64, path)
		if duplicated {
			return nil, ErrDuplicated
		}
		path.last().AppendSuccessor(dest, Jump)
		return []*BasicBlock{dest}, nil
	case vm.JUMPI:

		// fmt.Println(st.String())
		// fmt.Println(st.Hash())
		// fmt.Println(last.params[0].Expand())

		if last.params[0].value() != nil {
			condition := last.params[0].value().Uint64()
			if condition != 0 {
				// true branch only
				if last.params[1].value() == nil {
					return nil, fmt.Errorf("JUMPI's true branch unsolved, last.params[1].value() is nil")
				}
				destU64 := last.params[1].value().Uint64()
				dest := cb.cfg.GetValidDestBasicBlock(uint64(0), destU64)
				if dest == nil {
					return nil, fmt.Errorf("JUMPI's true branch unsolved, dest is nil")
				}
				duplicated := cb.StateMap.Insert(dest, st.Hash(), destU64, path)
				if duplicated {
					return nil, fmt.Errorf("duplicated")
				}
				path.last().AppendSuccessor(dest, JumpITrue)
				return []*BasicBlock{dest}, nil
			}

			// false branch only
			if last.params[1].value() == nil {
				return nil, ErrJumpiFalseUnsolved
			}
			if path.last().next == nil {
				return nil, fmt.Errorf("%w: path.last().next is nil", ErrJumpiFalseUnsolved)
			}
			duplicated := cb.StateMap.Insert(path.last().next, st.Hash(), path.last().next.Pc(), path)
			if duplicated {
				return nil, ErrDuplicated
			}
			path.last().AppendSuccessor(path.last().next, JumpIFalse)
			return []*BasicBlock{path.last().next}, nil
		}

		// last.params[0].value() == nil
		forks := make([]*BasicBlock, 0)
		errs := make([]error, 0)

		// true branch

		if last.params[1].value() == nil {
			errs = append(errs, ErrJumpiTrueUnsolved)
		} else {
			destU64 := last.params[1].value().Uint64()
			dest := cb.cfg.GetValidDestBasicBlock(uint64(0), destU64)
			if dest == nil {
				errs = append(errs, fmt.Errorf("%w: dest is nil", ErrJumpiTrueUnsolved))
			} else {
				duplicated := cb.StateMap.Insert(dest, st.Hash(), destU64, path)
				if !duplicated {
					path.last().AppendSuccessor(dest, JumpITrue)
					forks = append(forks, dest)
				} else {
					errs = append(errs, ErrDuplicated)
				}
			}
		}

		// false branch
		if path.last().next != nil {
			duplicated := cb.StateMap.Insert(path.last().next, st.Hash(), path.last().next.Pc(), path)
			if !duplicated {
				path.last().AppendSuccessor(path.last().next, JumpIFalse)
				forks = append(forks, path.last().next)
			} else {
				errs = append(errs, ErrDuplicated)
			}
		} else {
			errs = append(errs, ErrJumpiFalseUnsolved)
		}

		if len(forks) == 0 {
			if len(errs) == 0 {
				return nil, nil
			}
			return nil, fmt.Errorf("%v", errs)
		}
		return forks, nil
	default:
		// fallthrough
		if path.last().next == nil {
			return nil, ErrPathNextIsNil
		} else {
			duplicated := cb.StateMap.Insert(path.last().next, st.Hash(), path.last().next.Pc(), path)
			if !duplicated {
				return []*BasicBlock{path.last().next}, nil
			}
			return nil, nil
		}
	}
}

// executeDPath executes the D path and returns the next basic block, check duplicated internal
// D path is a path that ends with a context-aware block
// in this case, we need to update the liveness, check whether the path is MustJump or ContextAware
func (cb *CFGBuilder) executeDPath(path *Path) ([]*BasicBlock, error) {
	i := NewInterpreter(path, 1024)
	last, st, snaps, err := i.Run()
	if err != nil {
		return nil, err
	}
	if last == nil {
		return nil, ErrLastIsNil
	}

	_, _, flag := snaps[path.last()].Live(last.params[0])
	// If flag is false, the jump destination does not depend on the input stack
	if !flag {
		pathOne := NewPath(path.data[len(path.data)-1])
		i := NewInterpreter(pathOne, 1024)
		anyst := NewAnyStack(cb.cfg)
		i.SetStack(anyst)
		slast, _, _, err := i.Run()
		if err != nil {
			return nil, fmt.Errorf("any test failed: %w", err)
		}
		if last.params[0].value().Uint64() != slast.params[0].value().Uint64() {
			return nil, fmt.Errorf("any test failed: dest mismatch")
		}

		dstU64 := last.params[0].value().Uint64()
		dst := cb.cfg.verifyDest(dstU64)
		if dst == nil {
			return nil, fmt.Errorf("verifyDest failed")
		}

		duplicated := cb.StateMap.Insert(dst, st.Hash(), dstU64, path)
		if duplicated {
			return nil, ErrDuplicated
		}

		path.last().AppendSuccessor(dst, MustJump)
		cb.FixedD.Insert(path)

		return []*BasicBlock{dst}, nil
	}

	// flag is true: Last block depends on input stack (Context Aware)
	if last.params[0].value() == nil {
		return nil, fmt.Errorf("executeDPath: last.params[0].value() is nil")
	}
	destU64 := last.params[0].value().Uint64()
	dest := cb.cfg.verifyDest(destU64)
	if dest == nil {
		return nil, fmt.Errorf("verifyDest failed")
	}

	duplicated := cb.StateMap.Insert(dest, st.Hash(), destU64, path)
	if duplicated {
		return nil, ErrDuplicated
	}

	// Update liveness
	D_Block := path.last().index
	for i := len(path.data) - 1; i >= 0; i-- {
		slots, _, f := snaps[path.data[i]].Live(last.params[0])
		if !f {
			break
		}
		cb.Liveness.Live(path.data[i], slots, D_Block, destU64)
		// if path.data[i].index == 2019 { // TEMP
		// 	fmt.Println(res)
		// 	fmt.Println(slots)
		// }
	}

	path.last().AppendSuccessor(dest, ContextAware)
	cb.SumD.Insert(path)

	return []*BasicBlock{dest}, nil
}

// deprecated: use executeDPath()
// func (cb *CFGBuilder) DPathFound(path *Path) bool {
// 	i := NewInterpreter(path, 1024)
// 	last, _, snaps, err := i.Run()
// 	if err != nil {
// 		fmt.Println("err", err)
// 		return false
// 	}
// 	if last == nil {
// 		fmt.Println("last is nil")
// 		return false
// 	}
//
// 	// fmt.Println(last.Expand())
//
// 	_, _, flag := snaps[path.last()].Live(last.params[0])
//
// 	// 如果 last 的 dest 解析的值不依赖于 S_in，那么它应当直接来源于最后一个块
// 	// fmt.Println(path.String())
// 	// flag is false, 即 last 不依赖于输入栈
// 	// 此时应当断言它为 MustJump
// 	if !flag {
// 		path = NewPath(path.data[len(path.data)-1])
// 		i := NewInterpreter(path, 1024)
// 		anyst := NewAnyStack(cb.cfg)
// 		i.SetStack(anyst)
// 		slast, _, _, err := i.Run()
// 		if err != nil {
// 			fmt.Println("any test failed, err: ", err)
// 			// panic(fmt.Sprintf("any test failed, err: %s", address))
// 			return false
// 		}
// 		if last.params[0].value().Uint64() != slast.params[0].value().Uint64() {
// 			fmt.Println(path.String())
// 			fmt.Println("any test failed, last is not last")
// 			fmt.Println(last.params[0].value())
// 			fmt.Println(slast.params[0].value())
// 			fmt.Println(last.Expand())
// 			fmt.Println(slast.Expand())
// 			panic("any test failed, last is not last")
// 			return false
// 		}
// 		// fmt.Println("any test passed") // TODO: add this to fixed_D set
// 		// NOTE: append a JUMP edge
//
// 		dstU64 := last.params[0].value().Uint64()
// 		dst := cb.cfg.verifyDest(dstU64)
// 		if dst == nil {
// 			panic("verifyDest failed")
// 		}
//
// 		// duplicated := cb.StateMap.Insert(dst, st.Hash(), dstU64, path)
// 		// if !duplicated {
// 		// 	path.last().AppendSuccessor(dst, MustJump)
// 		// }
// 		// // path.last().AppendSuccessor(dst, MustJump)
// 		return false
// 	}
//
// 	// flag is true，即 last 依赖输入栈
// 	// 对于 path 中的每一个块，都有一个特定的 Liveness 的组合
// 	// 一个 Liveness 意味着：一个块的一个特定栈状态导出到特定的 D
// 	// fmt.Println(path.String())
// 	if last.params[0].value() == nil {
// 		fmt.Println("last.params[0].value() is nil")
// 		fmt.Println(path.String())
// 		fmt.Println(last.Expand())
// 		return false
// 	}
// 	dest := last.params[0].value().Uint64()
// 	D_Block := path.last().index
//
// 	var meaningful bool // update liveness at least once
// 	for i := len(path.data) - 1; i >= 0; i-- {
// 		slots, _, flag := snaps[path.data[i]].Live(last.params[0])
// 		if cb.Liveness.Live(path.data[i], slots, D_Block, dest) {
// 			meaningful = true
// 		}
// 		// fmt.Println(path.data[i].String())
// 		// fmt.Println(res)
// 		// fmt.Println(flag)
// 		if !flag {
// 			// fmt.Printf("cutoff at block %d of path %s\n", path.data[i].index, path.String())
// 			break
// 		}
// 	}
//
// 	// duplicated := cb.StateMap.Insert(path.last(), st.Hash(), path.last().Pc(), path)
// 	// if !duplicated {
// 	// 	path.last().AppendSuccessor(path.last(), ContextAware)
// 	// }
// 	//
// 	return meaningful
// }

func (cb *CFGBuilder) LiveCheck(path *Path) bool {
	// 需要测试最后一个块的输入栈，因此 执行的 path 是 path[0:len(path)-1]
	if len(path.data) <= 1 {
		return false
	}
	testPath := path.copy()
	bb := path.last()
	testPath.data = testPath.data[:len(testPath.data)-1]
	i := NewInterpreter(testPath, 1024)
	// fmt.Println(testPath.String())
	_, st, _, err := i.Run()
	if err != nil {
		return false
	}
	// 如果 liveness 命中，记录到 prune 中并且跳出循环
	if cb.Liveness.Check(st, path, bb.index) {
		// fmt.Println("liveness test passed, match one of the liveness")
		// fmt.Println(currentPath.String())
		return true
	}

	return false
}

func (cb *CFGBuilder) Build(path *Path, limit int, maxStackSize int) {
	cb.cfg.LiteralConnectSuccessors()
	if path == nil {
		path = NewPath(cb.cfg.BasicBlocks[0])
	}
	pathList := make([]*Path, 0)
	pathList = append(pathList, path)
	cb.build(pathList)
}

func (cb *CFGBuilder) build(pathList []*Path) {
	var dbstr string
	dprint := func(content string, flush bool, clean bool) {
		if clean {
			dbstr = ""
			return
		}
		dbstr += content
		if flush {
			fmt.Println(dbstr)
			dbstr = ""
			return
		}
	}

	var cnt int
	for len(pathList) > 0 {
		curPath := pathList[0]
		pathList = pathList[1:]
		dprint(curPath.String(), false, false)
		// fmt.Println(curPath.String())

		// halt check
		if curPath.last().isLeadingHalt() {
			dprint("[halt]", true, false)
			continue
		}

		if cb.LiveCheck(curPath) {
			dprint("[liveness]", true, false)
			continue
		}

		// update liveness // NOTE: move in the execute() -> executeDPath()
		// if curPath.last().isContextAware() {
		// 	np := curPath.copy()
		// 	if cb.DPathFound(np) {
		// 		cb.SumD.Insert(np)
		// 	} else {
		// 		cb.FixedD.Insert(np)
		// 	}
		// 	continue
		// }

		// for a loop, we need to check the liveness
		// P = {B1,B2,H1}
		// P = {B1,B2,H1,....,H1}
		header, isloop := curPath.LeadingLoop()
		if isloop {
			if !cb.Liveness.ContainsLive(header) {
				dprint("[loop header not contains live]", true, false)
				// fmt.Println("loop header not contains live")
				continue
			}
		}

		succ, err := cb.execute(curPath)
		if err != nil {
			// fmt.Println(err)
			dprint(fmt.Sprintf("[execute error: %s]", err), true, false)
			continue
		}
		// fmt.Println(succ)
		np := curPath.fork(succ)
		if len(np) > 0 {
			pathList = append(pathList, np...)
		}

		cnt++
		dprint("", true, true)
		// if cnt > 20 {
		// 	break
		// }
	}
	// Step 2: init D set and fixed D set
	// DFS from the entry basic block
}

func (cb *CFGBuilder) runWithAnyStack(startBB *BasicBlock) {
	path := NewPath(startBB)
	pathlist := []*Path{path}
	for len(pathlist) > 0 {
		curpath := pathlist[0]
		pathlist = pathlist[1:]

		anyst := NewAnyStack(cb.cfg)
		i := NewInterpreter(curpath, 1024)
		i.SetStack(anyst)
		last, _, _, err := i.Run()
		if err != nil {
			continue
		}
		if last == nil {
			continue
		}
	}
}

// func (cb *CFGBuilder) AnyStackConnectSuccessors() {
// }
