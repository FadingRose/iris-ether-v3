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
	stateMutex sync.RWMutex

	forceStop bool

	leadingHalt map[*BasicBlock]bool

	D      []*BasicBlock
	fixedD map[*BasicBlock]bool
}

type StateMap map[*BasicBlock]map[string]map[uint64][]*Path // map[block]map[stack_hash]map[dest]

func (sm *StateMap) Insert(basicBlock *BasicBlock, stackHash string, dest uint64, path *Path) bool {
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
		return true
	} else {
		// Already exists, return false
		// (*sm)[basicBlock][stackHash][dest] = append((*sm)[basicBlock][stackHash][dest], path)
		return false
	}
}

func NewCFGBuilder(cfg *CFG) *CFGBuilder {
	return &CFGBuilder{
		cfg: cfg,
		// worklist: &Worklist{
		// 	curQueue: []*Path{
		// 		{
		// 			data:       []*BasicBlock{cfg.BasicBlocks[0]},
		// 			parent:     nil,
		// 			children:   make([]*Path, 0),
		// 			terminated: false,
		// 		},
		// 	},
		// 	inLoopQueue:  make([]*Path, 0),
		// 	allowPending: true,
		// 	cfg:          cfg,
		// },
		// stateMap:   make(map[*BasicBlock]map[string]map[uint64][]*Path), // map[block]map[stack_hash]map[dest]
		StateMap:   make(StateMap),
		stateMutex: sync.RWMutex{},
		// vis:         nil,
		forceStop:   false,
		leadingHalt: make(map[*BasicBlock]bool),
		D:           make([]*BasicBlock, 0),
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
		return []*BasicBlock{dest}, nil
	case vm.JUMPI:
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
					forks = append(forks, dest)
				}
			}
		}

		// false branch
		if path.last().next != nil {
			duplicated := cb.StateMap.Insert(path.last().next, st.Hash(), path.last().next.Pc(), path)
			if !duplicated {
				forks = append(forks, path.last().next)
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
	panic(ErrUnreachable)
}

func (cb *CFGBuilder) build(limit int, maxStackSize int) {
	// Step 1: LiteralConnectSuccessors
	cb.cfg.LiteralConnectSuccessors()

	// Step 2: init D set and fixed D set
	// DFS from the entry basic block
	var (
		SumD []*Path
		// sigma_D []*Path
	)

	visited := make(map[*BasicBlock]bool)
	var dfs func(bb *BasicBlock, currentPath *Path)
	dfs = func(bb *BasicBlock, currentPath *Path) {
		if visited[bb] {
			return
		}
		visited[bb] = true

		// If this block is in D, we've found a path for sum_D
		if bb.isContextAware() {
			SumD = append(SumD, currentPath.copy())
		}

		for _, succ := range bb.succs {
			newPath := currentPath.copy()
			if succ.Dst == nil {
				continue
			}
			newPath.push(succ.Dst)
			dfs(succ.Dst, newPath)
		}
		visited[bb] = false
	}

	entry := cb.cfg.BasicBlocks[0]
	dfs(entry, NewPath(entry))

	// Step 3: loop until D set is empty
	// 	for len(cb.D) > 0 {
	// 	}
	for _, path := range SumD {
		i := NewInterpreter(path, 1024)
		last, _, snaps, err := i.Run()
		if err != nil {
			fmt.Println("err", err)
			continue
		}
		if last == nil {
			fmt.Println("last is nil")
			continue
		}

		_, _, flag := snaps[path.last()].Live(last.params[0])
		// fmt.Println(res)
		// 如果 last 的 dest 解析的值不依赖于 S_in，那么它应当直接来源于最后一个块
		if !flag {
			path = NewPath(path.data[len(path.data)-1])
			i := NewInterpreter(path, 1024)
			st := NewAnyStack(cb.cfg)
			i.SetStack(st)
			slast, _, _, err := i.Run()
			if err != nil {
				fmt.Println("any test failed, err: ", err)
				continue
			}
			if last.params[0].value().Uint64() != slast.params[0].value().Uint64() {
				fmt.Println(path.String())
				fmt.Println("any test failed, last is not last")
				fmt.Println(last.params[0].value())
				fmt.Println(slast.params[0].value())
				fmt.Println(last.Expand())
				fmt.Println(slast.Expand())
				continue
			}
			// fmt.Println(last.Expand())
			continue
		}

		// fmt.Println(last.Expand())
	}
	// cb.cfg.PrintReadableCFG()
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
