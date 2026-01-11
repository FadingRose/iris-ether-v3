package cfg

import (
	"fmt"
	"strings"
)

// Liveness tracks each basic block's liveness
type Liveness struct {
	lives  map[int32][]*live
	pruned map[int32]map[*Path]*live
}

type live struct {
	slots    map[int]string
	indicate map[int32]uint64 // map[D_Block]dest
}

func (l *live) String() string {
	var sb strings.Builder
	sb.WriteString("Slots:\n")
	for k, v := range l.slots {
		fmt.Fprintf(&sb, "  %d: %s\n", k, v)
	}
	fmt.Fprintf(&sb, "Indicate: %d\n", l.indicate)
	return sb.String()
}

func (l *live) isSame(other *live) bool {
	if len(l.slots) != len(other.slots) {
		return false
	}
	for k, v := range l.slots {
		if other.slots[k] != v {
			return false
		}
	}
	if len(l.indicate) != len(other.indicate) {
		return false
	}

	for k, v := range l.indicate {
		if other.indicate[k] != v {
			return false
		}
	}
	return true
}

func NewLiveness() *Liveness {
	return &Liveness{
		lives:  make(map[int32][]*live),
		pruned: make(map[int32]map[*Path]*live),
	}
}

// Check checks whether the stack match any liveness at the given block's input stack
func (l *Liveness) Check(st *Stack, path *Path, index int32) bool {
	var match bool
	for _, live := range l.lives[index] {
		m := true
		for pos, inst := range live.slots {
			reg := st.GetAtPosition(pos)
			if reg == nil || reg.inst == nil {
				m = false
				break
			}
			if reg.inst.String() != inst {
				m = false
				break
			}
		}
		match = match || m
		if match {
			l.pruned[index][path] = live
			break
		}
	}
	return match
}

func (l *Liveness) Live(bb *BasicBlock, slots map[int]string, D_Block int32, dest uint64) bool {
	if _, ok := l.lives[bb.index]; !ok {
		l.lives[bb.index] = make([]*live, 0)
	}
	if _, ok := l.pruned[bb.index]; !ok {
		l.pruned[bb.index] = make(map[*Path]*live)
	}

	nlive := &live{
		slots:    slots,
		indicate: map[int32]uint64{D_Block: dest},
	}
	// redundant
	for _, live := range l.lives[bb.index] {
		// fmt.Println("duplicate liveness")
		if live.isSame(nlive) {
			return false
		}
	}

	// fmt.Printf("Liveness: %s\n", nlive.String())
	l.lives[bb.index] = append(l.lives[bb.index], nlive)
	return true
}
