package cfg

import (
	"encoding/hex"
	"fmt"
	"strings"

	"fadingrose/iris-ether-v3/vm"
)

type BasicBlock struct {
	succs []*Edge
	preds []*Edge
	next  *BasicBlock
	index int32
	cfg   *CFG
	stmts instructions

	offset uint64

	visited int
	// entry  *BasicBlock

	// dfpos int32 // DFSP num
	halt bool
}

func NewBasicBlock(index int32, cfg *CFG, offset uint64) *BasicBlock {
	return &BasicBlock{
		succs:   make([]*Edge, 0),
		preds:   make([]*Edge, 0),
		next:    nil,
		index:   index,
		cfg:     cfg,
		stmts:   make(instructions, 0),
		offset:  offset,
		visited: 0,
		halt:    false,
	}
}

func (b *BasicBlock) Pc() uint64 {
	return b.stmts[0].pc
}

func (b *BasicBlock) LastStmt() instruction {
	return *b.stmts[len(b.stmts)-1]
}

func (b *BasicBlock) LastSecondStmt() *instruction {
	n := len(b.stmts)
	if n < 2 {
		return nil
	}
	return b.stmts[n-2]
}

func (b *BasicBlock) AppendSuccessor(succ *BasicBlock, typ SuccType, path *Path) bool {
	// b.visited++
	// succ.visited++
	if _, ok := b.appendSuccessor(succ, typ, nil); ok && path != nil {
		// fmt.Printf("%s\nEdge: %s\n", path.String(), edge.String())
		return true
	} else {
		return false
	}
	// return b.appendSuccessor(succ, typ, nil)
}

func (b *BasicBlock) appendSuccessor(succ *BasicBlock, typ SuccType, path *Path) (*Edge, bool) {
	if succ == nil {
		if typ == Any {
			b.succs = append(b.succs, &Edge{Src: b, Dst: succ, Typ: typ})
			return nil, false
		}

		// fmt.Printf("Warning: trying to add nil successor to Block %d with type %v\n", b.index, typ)
		return nil, false
	}

	for _, edge := range b.succs {
		if edge.Dst == nil {
			continue
		}
		if edge.Dst.index == succ.index {
			edge.VisitCount++
			edge.LastPaths = append(edge.LastPaths, path)
			return nil, false
		}
	}

	// if we reach here, the edge is new and not nil
	pathStr := ""
	if path != nil {
		pathStr = path.String()
	}
	edge := &Edge{Src: b, Dst: succ, Typ: typ, VisitCount: 1, DiscoverPath: pathStr, LastPaths: make([]*Path, 0)}
	b.succs = append(b.succs, edge)
	succ.preds = append(succ.preds, &Edge{Src: b, Dst: succ, Typ: typ, VisitCount: 1, DiscoverPath: pathStr, LastPaths: make([]*Path, 0)})

	return edge, true
}

func (b *BasicBlock) isContextAware() bool {
	if (b.LastStmt().op == vm.JUMP || b.LastStmt().op == vm.JUMPI) &&
		(b.LastSecondStmt() == nil || !b.LastSecondStmt().op.IsPush()) {
		return true
	}
	return false
}

var debugLeadingHalt = false

// isLeadingHalt checks whether the basic block leading halt for each possible execution path in future
func (b *BasicBlock) isLeadingHalt() bool {
	debug := debugLeadingHalt
	debugPrint := func(format string, a ...interface{}) {
		if debug {
			fmt.Printf(format+"\n", a...)
		}
	}

	// DFST from this block to all its successors
	visited := make(map[int32]bool)

	debugPrint("=======================================")
	debugPrint("Checking leading halt for Block %d", b.index)
	var ishalt func(bb *BasicBlock) bool
	ishalt = func(bb *BasicBlock) bool {
		visited[bb.index] = true
		if bb.halt {
			debugPrint("Block %d already marked halt", bb.index)
			return true
		}
		if bb.LastStmt().op.IsHalt() {
			debugPrint("Block %d is halt", bb.index)
			bb.halt = true
			return true
		}
		if bb.isContextAware() {
			debugPrint("		Block %d is context aware, not halt", bb.index)
			// halt := false
			return false
			// for _, edge := range bb.succs {
			// 	if edge.Dst == nil {
			// 		continue
			// 	}
			// 	if !edge.Dst.halt {
			// 		halt = false
			// 		break
			// 	}
			// }
			// if halt {
			// 	debugPrint("Block %d is context aware but all successors halt, so it halts", bb.index)
			// }
			// return halt
			// return false
		}
		halts := true
		for _, edge := range bb.succs {
			if edge.Dst == nil && edge.Typ == Any {
				debugPrint("Block %d has Any successor, skip", bb.index)
				halts = false
				continue
			}
			if edge.Dst == nil && edge.Typ != Any {
				debugPrint("Block %d has unknown successor, skip", bb.index)
				continue
			}
			if edge.Dst.index == b.index {
				debugPrint("Block %d has self-loop, skip", bb.index)
				continue
			}
			if _, ok := visited[edge.Dst.index]; ok {
				debugPrint("Block %d already visited, break", edge.Dst.index)
				halts = false
				break
			}
			if !ishalt(edge.Dst) {
				debugPrint("Block %d is not halt, so Block %d is not halt", edge.Dst.index, bb.index)
				halts = false
				break
			}
		}
		return halts
	}
	if ishalt(b) {
		b.halt = true
		debugPrint("Block %d is halt", b.index)
	} else {
		if b.halt {
			fmt.Printf("Unmarking halt for Block %d\n", b.index)
		}
		debugPrint("Block %d is NOT halt", b.index)
		b.halt = false
	}

	return b.halt
}

func (b *BasicBlock) Assemble() []byte {
	var code []byte
	for _, stmt := range b.stmts {
		code = append(code, byte(stmt.op))

		if stmt.op.IsPush() {
			// Calculate required size for this PUSH operation (PUSH1 = 1 byte, PUSH2 = 2 bytes, etc)
			pushSize := int(stmt.op - vm.PUSH1 + 1)

			// Create padded argument
			paddedArg := make([]byte, pushSize)
			// Copy original argument from right to left (maintaining big-endian order)
			copy(paddedArg[pushSize-len(stmt.arg):], stmt.arg)

			code = append(code, paddedArg...)
		}
	}
	return code
}

func (b *BasicBlock) containProbeCode(probe []byte) bool {
	code := b.Assemble()
	if len(code) < len(probe) {
		return false
	}
	codeStr, probeStr := hex.EncodeToString(code), hex.EncodeToString(probe)
	return strings.Contains(codeStr, probeStr)
}

func (b *BasicBlock) containSuccessor(out *BasicBlock) bool {
	in := b
	visited := make(map[int32]bool)
	queue := []*BasicBlock{in}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visited[curr.index] = true
		if curr.index == out.index {
			return true
		}
		for _, edge := range curr.succs {
			if edge.Dst == nil {
				continue
			}
			if _, ok := visited[edge.Dst.index]; ok {
				continue
			}
			queue = append(queue, edge.Dst)
		}
	}
	return false
}

func (b *BasicBlock) regs() []*Reg {
	rs := make([]*Reg, 0, len(b.stmts))
	for _, inst := range b.stmts {
		rs = append(rs, &Reg{
			inst:    inst,
			params:  make([]*Reg, 0),
			dupFrom: nil,
			exec:    Executor(inst.op),
			cfg:     b.cfg,
		})
	}
	return rs
}

func (b *BasicBlock) String() string {
	var sb strings.Builder
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
	return sb.String()
}
