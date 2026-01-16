// Package cfg provides a way to analyze the control flow graph of a smart contract.
package cfg

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	// "fadingrose/iris-ether-v3/compiler"
	"fadingrose/iris-ether-v3/vm"
)

type CFG struct {
	BasicBlocks []*BasicBlock `json:"blocks"`
	// ProbePairs  []compiler.ProbePair `json:"probe_pairs"`

	validDestMap map[uint64]map[uint64]*BasicBlock // validDestMap[offset]map[pc]*BasicBlock
	builder      *CFGBuilder

	TimeCost int64 // in ms
	memCost  uint64

	totalPaths      int // how many paths have been explored?
	meaningfulPaths int // how many paths introduced new coverage?
	brokenPaths     int // how many paths ended up in broken state?
	duplicatePaths  int // how many paths are duplicates?
	haltedPaths     int // how many paths ended up in halt state?
	selfloopPaths   int // how many paths ended up in self-loop state?

	haltReason string

	infos   []string
	profile []float64

	bytecode []byte
}

func NewCFG(bytecode []byte) *CFG {
	cfg := &CFG{
		BasicBlocks: make([]*BasicBlock, 0),
		// ProbePairs:   make([]compiler.ProbePair, 0),
		validDestMap: make(map[uint64]map[uint64]*BasicBlock),
		builder:      nil,

		infos:   make([]string, 0),
		profile: make([]float64, 0),

		bytecode: bytecode,
	}

	// if start with '0x', trim it
	if len(bytecode) >= 2 && bytecode[0] == '0' && (bytecode[1] == 'x' || bytecode[1] == 'X') {
		bytecode, _ = hex.DecodeString(string(bytecode[2:]))
	}
	it := vm.NewInstructionIterator(bytecode)
	nxtit := vm.NewInstructionIterator(bytecode)
	nxtit.Next()

	offset := uint64(0)
	cur := NewBasicBlock(int32(0), cfg, offset)

	for it.Next() {
		paramsSize, pushbackSize := vm.GetStackSize(it.Op())
		cur.stmts = append(cur.stmts, &instruction{
			pc:           it.PC(),
			arg:          it.Arg(),
			op:           it.Op(),
			offset:       offset,
			paramSize:    paramsSize,
			pushbackSize: pushbackSize,
		})

		if it.Op() == vm.INVALID {
			offset = it.PC() + 1
		}

		nxtit.Next()

		if isLeadingEnd(it.Op(), nxtit.Op()) {
			idx := cur.index + 1
			cfg.BasicBlocks = append(cfg.BasicBlocks, cur)
			cur = NewBasicBlock(idx, cfg, offset)
		}

		if it.Op() == vm.JUMPDEST {
			if cfg.validDestMap[offset] == nil {
				cfg.validDestMap[offset] = make(map[uint64]*BasicBlock)
			}
			cfg.validDestMap[offset][it.PC()-offset] = cur
			if cfg.validDestMap[uint64(0)] == nil {
				cfg.validDestMap[uint64(0)] = make(map[uint64]*BasicBlock)
			}
			cfg.validDestMap[uint64(0)][it.PC()] = cur
		}
	}

	for i := 0; i < len(cfg.BasicBlocks)-1; i++ {
		cfg.BasicBlocks[i].next = cfg.BasicBlocks[i+1]
	}

	if len(cfg.BasicBlocks) == 0 {
		return nil
	}
	return cfg
}

