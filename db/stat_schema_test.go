package db

import (
	"fmt"
	"testing"
)

func TestStatSchema(t *testing.T) {
	schema := NewStatTable("")
	args := []any{"../../dataset/", "DISL", "DISL_source_contract"}
	fmt.Println("Fetching stat schema data...")
	schema.fetch(args)
}
