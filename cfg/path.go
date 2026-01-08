package cfg

import (
	"fadingrose/iris-ether-v3/vm"
	"fmt"
	"strings"
)

type Path struct {
	data []*BasicBlock

	parent   *Path
	children []*Path

	terminated bool
}

func NewPath(data *BasicBlock) *Path {
	return &Path{
		data: []*BasicBlock{data},

		parent:   nil,
		children: make([]*Path, 0),

		terminated: false,
	}
}

func (p *Path) weight() int {
	var weight int
	// how many uncovered block in this path
	for _, b := range p.data {
		for _, inst := range b.stmts {
			if inst.op.IsPush() {
				dest := inst.Value()
				if dest != nil {
					destU64 := dest.Uint64()
					if b.cfg.validDestMap[inst.offset][destU64] != nil {
						if b.cfg.validDestMap[inst.offset][destU64].visited == 0 {
							weight++
						}
					}
				}
			}
		}
	}
	return weight
}

func (p *Path) regs() []*Reg {
	rs := make([]*Reg, 0)
	for _, b := range p.data {
		rs = append(rs, b.regs()...)
	}
	return rs
}

func (p *Path) fork() (p1 *Path, p2 *Path) {
	p1 = p
	p2 = &Path{
		data: make([]*BasicBlock, len(p.data)),
	}
	copy(p2.data, p1.data)
	return p1, p2
}

func (p *Path) fork2() (p1 *Path, p2 *Path) {
	p1 = &Path{
		data:       make([]*BasicBlock, len(p.data)),
		parent:     p,
		children:   make([]*Path, 0),
		terminated: false,
	}
	p2 = &Path{
		data:       make([]*BasicBlock, len(p.data)),
		parent:     p,
		children:   make([]*Path, 0),
		terminated: false,
	}
	copy(p1.data, p.data)
	copy(p2.data, p.data)

	p.children = append(p.children, p1, p2)
	return p1, p2
}

func (p *Path) fork1() (p1 *Path) {
	p1 = &Path{
		data:       make([]*BasicBlock, len(p.data)),
		parent:     p,
		children:   make([]*Path, 0),
		terminated: false,
	}
	copy(p1.data, p.data)
	p.children = append(p.children, p1)
	return p1
}

func (p *Path) copy() *Path {
	p1 := &Path{
		data:     make([]*BasicBlock, len(p.data)),
		parent:   p.parent,
		children: make([]*Path, 0),
	}
	copy(p1.data, p.data)
	copy(p1.children, p.children)
	return p1
}

func (p *Path) push(b *BasicBlock) {
	p.data = append(p.data, b)
}

func (p *Path) last() *BasicBlock {
	return p.data[len(p.data)-1]
}

func (p *Path) lastsecond() *BasicBlock {
	if len(p.data) < 2 {
		return nil
	}
	return p.data[len(p.data)-2]
}

func (p *Path) terminate() {
	p.terminated = true

	// recursively terminate parent paths
	current := p.parent
	flag := true
	for flag && current != nil {
		for _, child := range current.children {
			if child.terminated == false {
				flag = false
				break
			}
		}
		if flag {
			current.terminated = true
			current = current.parent
		}
	}
}

func (p *Path) isLeadingLoop() (bool, int) {
	last, nxt := p.data[len(p.data)-2], p.data[len(p.data)-1]
	for i := 0; i < len(p.data)-1; i++ {
		if p.data[i].index == last.index && p.data[i+1].index == nxt.index {
			return true, i
		}
	}
	return false, -1
}

func (p *Path) isMultiExitLoop(start int) (bool, []*BasicBlock) {
	end, _ := len(p.data)-1, len(p.data)
	if start < 0 || start >= end {
		return false, nil
	}

	potentialExits := make([]*BasicBlock, 0)
	visited := make(map[int32]bool)
	for i := start + 1; i < end; i++ {
		lastop := p.data[i].LastStmt().op
		inst := p.data[i].LastSecondStmt()
		if inst == nil {
			continue
		}
		lastsecond := inst.op
		if lastsecond.IsPush() {
			// fmt.Println("* inst:", inst.String())
			continue
		}
		if lastop == vm.JUMP || lastop == vm.JUMPI {
			if visited[p.data[i].index] {
				continue
			}
			potentialExits = append(potentialExits, p.data[i])
			visited[p.data[i].index] = true
		}
	}

	if len(potentialExits) == 0 {
		return false, nil
	}

	return true, potentialExits
}

func (p *Path) String() string {
	var sb strings.Builder
	cnt := make(map[int32]int, 0)
	for _, b := range p.data {
		cnt[b.index]++
	}
	for i, b := range p.data {
		flag := ""
		if b.halt {
			flag += "*"
		}
		if b.isContextAware() {
			flag += "^"
		}
		if cnt[b.index] > 1 {
			fmt.Fprintf(&sb, "[%d]", b.index)
		} else {
			fmt.Fprintf(&sb, "%d", b.index)
		}
		sb.WriteString(flag)
		if i != len(p.data)-1 {
			sb.WriteString(" -> ")
		}
	}

	return sb.String()
}

func (p *Path) Halt() []*BasicBlock {
	// for the end to the start, for each JUMPI block, DFST it
	halts := make([]*BasicBlock, 0)
	for i := len(p.data) - 1; i >= 0; i-- {
		if p.data[i].LastStmt().op == vm.JUMPI {
			if !p.data[i].isLeadingHalt() {
				// if the children blocks are NOT leading definite halt, the path blocks can not be leading halt
				break
			} else {
				halts = append(halts, p.data[i])
			}
		}
	}

	return halts
}

func (p *Path) containsHalt() bool {
	for _, b := range p.data {
		if b.halt {
			return true
		}
	}
	return false
}