//	func NewCFGFromJSON(bytecode string, result string) (*CFG, error) {
//		data, err := os.ReadFile(bytecode)
//		if err != nil {
//			return nil, fmt.Errorf("failed to read bytecode file: %w", err)
//		}
//		cfg := NewCFG(data)
//		bm := make(map[int32]*BasicBlock)
//		for _, b := range cfg.BasicBlocks {
//			bm[b.index] = b
//		}
//
//		data, err = os.ReadFile(result)
//		if err != nil {
//			return nil, fmt.Errorf("failed to read result file: %w", err)
//		}
//
//		var res blocks
//
//		err = json.Unmarshal(data, &res)
//		if err != nil {
//			return nil, err
//		}
//
//		for _, b := range res.Blocks {
//			src := bm[b.Index]
//			for _, succ := range b.Succs {
//				_, ok1 := bm[succ.Src]
//				_, ok2 := bm[succ.Dst]
//				if ok1 && ok2 {
//					src.appendSuccessor(bm[succ.Dst], ToEdgeType(succ.Typ), nil)
//				}
//			}
//		}
//
//		return cfg, nil
//	}
func (cfg *CFG) LiteralConnectSuccessors() {
	bs := cfg.BasicBlocks

	for _, b := range bs {
		lastop := b.LastStmt().op

		// No successors, ignores
		if lastop == vm.STOP || lastop == vm.RETURN || lastop == vm.REVERT || lastop == vm.INVALID || lastop == vm.SELFDESTRUCT {
			continue
		}

		// try to connect the direct next
		if lastop != vm.JUMP {
			if b.next != nil {
				if lastop == vm.JUMPI {
					b.appendSuccessor(b.next, JumpIFalse, nil)
				} else {
					b.appendSuccessor(b.next, DirectLink, nil)
				}
			}
		}

		// whether there is a direct literal jumpdest?
		if b.LastSecondStmt() == nil {
			continue
		}

		// there is NO PUSH2 indicates a jumpdest before JUMP/JUMPI
		if !cfg.verifyDestPush(b.LastSecondStmt()) {
			if lastop == vm.JUMP || lastop == vm.JUMPI {
				b.appendSuccessor(nil, Any, nil)
			}
			continue
		} else {
			if lastop == vm.JUMP {
				b.appendSuccessor(cfg.verifyDest(b.LastSecondStmt().Value().Uint64()), Jump, nil)
			} else if lastop == vm.JUMPI {
				b.appendSuccessor(cfg.verifyDest(b.LastSecondStmt().Value().Uint64()), JumpITrue, nil)
			}
		}

		// dest := b.LastSecondStmt().Value().Uint64()
		// offset := b.offset
		// if destBlock := cfg.GetValidDestBasicBlock(offset, dest); destBlock != nil {
		//
		// 	// if destBlock, ok := cfg.validDestMap[offset][dest]; ok {
		// 	if lastop == vm.JUMP {
		// 		b.appendSuccessor(destBlock, Jump)
		// 	}
		// 	if lastop == vm.JUMPI {
		// 		b.appendSuccessor(destBlock, JumpITrue)
		// 	}
		// }

	}
}

func (cfg *CFG) GetValidDestBasicBlock(offset uint64, pc uint64) *BasicBlock {
	if destBlock, ok := cfg.validDestMap[offset][pc]; ok {
		return destBlock
	}
	return nil
}

func (cfg *CFG) GetBasicBlockViaPc(offset uint64, pc uint64) *BasicBlock {
	for _, bb := range cfg.BasicBlocks {
		if bb.offset == offset && bb.Pc() == pc {
			return bb
		}
	}
	return nil
}

// isLeadingEnd returns true if the opcode is a leading end of a basic block.
// JUMP | JUMPI | STOP | RETURN | REVERT | INVALID | SELFDESTRUCT
func isLeadingEnd(op vm.OpCode, nxtop vm.OpCode) bool {
	switch op {
	case vm.JUMP, vm.JUMPI, vm.STOP, vm.RETURN, vm.REVERT, vm.INVALID, vm.SELFDESTRUCT:
		return true
	default:
		return isLeadingStart(nxtop)
	}
}

// isLeadingStart returns true if the opcode is a leading start of a basic block.
// JUMPDEST
func isLeadingStart(op vm.OpCode) bool {
	return op == vm.JUMPDEST
}

// verifyDestPush verifies whether the instruction is a valid PUSH instruction for jump destination.
func (cfg *CFG) verifyDestPush(ins *instruction) bool {
	if !ins.op.IsPush() {
		return false
	}
	if _, exist := cfg.validDestMap[ins.offset][ins.Value().Uint64()]; exist {
		return true
	}
	if _, exist := cfg.validDestMap[uint64(0)][ins.Value().Uint64()]; exist {
		return true
	}
	return false
}

