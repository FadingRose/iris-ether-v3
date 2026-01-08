package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	*sql.DB
}

// OpenOrCreateDBFromFile opens a database connection based on a file file.
func OpenOrCreateDBFromFile(path string) (*Database, error) {
	if path == "" {
		path = "./database.db"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		err := os.MkdirAll(dir, 0o755) // Create directory with read/write/execute for owner, read/execute for others
		if err != nil {
			return nil, err
		}
		_, err = os.Create(path)
		if err != nil {
			return nil, err
		}
	}

	sqlDB, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify the connection
	if err = sqlDB.Ping(); err != nil {
		// sqlDB.Close() // Close the connection if ping fails
		return nil, err
	}

	return &Database{sqlDB}, nil
}

// CreateTableIfNotExists creates a table based on the provided schema.
func (d *Database) CreateTableIfNotExists(schema schema) error {
	query, err := schema.create()
	if err != nil {
		return fmt.Errorf("failed to generate create table query: %w", err)
	}

	_, err = d.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", schema.tableName, err)
	}
	return nil
}

// InsertOrUpdate inserts or updates a record in the table based on the provided schema and values.
// if override is true, existing records will be replaced.
func (d *Database) InsertOrUpdate(schema schema, values map[string]any, override bool) error {
	query, args, err := schema.insert(values, override)
	if err != nil {
		return fmt.Errorf("failed to generate insert query: %w", err)
	}

	_, err = d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert into table %s: %w", schema.tableName, err)
	}
	return nil
}

// ListSchemas returns a list of all table names in the database.
func (d *Database) ListSchemas() ([]string, error) {
	rows, err := d.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan schema name: %w", err)
		}
		schemas = append(schemas, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating schemas: %w", err)
	}

	return schemas, nil
}

// GetTableDetails retrieves detailed structural information for a specific table.
func (d *Database) GetTableDetails(tableName string) (string, error) {
	// ColumnDetails represents the metadata of a database column in SQLite.
	type ColumnDetails struct {
		ID           int
		Name         string
		Type         string
		NotNull      bool
		DefaultValue *string
		PK           int // 0 means not a PK, >0 indicates the column index in the PK
	}
	// Use fmt.Sprintf safely here because PRAGMA statements usually don't support parameter binding for table names in standard drivers.
	// %q helps quote the table name to prevent basic SQL injection if tableName contains spaces or special characters.
	query := fmt.Sprintf("PRAGMA table_info(%q)", tableName)

	rows, err := d.Query(query)
	if err != nil {
		return "", fmt.Errorf("failed to get table details for %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []ColumnDetails
	for rows.Next() {
		var col ColumnDetails
		var notNull int
		// schema of PRAGMA table_info: cid, name, type, notnull, dflt_value, pk
		if err := rows.Scan(&col.ID, &col.Name, &col.Type, &notNull, &col.DefaultValue, &col.PK); err != nil {
			return "", fmt.Errorf("failed to scan column info: %w", err)
		}
		col.NotNull = notNull != 0
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating columns: %w", err)
	}

	var result string
	for _, col := range columns {
		result += fmt.Sprintf("Column ID: %d, Name: %s, Type: %s, NotNull: %t, DefaultValue: %v, PK Index: %d\n", col.ID, col.Name, col.Type, col.NotNull, col.DefaultValue, col.PK)
	}

	return result, nil
}

// GetSchema checks if a table with the given name exists.
// If it exists, it builds and returns a schema object based on the table's structure.
// Returns an error if the table does not exist or if there is an issue querying the database.
func (d *Database) GetSchema(tableName string) (schema, error) {
	// Step 1: Check if table exists
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	row := d.QueryRow(query, tableName)
	var count int
	if err := row.Scan(&count); err != nil {
		return schema{}, fmt.Errorf("failed to check table existence: %w", err)
	}
	if count == 0 {
		return schema{}, fmt.Errorf("table %s does not exist", tableName)
	}

	// Step 2: Get table structure using PRAGMA
	type ColDetails struct {
		ID           int
		Name         string
		Type         string
		NotNull      int
		DefaultValue *string
		PK           int
	}
	pragmaQuery := fmt.Sprintf("PRAGMA table_info(%q)", tableName)
	rows, err := d.Query(pragmaQuery)
	if err != nil {
		return schema{}, fmt.Errorf("[GetSchema]failed to query table structure: %w", err)
	}
	defer rows.Close()

	var columns []column
	for rows.Next() {
		var col ColDetails
		if err := rows.Scan(&col.ID, &col.Name, &col.Type, &col.NotNull, &col.DefaultValue, &col.PK); err != nil {
			return schema{}, fmt.Errorf("failed to scan column: %w", err)
		}
		// Build column
		c := NewColumn(col.Name, mapSQLType(col.Type))
		if col.NotNull != 0 {
			c = c.WithConstraint(NOT_NULL)
		}
		if col.DefaultValue != nil {
			c = c.WithConstraintVal(WithDefault(*col.DefaultValue))
		}
		if col.PK > 0 {
			c = c.PrimaryKey()
		}
		columns = append(columns, c)
	}
	if err := rows.Err(); err != nil {
		return schema{}, fmt.Errorf("error iterating table columns: %w", err)
	}

	// Step 3: Build and return schema
	return schema{
		tableName: tableName,
		columns:   columns,
		handler:   nil, // Not applicable for dynamically built schemas
	}, nil
}
