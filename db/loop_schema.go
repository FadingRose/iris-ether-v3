package db

import "fmt"

type LoopRecord struct {
	// TotalLoops (L_all): Total count of identified loops (Natural + Irreducible).
	TotalLoops int `json:"total_loops"`

	// NaturalLoops (L_natural): Number of loops defined by a back-edge to a dominator.
	// These loops have a single entry point (header).
	NaturalLoops int `json:"natural_loops"`

	// IrreducibleLoops (L_irreducible): Number of SCCs with multiple entry points.
	IrreducibleLoops int `json:"irreducible_loops"`

	// MaxDepth: The maximum nesting level of natural loops.
	MaxDepth int `json:"max_depth"`

	// LoopSizes: A list containing the number of basic blocks in each detected Natural Loop.
	LoopSizes []int `json:"loop_sizes"`

	// NaturalLoopDepths: The nesting depth of each Natural Loop corresponding to LoopSizes.
	NaturalLoopDepths []int `json:"natural_loop_depths"`

	// IrreducibleLoopSizes: A list containing the number of basic blocks in each detected Irreducible Loop (SCC).
	IrreducibleLoopSizes []int `json:"irreducible_loop_sizes"`

	// IrreducibleLoopDepths: The nesting depth of each Irreducible Loop corresponding to IrreducibleLoopSizes.
	// Depth is calculated as 1 + (number of Natural Loops fully containing this SCC).
	IrreducibleLoopDepths []int `json:"irreducible_loop_depths"`
}

type loopRecordHandlerImpl struct{}

func NewLoopTable(name string) schema {
	if name == "" {
		name = "loop"
	}
	handler := loopRecordHandlerImpl{}
	s, err := NewSchema[LoopRecord](name, handler).AddColumn(
		NewColumn(
			"addresses", TEXT).PrimaryKey().WithConstraint(NOT_NULL)).AddColumn(
		NewColumn("total_loops", INT).WithConstraint(NOT_NULL)).AddColumn(

		NewColumn("natural_loops", INT).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("irreducible_loops", INT).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("max_depth", INT).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("loop_sizes", JSON).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("natural_loop_depths", JSON).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("irreducible_loop_sizes", JSON).WithConstraint(NOT_NULL),
	).AddColumn(
		NewColumn("irreducible_loop_depths", JSON).WithConstraint(NOT_NULL),
	).Build()
	if err != nil {
		fmt.Printf("Error creating stat domination schema: %v\n", err)
	}
	return s
}

func (h loopRecordHandlerImpl) Fetch(args ...any) ([]LoopRecord, []map[string]any, error) {
	return nil, nil, fmt.Errorf("not implemented")
}