// func (cfg *CFG) Build(limit int, max_stack_size int, timeout int, visul bool) {
// 	// Capture memory stats before the operation
// 	var m1 runtime.MemStats
// 	runtime.ReadMemStats(&m1)
//
// 	// Record start time
// 	start := time.Now()
//
// 	builder := NewCFGBuilder(cfg)
// 	cfg.builder = builder
//
// 	// if visul {
// 	// 	builder.StartVisuailize()
// 	// 	time.Sleep(3 * time.Second) // wait for profiler to attach
// 	// }
// 	//
// 	if timeout > 0 {
// 		done := make(chan struct{})
// 		go func() {
// 			defer close(done)
// 			builder.Build(nil, limit, max_stack_size)
// 		}()
// 		select {
// 		case <-done:
// 			break
// 		case <-time.After(time.Duration(timeout) * time.Second):
// 			cfg.haltReason = "timeout"
// 			break
// 		}
// 	} else {
// 		builder.Build(nil, limit, max_stack_size)
// 	}
//
// 	// Record duration
// 	duration := time.Since(start)
//
// 	// Capture memory stats after the operation
// 	var m2 runtime.MemStats
// 	runtime.ReadMemStats(&m2)
//
// 	// Calculate memory allocated
// 	// m2.TotalAlloc - m1.TotalAlloc gives the cumulative bytes allocated.
// 	// m2.Alloc - m1.Alloc gives the change in heap size.
// 	alloc := m2.TotalAlloc - m1.TotalAlloc
//
// 	cfg.TimeCost = duration.Milliseconds()
// 	cfg.memCost = alloc
// }

//	func (cfg *CFG) StopBuild() {
//		cfg.builder.stop()
//	}
func (cfg *CFG) verifyDest(dest uint64) *BasicBlock {
	for offset, scope := range cfg.validDestMap {
		for pc, destBB := range scope {
			if dest == pc+offset {
				return destBB
			}
		}
	}
	return nil
}

type Summary struct {
	// block coverage
	CoveredBlocks int     `json:"covered_blocks"`
	TotalBlocks   int     `json:"total_blocks"`
	Blockcoverage float64 `json:"block_coverage"`

	EdgeNums int    `json:"edge_nums"`
	Time     int64  `json:"time"` // in ms
	Mem      uint64 `json:"mem"`  // in KB

	// path summary
	TotalPaths      int `json:"total_paths"`
	MeaningfulPaths int `json:"meaningful_paths"`
	BrokenPaths     int `json:"broken_paths"`
	DuplicatePaths  int `json:"duplicate_paths"`
	HaltedPaths     int `json:"halted_paths"`

	HaltReason string `json:"halt_reason,omitempty"`

	Infos   []string  `json:"infos"`
	Profile []float64 `json:"profile,omitempty"`

	PairCovered  int `json:"pair_covered"`
	PairTotal    int `json:"pair_total"`
	ProbeCovered int `json:"probe_covered"`
	ProbeTotal   int `json:"probe_total"`
}

func (s Summary) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Block Coverage: %d/%d (%.2f%%)\n", s.CoveredBlocks, s.TotalBlocks, s.Blockcoverage))
	builder.WriteString(fmt.Sprintf("Total Edges: %d\n", s.EdgeNums))
	builder.WriteString(fmt.Sprintf("Execution Time: %d ms\n", s.Time))
	builder.WriteString(fmt.Sprintf("Memory Usage: %d KB\n", s.Mem))
	builder.WriteString(fmt.Sprintf("Paths Explored: %d\n", s.TotalPaths))
	builder.WriteString(fmt.Sprintf("  - Meaningful: %d\n", s.MeaningfulPaths))
	builder.WriteString(fmt.Sprintf("  - Broken: %d\n", s.BrokenPaths))
	builder.WriteString(fmt.Sprintf("  - Duplicate: %d\n", s.DuplicatePaths))
	builder.WriteString(fmt.Sprintf("  - Halted: %d\n", s.HaltedPaths))
	builder.WriteString(fmt.Sprintf("Halt Reason: %s\n", s.HaltReason))
	builder.WriteString("Profile: ")
	for _, p := range s.Profile {
		builder.WriteString(fmt.Sprintf("%.2f%% ", p*100.0))
	}
	return builder.String()
}

func (s Summary) ProfileString() string {
	var builder strings.Builder
	builder.WriteString("Profile per second:\n")
	for i, p := range s.Profile {
		fmt.Fprintf(&builder, "  Second %d: %.2f%%\n", i+1, p)
	}
	return builder.String()
}

