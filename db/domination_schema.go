package db

import "fmt"

type DominationRecord struct {
	// TotalBlocks (V_all): Total number of reachable basic blocks in the CFG.
	TotalBlocks int

	// DynamicEdges (E_dynamic): Number of control flow edges derived from context-dependent jumps
	// (excluding static fall-throughs from JUMPI).
	DynamicEdges int
	// DynamicTargets (V_target): Number of unique basic blocks targeted by dynamic edges.
	DynamicTargets int

	// DominatedSize maps a Dynamic Target's index to its domination scope size.
	// DomSize(v): How many blocks become unreachable if target v is missed.
	DominatedSize map[int32]int

	// TotalDominatedLoad is the sum of DominatedSize for all dynamic targets.
	TotalDominatedLoad int
}

type dominationRecordHanderImpl struct{}

func NewDominationTable(name string) schema {
	if name == "" {
		name = "domination"
	}
	handler := dominationRecordHanderImpl{}
	s, err := NewSchema[DominationRecord](name, handler).AddColumn(
		NewColumn(
			"addreses", TEXT).PrimaryKey().PrimaryKey().WithConstraint(NOT_NULL)).AddColumn(
		NewColumn("total_blocks", INT).WithConstraint(NOT_NULL)).AddColumn(

		NewColumn("dynamic_edges", INT).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("dynamic_targets", INT).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("dominated_size", JSON).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("total_dominated_load", INT).WithConstraint(NOT_NULL),
	).Build()
	if err != nil {
		fmt.Printf("Error creating stat domination schema: %v\n", err)
	}
	return s
}

func (h dominationRecordHanderImpl) Fetch(args ...any) ([]DominationRecord, []map[string]any, error) {
	return nil, nil, fmt.Errorf("not implemented")
}
