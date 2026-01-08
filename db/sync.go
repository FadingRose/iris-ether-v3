package db

import (
	"fmt"
	"log"
)

var schemas = []schema{
	NewFileTable("files"),
	// NewStatTable("stat_base"),
	// NewStatInstrumentedTable("stat_instrumented"),
}

var tables = map[string]func(string) schema{
	// "files":             func(s string) schema { return NewFileTable(s) },
	// "stat_base":         NewStatTable("stat_base"),
	// "stat_instrumented": NewStatInstrumentedTable("stat_instrumented"),
	"dataset": func(s string) schema { return NewDatasetTable(s) },
}

func SyncOnce(path string, table string, args ...any) error {
	db, err := OpenOrCreateDBFromFile(path)
	if err != nil {
		return err
	}

	switch table {
	case "dataset":
		// args[0] is dataset name
		// args[1] is dataset addresses file
		err = db.CreateTableIfNotExists(tables[table](args[0].(string)))
		if err != nil {
			return err
		}
		schema := tables[table](args[0].(string))
		raw, err := schema.fetch(args[1].(string))
		if err != nil {
			return err
		}
		for _, values := range raw {
			db.InsertOrUpdate(schema, values, true) // always override when constructing dataset
		}
	default:
		return fmt.Errorf("unknown table: %s", table)
	}
	return nil
}

func Sync(path string, override bool, args ...any) error {
	db, err := OpenOrCreateDBFromFile(path)
	if err != nil {
		return err
	}

	// ensure all tables are created
	for _, schema := range schemas {
		err = db.CreateTableIfNotExists(schema)
		if err != nil {
			return err
		}
	}

	// sync all schemas
	for _, schema := range schemas {
		fmt.Printf("Syncing schema for table: %s\n", schema.tableName)
		raw, err := schema.fetch(args...)
		if err != nil {
			return err
		}
		log.Printf("Syncing data to database: \n%s", schema.tableName)
		for i, value := range raw {
			if i%100 == 0 || i == len(raw)-1 {
				log.Printf("Sync progress: %d/%d", i+1, len(raw))
			}
			db.InsertOrUpdate(schema, value, override)
		}
	}
	return nil
}
