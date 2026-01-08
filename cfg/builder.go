package cfg

import (
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
	}
}

func (cb *CFGBuilder) build(limit int, maxStackSize int) {
	// Step 1: LiteralConnectSuccessors
	cb.cfg.LiteralConnectSuccessors()
}
