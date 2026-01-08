package cfg

import (
	"fmt"
	"sync"
)

type (
	Hash [32]byte
)

type CFGBuilder struct {
	cfg *CFG
	// worklist []*Path
	// worklist *Worklist

	stateMap   map[*BasicBlock]map[string]map[int64][]*Path // map[block]map[stack_hash]map[dest]
	stateMutex sync.RWMutex

	forceStop bool

	leadingHalt map[*BasicBlock]bool

	D      []*BasicBlock
	fixedD map[*BasicBlock]bool
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
		stateMap:   make(map[*BasicBlock]map[string]map[int64][]*Path), // map[block]map[stack_hash]map[dest]
		stateMutex: sync.RWMutex{},
		// vis:         nil,
		forceStop:   false,
		leadingHalt: make(map[*BasicBlock]bool),
		D:           make([]*BasicBlock, 0),
		fixedD:      make(map[*BasicBlock]bool),
	}
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

		_, flag := snaps[path.last()].Live(last.params[0])
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