func (cfg *CFG) coverage() (int, int) {
	entry := cfg.BasicBlocks[0]
	visited := make(map[int32]bool)
	var dfs func(bb *BasicBlock)
	dfs = func(bb *BasicBlock) {
		visited[bb.index] = true
		bb.visited++
		for _, succ := range bb.succs {
			if succ.Dst == nil {
				continue
			}
			if visited[succ.Dst.index] {
				continue
			}
			dfs(succ.Dst)
		}
	}
	dfs(entry)

	return len(visited), len(cfg.BasicBlocks)
}

func (cfg *CFG) edges() int {
	var edges int
	for _, b := range cfg.BasicBlocks {
		for _, succ := range b.succs {
			if succ.Dst == nil {
				continue
			}
			edges++
		}
	}
	return edges
}

// func (cfg *CFG) Summary() Summary {
// 	// block coverage
// 	var s Summary
// 	for _, b := range cfg.BasicBlocks {
// 		if b.visited > 0 {
// 			s.CoveredBlocks++
// 		}
// 		for _, succ := range b.succs {
// 			if succ.Dst == nil {
// 				continue
// 			}
// 			s.EdgeNums++
// 		}
// 	}
//
// 	// from Block 0 to Block INVALID
// 	s.TotalBlocks = len(cfg.BasicBlocks)
// 	// some of the blocks after INVALID may be touched, we count all the blocks including data
// 	afterInvalid := false
// 	for _, b := range cfg.BasicBlocks {
// 		if len(b.stmts) == 1 && b.stmts[0].op == vm.INVALID {
// 			s.TotalBlocks = int(b.index)
// 			afterInvalid = true
// 		}
// 		if afterInvalid && b.visited > 0 {
// 			s.TotalBlocks++
// 		}
// 	}
//
// 	s.Blockcoverage = float64(s.CoveredBlocks) / float64(s.TotalBlocks) * 100.0
//
// 	s.Time = cfg.TimeCost
// 	s.Mem = cfg.memCost
//
// 	s.TotalPaths = cfg.totalPaths
// 	s.MeaningfulPaths = cfg.meaningfulPaths
// 	s.BrokenPaths = cfg.brokenPaths
// 	s.DuplicatePaths = cfg.duplicatePaths
// 	s.HaltedPaths = cfg.haltedPaths
//
// 	s.HaltReason = cfg.haltReason
//
// 	s.Infos = cfg.infos
// 	if len(cfg.infos) > 10 {
// 		s.Infos = cfg.infos[:10] // limit the output of infos
// 	}
// 	s.Profile = cfg.profile
//
// 	return s
// }

// func (cfg *CFG) SummaryMarkdown(path string) error {
// 	return nil
// }

