package db

import (
	"fmt"
	"os"
	"strings"
)

// DatasetRecord define a dataset address, current dataset is DISL
type DatasetRecord struct {
	Addrss string
}

type datasetRecordHandlerImpl struct{}

func NewDatasetTable(name string) schema {
	if name == "" {
		return schema{}
	}

	handler := datasetRecordHandlerImpl{}
	s, err := NewSchema(name, handler).AddColumn(
		NewColumn("address", TEXT).PrimaryKey(),
	).Build()
	if err != nil {
		fmt.Printf("Error creating dataset schema: %v\n", err)
	}
	return s
}

// Fetch dataset addresses from file
// args[0] is dataset addresses file
func (h datasetRecordHandlerImpl) Fetch(args ...any) ([]DatasetRecord, []map[string]any, error) {
	if len(args) != 1 {
		return nil, nil, fmt.Errorf("invalid args: %v", args)
	}
	data, err := os.ReadFile(args[0].(string))
	if err != nil {
		return nil, nil, err
	}
	var records []DatasetRecord
	var raws []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		// Support 0x and 0X prefixes, or add if missing
		if !strings.HasPrefix(line, "0x") && !strings.HasPrefix(line, "0X") {
			line = "0x" + line
		}
		// Validation: Ethereum address is 20 bytes (40 hex chars) + 2 prefix chars = 42
		if len(line) != 42 {
			continue
		}
		records = append(records, DatasetRecord{Addrss: line})
		raws = append(raws, map[string]any{"address": line})
	}
	return records, raws, nil
}
