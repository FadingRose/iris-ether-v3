package db

import (
	"fmt"
	"strings"
)

// column represents a single column in a database table.
type column struct {
	identifier     string
	sqlType        SQLType
	constraints    []ConstraintType    // For simple, value-less constraints
	valConstraints []ConstraintWithVal // For constraints that include a value
	isPrimaryKey   bool
}

type SQLType = string

const (
	INT       SQLType = "INT"
	INTEGER   SQLType = "INTEGER"
	TEXT      SQLType = "TEXT"
	VARCHAR   SQLType = "VARCHAR(255)"
	BOOLEAN   SQLType = "BOOLEAN"
	REAL      SQLType = "REAL"
	BLOB      SQLType = "BLOB"
	DATETIME  SQLType = "DATETIME"
	INT_ARRAY SQLType = "TEXT"
	JSON      SQLType = "TEXT"
)

// ConstraintType represents a valid SQL column constraint without a specific value.
type ConstraintType string

const (
	NOT_NULL       ConstraintType = "NOT NULL"
	UNIQUE         ConstraintType = "UNIQUE"
	AUTO_INCREMENT ConstraintType = "AUTO_INCREMENT"
)

// NewColumn creates a new column with the given name and sql type.
func NewColumn(identifier string, sqlType SQLType) column {
	return column{
		identifier: identifier,
		sqlType:    sqlType,
	}
}

// ConstraintWithVal represents a SQL column constraint that includes a value.
type ConstraintWithVal struct {
	Type  ConstraintType // e.g., DEFAULT, CHECK
	Value string         // The value associated with the constraint
}

// Named constructors for ConstraintWithVal to ensure valid types
func WithDefault(value string) ConstraintWithVal {
	return ConstraintWithVal{Type: "DEFAULT", Value: value}
}

func WithCheck(condition string) ConstraintWithVal {
	return ConstraintWithVal{Type: "CHECK", Value: condition}
}

// WithConstraint adds a single simple constraint (e.g., NOT NULL, UNIQUE).
func (c column) WithConstraint(constraint ConstraintType) column {
	if strings.ToUpper(string(constraint)) == "PRIMARY KEY" {
		return c // PRIMARY KEY is handled by PrimaryKey()
	}
	c.constraints = append(c.constraints, constraint)
	return c
}

// WithConstraints adds multiple simple constraints.
func (c column) WithConstraints(constraints ...ConstraintType) column {
	for _, constraint := range constraints {
		c = c.WithConstraint(constraint)
	}
	return c
}

// WithConstraintVal adds a constraint that includes a value (e.g., DEFAULT 'value').
func (c column) WithConstraintVal(valConstraint ConstraintWithVal) column {
	// Add validation if needed, e.g., check if valConstraint.Type is "DEFAULT" or "CHECK"
	c.valConstraints = append(c.valConstraints, valConstraint)
	return c
}

// WithConstraintVals adds multiple value-based constraints.
func (c column) WithConstraintVals(valConstraints ...ConstraintWithVal) column {
	for _, vc := range valConstraints {
		c = c.WithConstraintVal(vc)
	}
	return c
}

// PrimaryKey sets the column as a primary key.
func (c column) PrimaryKey() column {
	c.isPrimaryKey = true
	return c
}

type RecordHandler[T any] interface {
	Fetch(...any) ([]T, []map[string]any, error)
}

// schema represents the schema of a database table.
type schema struct {
	tableName string
	columns   []column
	handler   any // RecordHandler instance
}

// NewSchema creates a new schema with a generic RecordHandler.
func NewSchema[T any](tableName string, handler RecordHandler[T]) *SchemaBuilder {
	return &SchemaBuilder{
		schema: schema{
			tableName: tableName,
			columns:   []column{},
			handler:   handler,
		},
		err: fmt.Errorf("need at least one primary key column"),
	}
}

// SchemaBuilder helps in constructing a schema using a fluent API.
type SchemaBuilder struct {
	schema schema
	err    error
}

// AddColumn adds a column definition to the schema.
// if there are multiple primary keys
// consider them as comoposite primary keys, see query() method for more details.
func (sb *SchemaBuilder) AddColumn(col column) *SchemaBuilder {
	if col.isPrimaryKey {
		sb.err = nil // Clear error if at least one primary key is added
	}
	sb.schema.columns = append(sb.schema.columns, col)
	return sb
}