// func (cfg *CFG) SummaryJSON(path string) error {
// 	type edge struct {
// 		Src int32  `json:"src"`
// 		Dst int32  `json:"dst"`
// 		Typ string `json:"type"`
// 	}
// 	type block struct {
// 		Index   int32    `json:"index"`
// 		Code    string   `json:"code,omitempty"`
// 		Preds   []edge   `json:"preds"`
// 		Succs   []edge   `json:"succs"`
// 		Offset  uint64   `json:"offset"`
// 		Stmts   []string `json:"stmts"`
// 		Visited int      `json:"visited"`
// 	}
// 	type probes struct {
// 		// ProbePairs   []compiler.ProbePair `json:"probe_pairs"`
// 		PairCovered  int                  `json:"pair_covered"`
// 		PairTotal    int                  `json:"pair_total"`
// 		ProbeCovered int                  `json:"probe_covered"`
// 		ProbeTotal   int                  `json:"probe_total"`
// 	}
// 	type export struct {
// 		Summary Summary `json:"summary"`
// 		Blocks  []block `json:"blocks"`
// 		Probe   probes  `json:"probes"`
// 	}
//
// 	blocks := make([]block, len(cfg.BasicBlocks))
// 	for i, b := range cfg.BasicBlocks {
// 		preds := make([]edge, 0)
// 		for _, p := range b.preds {
// 			if p.Dst == nil {
// 				continue
// 			}
// 			preds = append(preds, edge{
// 				Src: p.Src.index,
// 				Dst: p.Dst.index,
// 				Typ: p.Typ.String(),
// 			})
// 		}
// 		succs := make([]edge, 0)
// 		for _, p := range b.succs {
// 			if p.Dst == nil {
// 				continue
// 			}
// 			succs = append(succs, edge{
// 				Src: p.Src.index,
// 				Dst: p.Dst.index,
// 				Typ: p.Typ.String(),
// 			})
// 		}
// 		stmtsStr := make([]string, len(b.stmts))
// 		for j, s := range b.stmts {
// 			stmtsStr[j] = s.String()
// 		}
// 		blocks[i] = block{
// 			Index:   b.index,
// 			Code:    hex.EncodeToString(b.Assemble()),
// 			Preds:   preds,
// 			Succs:   succs,
// 			Offset:  b.offset,
// 			Stmts:   stmtsStr,
// 			Visited: b.visited,
// 		}
// 	}
// 	probe := probes{
// 		ProbePairs:   make([]compiler.ProbePair, 0),
// 		PairCovered:  0,
// 		PairTotal:    0,
// 		ProbeCovered: 0,
// 		ProbeTotal:   0,
// 	}
// 	// if len(cfg.ProbePairs) > 0 {
// 	// 	for _, pair := range cfg.ProbePairs {
// 	// 		probe.ProbeTotal += pair.TotalProbes()
// 	// 	}
// 	// 	for _, pair := range cfg.ProbePairs {
// 	// 		pair, paircovered, probecoverd := cfg.isCoveredProbePair(pair)
// 	// 		probe.ProbePairs = append(probe.ProbePairs, pair)
// 	// 		probe.PairTotal++
// 	// 		probe.PairCovered += paircovered
// 	// 		probe.ProbeCovered += probecoverd
// 	// 	}
// 	// }
// 	//
// 	summary := cfg.Summary()
// 	summary.PairCovered = probe.PairCovered
// 	summary.PairTotal = probe.PairTotal
// 	summary.ProbeCovered = probe.ProbeCovered
// 	summary.ProbeTotal = probe.ProbeTotal
//
// 	exportData := export{
// 		summary,
// 		blocks,
// 		probe,
// 	}
// 	data, err := json.MarshalIndent(exportData, "", "  ")
// 	if err != nil {
// 		return err
// 	}
//
// 	f, err := os.Create(path)
// 	if err != nil {
// 		return err
// 	}
//
// 	_, err = f.Write(data)
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }

