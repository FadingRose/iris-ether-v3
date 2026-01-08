package cfg

import "fmt"

type SuccType int

const (
	Any SuccType = iota

	JumpITrue
	JumpIFalse
	Jump
	DirectLink
	Oracle
	Symbolic // dest relies on symbolic resolving
	ContextAware

	Broken
	Leverage  // indicates header to tail
	Arbitrary // indicates inner block, our work do NOT use this, for adapt to other tool-chain convert to *CFG
)

type Edge struct {
	Src          *BasicBlock
	Dst          *BasicBlock
	Typ          SuccType
	VisitCount   int
	DiscoverPath string
	LastPaths    []*Path
}

func (e Edge) String() string {
	indent := "			"
	if e.Dst != nil {
		if e.VisitCount > 100 {
			lastpaths := ""
			e.LastPaths = e.LastPaths[len(e.LastPaths)-10:]
			for _, p := range e.LastPaths {
				lastpaths += p.String() + "\n"
			}
			return fmt.Sprintf("%s%d -> %d, !%s [%d] %s\n%s", indent, e.Src.index, e.Dst.index, e.Typ.String(), e.VisitCount, e.DiscoverPath, lastpaths)
		} else {
			return fmt.Sprintf("%s%d -> %d, %s [%d] %s", indent, e.Src.index, e.Dst.index, e.Typ.String(), e.VisitCount, e.DiscoverPath)
		}
	}
	return fmt.Sprintf("%s%d -> nil, %s", indent, e.Src.index, e.Typ.String())
}

func (s SuccType) String() string {
	switch s {
	case Any:
		return "Any"
	case JumpITrue:
		return "JumpITrue"
	case JumpIFalse:
		return "JumpIFalse"
	case Jump:
		return "Jump"
	case DirectLink:
		return "DirectLink"
	case Oracle:
		return "Oracle"
	case Symbolic:
		return "Symbolic"
	case ContextAware:
		return "ContextAware"
	case Broken:
		return "Broken"
	case Leverage:
		return "Leverage"
	case Arbitrary:
		return "Arbitrary"
	default:
		return "Unknown"
	}
}

func ToEdgeType(s string) SuccType {
	switch s {
	case "Any":
		return Any
	case "JumpITrue":
		return JumpITrue
	case "JumpIFalse":
		return JumpIFalse
	case "Jump":
		return Jump
	case "DirectLink":
		return DirectLink
	case "Oracle":
		return Oracle
	case "Symbolic":
		return Symbolic
	case "ContextAware":
		return ContextAware
	case "Broken":
		return Broken
	case "Leverage":
		return Leverage
	case "Arbitrary":
		return Arbitrary
	default:
		return Any
	}
}