// Build finalizes the schema creation.
func (sb *SchemaBuilder) Build() (schema, error) {
	if sb.err != nil {
		// if no primary key was added, return error
		return schema{}, sb.err
	}
	return sb.schema, nil
}

// create generates the SQL CREATE TABLE statement for the schema.
func (s schema) create() (string, error) {
	if s.tableName == "" {
		return "", fmt.Errorf("table name cannot be empty")
	}
	if len(s.columns) == 0 {
		return "", fmt.Errorf("schema must have at least one column")
	}

	var columnDefs []string
	var primaryKeys []string
	for _, col := range s.columns {
		if col.identifier == "" || col.sqlType == "" {
			return "", fmt.Errorf("column identifier and type cannot be empty")
		}

		var currentConstraints []string
		// Add simple constraints
		for _, cons := range col.constraints {
			currentConstraints = append(currentConstraints, string(cons))
		}
		// Add value-based constraints
		for _, vCons := range col.valConstraints {
			// Special handling for DEFAULT or CHECK to format correctly
			switch vCons.Type {
			case "DEFAULT":
				currentConstraints = append(currentConstraints, fmt.Sprintf("%s %s", vCons.Type, vCons.Value))
			case "CHECK":
				currentConstraints = append(currentConstraints, fmt.Sprintf("%s (%s)", vCons.Type, vCons.Value))
			default:
				currentConstraints = append(currentConstraints, fmt.Sprintf("%s %s", vCons.Type, vCons.Value))
			}
		}

		def := fmt.Sprintf("%s %s", col.identifier, col.sqlType)
		if len(currentConstraints) > 0 {
			def = fmt.Sprintf("%s %s", def, strings.Join(currentConstraints, " "))
		}

		if col.isPrimaryKey {
			primaryKeys = append(primaryKeys, col.identifier)
		}
		columnDefs = append(columnDefs, def)
	}

	if len(primaryKeys) > 0 {
		columnDefs = append(columnDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", s.tableName, strings.Join(columnDefs, ", "))
	return query, nil
}

// insert generates the SQL INSERT statement and arg values based on the schema and input map.
func (s schema) insert(values map[string]any, override bool) (string, []any, error) {
	if s.tableName == "" {
		return "", nil, fmt.Errorf("table name cannot be empty")
	}

	verb := "INSERT IGNORE"
	if override {
		verb = "REPLACE"
	}

	var cols []string
	var args []any
	var placeholders []string

	// Iterate over schema columns to maintain defined order and validate column existence
	for _, col := range s.columns {
		if val, ok := values[col.identifier]; ok {
			cols = append(cols, col.identifier)
			args = append(args, val)
			placeholders = append(placeholders, "?")
		}
	}

	if len(cols) == 0 {
		return "", nil, fmt.Errorf("no matching columns found in values map")
	}

	query := fmt.Sprintf("%s INTO %s (%s) VALUES (%s);",
		verb,
		s.tableName,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "))

	return query, args, nil
}

func (s schema) fetch(args ...any) ([]map[string]any, error) {
	if rh, ok := s.handler.(RecordHandler[FileRecord]); ok {
		_, raws, err := rh.Fetch(args...)
		if err != nil {
			return nil, err
		}
		return raws, nil
	}
	if rh, ok := s.handler.(RecordHandler[StatInstrumentRecord]); ok {
		_, raws, err := rh.Fetch(args...)
		if err != nil {
			return nil, err
		}
		return raws, nil
	}
	if rh, ok := s.handler.(RecordHandler[StatRecord]); ok {
		_, raws, err := rh.Fetch(args...)
		if err != nil {
			return nil, err
		}
		return raws, nil
	}
	if rh, ok := s.handler.(RecordHandler[DatasetRecord]); ok {
		_, raws, err := rh.Fetch(args...)
		if err != nil {
			return nil, err
		}
		return raws, nil
	}
	return nil, fmt.Errorf("unsupported record handler type in fetch")
}

// Helper to map SQLite types to SQLType constants
func mapSQLType(sqliteType string) SQLType {
	switch strings.ToUpper(sqliteType) {
	case "INT", "INTEGER":
		return INTEGER
	case "TEXT":
		return TEXT
	case "VARCHAR", "VARCHAR(255)":
		return VARCHAR
	case "BOOLEAN":
		return BOOLEAN
	case "REAL":
		return REAL
	case "BLOB":
		return BLOB
	case "DATETIME":
		return DATETIME
	default:
		// Fallback to TEXT for unknown types
		return TEXT
	}
}
