package db

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type StatInstrumentRecord struct {
	Address        string
	Tool           string
	ResultJSONPath string
	PairCovered    int
	PairTotal      int
	ProbeCovered   int
	ProbeTotal     int
	CoveredBlocks  int
	TotalBlocks    int
}

type statInstrumentRecordHandlerImpl struct{}

func NewStatInstrumentedTable(name string) schema {
	if name == "" {
		name = "stat_instrumented"
	}
	handler := statInstrumentRecordHandlerImpl{}
	s, err := NewSchema[StatInstrumentRecord](name, handler).AddColumn(
		NewColumn("address", TEXT).WithConstraint(NOT_NULL).PrimaryKey(),
	).AddColumn(
		NewColumn("tool", TEXT).WithConstraint(NOT_NULL).PrimaryKey(),
	).AddColumn(
		NewColumn("result_json_path", TEXT),
	).AddColumn(
		NewColumn("pair_covered", INT),
	).AddColumn(
		NewColumn("pair_total", INT),
	).AddColumn(
		NewColumn("probe_covered", INT),
	).AddColumn(
		NewColumn("probe_total", INT),
	).AddColumn(
		NewColumn("covered_blocks", INT),
	).AddColumn(
		NewColumn("total_blocks", INT),
	).Build()
	if err != nil {
		fmt.Printf("Error creating stat instrumented schema: %v\n", err)
	}
	return s
}

func (h statInstrumentRecordHandlerImpl) Fetch(args ...any) ([]StatInstrumentRecord, []map[string]any, error) {
	if len(args) < 3 {
		return nil, nil, fmt.Errorf("insufficient arguments to fetch file table")
	}

	var (
		dir            string
		subdirInstrBin string
		ok             bool
	)
	if dir, ok = args[0].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for dir, got %T", args[0])
	}
	if subdirInstrBin, ok = args[2].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for subdirInstrBin, got %T", args[2])
	}

	type item struct {
		resultJSONPath string
		pairCovered    int
		pairTotal      int
		probeCovered   int
		probeTotal     int
	}

	mapping := make(map[string]map[string]item)
	targetDir := filepath.Join(dir, subdirInstrBin)
	subDirs, err := listDirectories(targetDir)
	if err != nil {
		return nil, nil, err
	}

	for i, subDir := range subDirs {

		if (i+1)%10 == 0 || i == len(subDirs)-1 { // Log every 10 items or at the end
			log.Printf("DEBUG: Step 2 progress: Processed %d/%d instrumented binaries.", i+1, len(subDirs))
		}
		address := filepath.Base(subDir)

		if _, exists := mapping[address]; !exists {
			mapping[address] = make(map[string]item)
		}

		ds, err := listDirectories(subDir)
		if err != nil {
			return nil, nil, err
		}
		var listdir string
		for _, d := range ds {
			if strings.HasPrefix(filepath.Base(d), "instrumented_") {
				listdir = d
				// log.Printf("DEBUG: Found instrumented directory: %s", listdir)
				break
			}
		}
		if listdir == "" {
			// log.Printf("DEBUG: No 'instrumented_' prefixed directory found in %s. Skipping.", subDir)
			continue
		}
		jsonFiles, err := listFiles(listdir, "json")
		if err != nil {
			continue
		}
		resultJSON, tool := func() (string, string) {
			for _, f := range jsonFiles {
				// valid file name
				// instrumented_TexasTITS_0x0a0a81cb2e91cd69f852354d7ed99d51c50cb3fa.iris-ether.json
				parts := strings.Split(f, ".")
				if len(parts) != 3 {
					continue
				}
				subparts := strings.Split(parts[0], "_")
				if len(subparts) != 3 {
					continue
				}
				if subparts[0] != "instrumented" || subparts[2] != address {
					continue
				}
				return f, parts[1]
			}
			return "", ""
		}()
		if resultJSON == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(listdir, resultJSON))
		if err != nil {
			continue
		}

		var v map[string]any
		err = json.Unmarshal(data, &v)
		if err != nil {
			continue
		}

		var summary map[string]any
		if summary, ok = v["summary"].(map[string]any); !ok {
			log.Printf("DEBUG: 'summary' key not found or not a map[string]any in JSON from %s. Skipping.", resultJSON)
			continue
		}
		// Extract values from the summary map
		var currentItem item
		// JSON numbers are unmarshaled as float64 by default, so we assert to float64 and then convert to int
		if pairCovered, ok := summary["pair_covered"].(float64); ok {
			currentItem.pairCovered = int(pairCovered)
		}
		if pairTotal, ok := summary["pair_total"].(float64); ok {
			currentItem.pairTotal = int(pairTotal)
		}
		if probeCovered, ok := summary["probe_covered"].(float64); ok {
			currentItem.probeCovered = int(probeCovered)
		}
		if probeTotal, ok := summary["probe_total"].(float64); ok {
			currentItem.probeTotal = int(probeTotal)
		}

		currentItem.resultJSONPath = resultJSON

		if _, exists := mapping[address]; !exists {
			mapping[address] = make(map[string]item)
		}
		mapping[address][tool] = currentItem
	}

	// Convert mapping to []StatInstrumentRecord and []map[string]any
	var records []StatInstrumentRecord
	var rawRecords []map[string]any

	for addr, toolItems := range mapping {
		for tool, item := range toolItems {
			records = append(records, StatInstrumentRecord{
				Address:        addr,
				Tool:           tool,
				ResultJSONPath: item.resultJSONPath,
				PairCovered:    item.pairCovered,
				PairTotal:      item.pairTotal,
				ProbeCovered:   item.probeCovered,
				ProbeTotal:     item.probeTotal,
			})
			rawRecords = append(rawRecords, map[string]any{
				"address":          addr,
				"tool":             tool,
				"result_json_path": item.resultJSONPath,
				"pair_covered":     item.pairCovered,
				"pair_total":       item.pairTotal,
				"probe_covered":    item.probeCovered,
				"probe_total":      item.probeTotal,
			})
		}
	}

	log.Printf("DEBUG: Total stat instrument records created: %d", len(rawRecords))

	return records, rawRecords, nil
}
