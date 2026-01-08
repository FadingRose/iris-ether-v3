package db

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type StatRecord struct {
	Address        string
	Tool           string
	ResultJSONPath string
	CoveredBlocks  int
	TotalBlocks    int
	EdgeNums       int
	Time           int
	Mem            int
	Profile        []float64
	HaltReason     string
}

type statRecordHandlerImpl struct{}

func GetStatSchema(name string) schema {
	if strings.HasPrefix(name, "stat_instrumented") {
		return NewStatInstrumentedTable(name)
	}
	switch name {
	case "stat_base":
		return NewStatTable(name)
	case "stat_instrument":
		return NewStatInstrumentedTable(name)
	default:
		fmt.Printf("unsupported schema: %s\n", name)
		panic("unsupported schema")
	}
}

func NewStatTable(name string) schema {
	if name == "" {
		name = "stat_base"
	}

	handler := statRecordHandlerImpl{}
	s, err := NewSchema[StatRecord](name, handler).AddColumn(
		NewColumn("address", TEXT).WithConstraint(NOT_NULL).PrimaryKey(),
	).AddColumn(
		NewColumn("tool", TEXT).WithConstraint(NOT_NULL).PrimaryKey(),
	).AddColumn(
		NewColumn("result_json_path", TEXT),
	).AddColumn(
		NewColumn("covered_blocks", INT),
	).AddColumn(
		NewColumn("total_blocks", INT),
	).AddColumn(
		NewColumn("edge_nums", INT),
	).AddColumn(
		NewColumn("time", INT),
	).AddColumn(
		NewColumn("mem", INT),
	).AddColumn(
		NewColumn("profile", INT_ARRAY),
	).AddColumn(
		NewColumn("halt_reason", TEXT),
	).Build()
	if err != nil {
		fmt.Printf("Error creating stat instrumented schema: %v\n", err)
	}
	return s
}

func (h statRecordHandlerImpl) Fetch(args ...any) ([]StatRecord, []map[string]any, error) {
	if len(args) < 3 {
		return nil, nil, fmt.Errorf("insufficient arguments to fetch file table")
	}

	var (
		dir       string
		subdirBin string
		ok        bool
	)
	if dir, ok = args[0].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for dir, got %T", args[0])
	}
	if subdirBin, ok = args[1].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for subdirInstrBin, got %T", args[2])
	}

	type item struct {
		resultJSONPath string
		coveredBlocks  int
		totalBlocks    int
		edgeNums       int
		time           int
		mem            int
		profile        []float64
		haltReason     string
	}

	mapping := make(map[string]map[string]item)

	targetDir := filepath.Join(dir, subdirBin)
	if subDirs, err := listDirectories(targetDir); err != nil {
		return nil, nil, err
	} else {
		for i, subDir := range subDirs {
			if (i+1)%10 == 0 || i == len(subDirs)-1 { // Log every 10 items or at the end
				log.Printf("DEBUG: Processed %d/%d original binaries.", i+1, len(subDirs))
			}
			address := filepath.Base(subDir)
			if _, exists := mapping[address]; !exists {
				mapping[address] = make(map[string]item)
			}
			jsonFiles, err := listFiles(subDir, "json")
			if err != nil || len(jsonFiles) == 0 {
				continue
			}
			for _, jsonFile := range jsonFiles {
				parts := strings.Split(jsonFile, ".")
				if len(parts) < 3 || parts[0] != address {
					continue
				}
				tool := parts[1]
				jsonFilePath := filepath.Join(subDir, jsonFile)
				if data, err := os.ReadFile(jsonFilePath); err != nil {
					continue
				} else {
					var v map[string]any
					if err = json.Unmarshal(data, &v); err != nil {
						continue
					}
					var summary map[string]any
					if summary, ok = v["summary"].(map[string]any); !ok {
						log.Printf("DEBUG: 'summary' key not found or not a map[string]any in JSON from %s. Skipping.", jsonFilePath)
						continue
					}

					var currentItem item
					// JSON numbers are unmarshaled as float64 by default, so we assert to float64 and then convert to int
					if coveredBlocks, ok := summary["covered_blocks"].(float64); ok {
						currentItem.coveredBlocks = int(coveredBlocks)
					}
					if totalBlocks, ok := summary["total_blocks"].(float64); ok {
						currentItem.totalBlocks = int(totalBlocks)
					}
					if edgeNums, ok := summary["edge_nums"].(float64); ok {
						currentItem.edgeNums = int(edgeNums)
					}
					if time, ok := summary["time"].(float64); ok {
						currentItem.time = int(time)
					}
					if time, ok := summary["mem"].(float64); ok {
						currentItem.time = int(time)
					}
					if profile, ok := summary["profile"].([]any); ok {
						currentItem.profile = make([]float64, len(profile))
						for i, v := range profile {
							currentItem.profile[i] = v.(float64)
						}
					}
					if haltReason, ok := summary["halt_reason"].(string); ok {
						currentItem.haltReason = haltReason
					}

					currentItem.resultJSONPath = jsonFilePath
					if _, exists := mapping[address]; !exists {
						mapping[address] = make(map[string]item)
					}
					mapping[address][tool] = currentItem
				}
			}
		}
	}

	// Convert mapping to []StatInstrumentRecord and []map[string]any
	var records []StatRecord
	var rawRecords []map[string]any

	for addr, toolItems := range mapping {
		for tool, item := range toolItems {
			records = append(records, StatRecord{
				Address:        addr,
				Tool:           tool,
				ResultJSONPath: item.resultJSONPath,
				CoveredBlocks:  item.coveredBlocks,
				TotalBlocks:    item.totalBlocks,
				EdgeNums:       item.edgeNums,
				Time:           item.time,
				Mem:            item.mem,
				Profile:        item.profile,
				HaltReason:     item.haltReason,
			})
			rawRecords = append(rawRecords, map[string]any{
				"address":          addr,
				"tool":             tool,
				"result_json_path": item.resultJSONPath,
				"covered_blocks":   item.coveredBlocks,
				"total_blocks":     item.totalBlocks,
				"edge_nums":        item.edgeNums,
				"time":             item.time,
				"mem":              item.mem,
				"profile": func(vs []float64) string {
					if bs, err := json.Marshal(vs); err != nil {
						return "[]"
					} else {
						return string(bs)
					}
				}(item.profile),
				"halt_reason": item.haltReason,
			})
		}
	}

	log.Printf("DEBUG: Total stat original binaries records created: %d", len(rawRecords))

	return records, rawRecords, nil
}
