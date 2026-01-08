package db

import (
	"fmt"
	"strings"
)

// QueryBuilder provides a fluent API for building SQL SELECT queries based on the schema.
// It follows a similar pattern to SchemaBuilder for consistency.
type QueryBuilder struct {
	schema schema
	where  string
	args   []any
	order  string
	limit  *int
	offset *int
}

// NewQuery starts building a SELECT query from the schema.
func (s schema) NewQuery() *QueryBuilder {
	return &QueryBuilder{schema: s}
}

func (qb *QueryBuilder) Clean() *QueryBuilder {
	qb.where = ""
	qb.args = nil
	qb.order = ""
	qb.limit = nil
	qb.offset = nil
	return qb
}

// Where sets the WHERE clause for the query.
// Example: .Where("id = ? AND name = ?")
func (qb *QueryBuilder) Where(condition string) *QueryBuilder {
	qb.where = condition
	return qb
}

// WithArgs sets the arguments for the WHERE clause placeholders.
func (qb *QueryBuilder) WithArgs(args ...any) *QueryBuilder {
	qb.args = args
	return qb
}

// OrderBy sets the ORDER BY clause for the query.
// Example: .OrderBy("id ASC, name DESC")
func (qb *QueryBuilder) OrderBy(clause string) *QueryBuilder {
	qb.order = clause
	return qb
}

// Limit sets the LIMIT for the query.
func (qb *QueryBuilder) Limit(n int) *QueryBuilder {
	qb.limit = &n
	return qb
}

// Offset sets the OFFSET for the query.
func (qb *QueryBuilder) Offset(n int) *QueryBuilder {
	qb.offset = &n
	return qb
}

// Build generates the SQL query string, bound arguments, and returns them.
// Returns an error if the schema is invalid.
func (qb *QueryBuilder) Build() (string, []any, error) {
	params := QueryParams{
		Where:  qb.where,
		Args:   qb.args,
		Order:  qb.order,
		Limit:  qb.limit,
		Offset: qb.offset,
	}
	return qb.schema.query(params)
}

// QueryParams holds parameters for building a SQL query.
type QueryParams struct {
	Where  string // Optional WHERE clause (e.g., "id = ? AND name = ?")
	Args   []any  // Arguments for WHERE clause placeholders
	Order  string // Optional ORDER BY clause (e.g., "id ASC, name DESC")
	Limit  *int   // Optional LIMIT value
	Offset *int   // Optional OFFSET value
}

// query builds a SELECT SQL query string and bound arguments based on the schema and query parameters.
// It generates "SELECT column1, column2, ... FROM table WHERE ... ORDER BY ... LIMIT ... OFFSET ...;"
// If no Where is provided, it selects all rows. Fails if the schema is invalid.
func (s schema) query(params QueryParams) (string, []any, error) {
	if s.tableName == "" {
		return "", nil, fmt.Errorf("table name cannot be empty")
	}
	if len(s.columns) == 0 {
		return "", nil, fmt.Errorf("schema must have at least one column")
	}

	// Build column list from schema columns
	var cols []string
	for _, col := range s.columns {
		cols = append(cols, col.identifier)
	}

	// Start with base SELECT
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), s.tableName)

	// Append WHERE if provided
	if params.Where != "" {
		query += " WHERE " + params.Where
	}

	// Append ORDER BY if provided
	if params.Order != "" {
		query += " ORDER BY " + params.Order
	}

	// Append LIMIT if provided
	if params.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *params.Limit)
	}

	// Append OFFSET if provided (only if LIMIT is set)
	if params.Offset != nil && params.Limit != nil {
		query += fmt.Sprintf(" OFFSET %d", *params.Offset)
	}

	query += ";"

	return query, params.Args, nil
}