func (cfg *CFG) GenerateReadableCFG(path string) error {
	var sb strings.Builder

	// sb.WriteString(cfg.Summary().String())
	for _, b := range cfg.BasicBlocks {
		haltFlag := ""
		if b.halt {
			haltFlag = "*"
		}
		fmt.Fprintf(&sb, "Block %d%s (offset: 0x%x, visited: %d):\n", b.index, haltFlag, b.offset, b.visited)
		for _, s := range b.stmts {
			sb.WriteString(s.String())
			sb.WriteString("\n")
		}
		for _, succ := range b.succs {
			sb.WriteString(succ.String())
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func (cfg *CFG) PrintReadableCFG() {
	for _, b := range cfg.BasicBlocks {
		haltFlag := ""
		if b.halt {
			haltFlag = "*"
		}
		fmt.Printf("Block %d%s (offset: 0x%x, visited: %d):\n", b.index, haltFlag, b.offset, b.visited)
		for _, s := range b.stmts {
			fmt.Println(s.String())
		}
		for _, succ := range b.succs {
			fmt.Println(succ.String())
		}
		fmt.Println()
	}
}

func (cfg *CFG) CFGBuilder() *CFGBuilder {
	if cfg.builder == nil {
		cfg.builder = NewCFGBuilder(cfg)
	}
	return cfg.builder
}

// containsJump checks whether the stack contains unreachable jumpdest
func (cfg *CFG) containsJump(st *Stack) (*BasicBlock, *Reg, bool) {
	unreachable := make(map[uint64]*BasicBlock)
	for _, bb := range cfg.BasicBlocks {
		if len(bb.preds) != 0 || bb.stmts[0].op != vm.JUMPDEST {
			continue
		}
		unreachable[bb.stmts[0].PC()] = bb
	}
	for _, reg := range st.items {
		if reg.inst == nil || reg.value() == nil {
			continue
		}
		valU64 := reg.value().Uint64()
		if bb, ok := unreachable[valU64]; ok {
			return bb, reg, true
		}
	}
	return nil, nil, false
}

// func (cfg *CFG) ExportCFG(path string) error {
// 	type block struct {
// 		Index   int32    `json:"index"`
// 		Preds   []int32  `json:"preds"`
// 		Offset  uint64   `json:"offset"`
// 		Stmts   []string `json:"stmts"`
// 		Visited int      `json:"visited"`
// 	}
//
// 	type graph struct {
// 		Blocks []block `json:"blocks"`
// 	}
//
// 	for _, b := range cfg.BasicBlocks {
// 		stmsStr := make([]string, len(b.stmts))
// 		for i, s := range b.stmts {
// 			stmsStr[i] = s.String()
// 		}
//
// 		blcok := block{
// 			Index: b.index,
// 		}
// 	}
// }

// func (cfg *CFG) availableHalt() bool {
// 	return true
// 	cover, total := cfg.coverage()
// 	return float64(cover)/float64(total) >= float64(0.90)
// }

// func (cfg *CFG) IsCoveredProbePair(pair compiler.ProbePair) (compiler.ProbePair, int, int) {
// 	return cfg.isCoveredProbePair(pair)
// }

// func (cfg *CFG) isCoveredProbePair(pair compiler.ProbePair) (compiler.ProbePair, int, int) {
// 	entry := pair.Entry
// 	exits := pair.Exits
// 	paircovered := 0
// 	probeCovered := 0
// 	// step 1 entry covered?
// 	in, _ := entry.Code()
// 	covered, inBBs := cfg.containProbeCode(in)
// 	if covered {
// 		pair.EntryCoverage = true
// 		probeCovered++
// 	}
//
// 	// step 2 exits covered?
// 	outBBs := make([]*BasicBlock, 0)
// 	for i, exit := range exits {
// 		_, out := exit.Code()
// 		covered, outBB := cfg.containProbeCode(out)
// 		if covered {
// 			pair.ExitCoverage[i] = true
// 			outBBs = append(outBBs, outBB...)
// 			probeCovered++
// 		}
// 	}
//
// 	// step 3 pair covered?
// 	// each exit basicblock is the successor of entry basicblock
// 	if inBBs == nil || outBBs == nil {
// 		return pair, paircovered, probeCovered
// 	}
//
// 	paircovered = 1
// 	for _, inBB := range inBBs {
// 		for _, outBB := range outBBs {
// 			if !inBB.containSuccessor(outBB) {
// 				paircovered = 0
// 				break
// 			}
// 		}
// 	}
// 	return pair, paircovered, probeCovered
// }

// func (cfg *CFG) containProbeCode(probe []byte) (bool, []*BasicBlock) {
// 	blocks := make([]*BasicBlock, 0)
// 	for _, b := range cfg.BasicBlocks {
// 		if b.containProbeCode(probe) {
// 			blocks = append(blocks, b)
// 		}
// 	}
// 	if len(blocks) > 0 {
// 		return true, blocks
// 	}
// 	return false, nil
// }
//
// func (cfg *CFG) SetProbePairs(pairs []compiler.ProbePair) {
// 	bytecode := hex.EncodeToString(cfg.bytecode)
//
// 	for _, pair := range pairs {
// 		entryFlag := false
// 		exitFlag := false
// 		in, _ := pair.Entry.Code()
// 		encodedIn := hex.EncodeToString(in)
// 		if strings.Contains(bytecode, encodedIn) {
// 			entryFlag = true
// 		}
// 		existsExits := make([]compiler.Probe, 0)
// 		existsExitsCoverage := make([]bool, 0)
// 		for _, exit := range pair.Exits {
// 			_, out := exit.Code()
// 			encodedOut := hex.EncodeToString(out)
// 			if strings.Contains(bytecode, encodedOut) {
// 				exitFlag = true
// 				existsExits = append(existsExits, exit)
// 				existsExitsCoverage = append(existsExitsCoverage, false)
// 			}
// 		}
// 		if entryFlag && exitFlag {
// 			cfg.ProbePairs = append(cfg.ProbePairs, compiler.ProbePair{
// 				Entry:         pair.Entry,
// 				EntryCoverage: false,
// 				Exits:         existsExits,
// 				ExitCoverage:  existsExitsCoverage,
// 			})
// 		}
// 	}
// }
