// Package db provides database schema definitions and utilities.
package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// FileRecord represents a single record for the files table.
type FileRecord struct {
	Address             string
	BinFile             string
	InstrumentedBinFile string
	ProbesFile          string
}

// filerecordhandlerimpl implements recordhandler[filerecord]
type fileRecordHandlerImpl struct{}

// NewFileTable creates a schema for a CFG statistics table for a specific tool.
// address: contract address on Ethereum mainnet
// bin_file: the binary file analyzed
// instrumented_bin_file: the instrumented binary file analyzed
// probes_file: the probes file used during instrumentation
func NewFileTable(name string) schema {
	if name == "" {
		name = "files"
	}
	// Create an instance of our specific handler for FileRecord
	handler := fileRecordHandlerImpl{}
	s, err := NewSchema[FileRecord](name, handler).AddColumn(
		NewColumn("address", TEXT).WithConstraint(NOT_NULL).PrimaryKey(),
	).AddColumn(
		NewColumn("bin_file", TEXT),
	).AddColumn(
		NewColumn("instrumented_bin_file", TEXT),
	).AddColumn(
		NewColumn("probes_file", TEXT),
	).Build()
	if err != nil {
		fmt.Printf("Error creating stat instrumented schema: %v\n", err)
	}
	return s
}

// Fetch implements the Fetch method of RecordHandler[FileRecord].
// args[0] is the top-level directory to scan for files.
// args[1] is subdirBin, args[2] is subdirInstrBin
func (h fileRecordHandlerImpl) Fetch(args ...any) ([]FileRecord, []map[string]any, error) {
	if len(args) < 3 {
		log.Printf("DEBUG: Insufficient arguments provided to Fetch: %d, expected 3", len(args))
		return nil, nil, fmt.Errorf("insufficient arguments to fetch file table")
	}

	var (
		dir            string
		subdirBin      string
		subdirInstrBin string
		ok             bool
	)
	if dir, ok = args[0].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for dir, got %T", args[0])
	}
	if subdirBin, ok = args[1].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for subdirBin, got %T", args[1])
	}
	if subdirInstrBin, ok = args[2].(string); !ok {
		return nil, nil, fmt.Errorf("expected string for subdirInstrBin, got %T", args[2])
	}

	type item struct { // Re-using item here for intermediate processing
		binFile      string
		instrBinFile string
		probesFile   string
	}
	mapping := make(map[string]item)

	// step 1 scan for original binaries
	log.Printf("DEBUG: Starting step 1: Scanning for original binaries... [1/3]")
	targetDir := filepath.Join(dir, subdirBin)
	log.Printf("DEBUG: Scanning targetDir for original binaries: %s", targetDir)
	subDirs, err := listDirectories(targetDir)
	if err != nil {
		log.Printf("DEBUG: Error listing directories in %s: %v", targetDir, err)
		return nil, nil, err
	}
	log.Printf("DEBUG: Found %d subdirectories in %s. Processing original binaries...", len(subDirs), targetDir)
	for i, subDir := range subDirs {
		// Simulate progress for step 1
		if (i+1)%10 == 0 || i == len(subDirs)-1 { // Log every 10 items or at the end
			log.Printf("DEBUG: Step 1 progress: Processed %d/%d original binaries.", i+1, len(subDirs))
		}
		address := filepath.Base(subDir)
		if _, exists := mapping[address]; !exists {
			mapping[address] = item{}
		}
		files, err := listFiles(subDir, ".bin-runtime")
		if err != nil {
			return nil, nil, err
		}
		if len(files) == 0 {
			continue
		}
		cur := mapping[address]
		cur.binFile = filepath.Join(subDir, files[0])
		mapping[address] = cur
	}
	log.Printf("DEBUG: Finished step 1. Mapping size: %d. [1/3]")

	// step 2 scan for instrumented binaries
	log.Printf("DEBUG: Starting step 2: Scanning for instrumented binaries... [2/3]")
	targetDir = filepath.Join(dir, subdirInstrBin)
	log.Printf("DEBUG: Scanning targetDir for instrumented binaries: %s", targetDir)
	subDirs, err = listDirectories(targetDir)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("DEBUG: Found %d subdirectories in %s. Processing instrumented binaries...", len(subDirs), targetDir)
	for i, subDir := range subDirs {
		// Simulate progress for step 2
		if (i+1)%10 == 0 || i == len(subDirs)-1 { // Log every 10 items or at the end
			log.Printf("DEBUG: Step 2 progress: Processed %d/%d instrumented binaries.", i+1, len(subDirs))
		}
		address := filepath.Base(subDir)
		// log.Printf("DEBUG: Processing subdirectory for instrumented binaries: %s, address: %s", subDir, address)
		if _, exists := mapping[address]; !exists {
			mapping[address] = item{}
			// log.Printf("DEBUG: New address '%s' added to mapping during instrumented scan.", address)
		}
		ds, err := listDirectories(subDir)
		if err != nil {
			// log.Printf("DEBUG: Error listing directories in %s: %v", subDir, err)
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

		probefile := filepath.Join(listdir, "probes.json")
		// log.Printf("DEBUG: Checking for probes file: %s", probefile)
		if _, err = os.Stat(probefile); os.IsNotExist(err) {
			continue // skip if probes file does not exist
		}
		// otherwise find the instrumented binary
		binfile, err := findRuntimeBinFiles(filepath.Join(listdir, "instrumented_output"))
		if err != nil {
			continue
		}
		cur := mapping[address]
		cur.instrBinFile = binfile
		cur.probesFile = probefile
		mapping[address] = cur
		// log.Printf("DEBUG: Found instrBinFile for address '%s': %s, probesFile: %s", address, cur.instrBinFile, cur.probesFile)
	}
	log.Printf("DEBUG: Finished step 2. Final mapping size: %d. [2/3]", len(mapping))

	// Convert mapping to []FileRecord
	var records []FileRecord
	var rawRecords []map[string]any
	log.Printf("DEBUG: Starting step 3: Converting mapping to FileRecords... [3/3]")
	// Calculate total items to process for final step's progress
	totalMappingItems := len(mapping)
	processedMappingItems := 0
	for addr, item := range mapping {
		processedMappingItems++
		if processedMappingItems%10 == 0 || processedMappingItems == totalMappingItems {
			log.Printf("DEBUG: Step 3 progress: Converted %d/%d items to FileRecords.", processedMappingItems, totalMappingItems)
		}

		records = append(records, FileRecord{
			Address:             addr,
			BinFile:             item.binFile,
			InstrumentedBinFile: item.instrBinFile,
			ProbesFile:          item.probesFile,
		})
		rawRecords = append(rawRecords, map[string]any{
			"address":               addr,
			"bin_file":              item.binFile,
			"instrumented_bin_file": item.instrBinFile,
			"probes_file":           item.probesFile,
		})
	}
	log.Printf("DEBUG: Total raw records created: %d", len(rawRecords))

	return records, rawRecords, nil
}
