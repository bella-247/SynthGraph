package exporter

import (
	"fmt"
	"io"
	"strings"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// writeSQLTable writes a single table as an INSERT statement with all its rows.
func writeSQLTable(writer io.Writer, table *generator.GeneratedTable, model *schema.Model, config *ExportConfig) error {
	schemaTable := model.TableMap[table.TableName]
	if schemaTable == nil {
		return fmt.Errorf("table %q not found in schema model", table.TableName)
	}
	if len(table.Rows) == 0 {
		return nil
	}

	columns := columnNames(schemaTable)
	colTypes := columnTypeMap(schemaTable)

	tableName := table.TableName
	if config.SchemaName != "" {
		tableName = config.SchemaName + "." + tableName
	}

	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = quoteIdentifier(col)
	}

	valueGroups := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		values := make([]string, len(columns))
		for i, col := range columns {
			values[i] = formatSQLValue(row[col], colTypes[col])
		}
		valueGroups = append(valueGroups, "("+strings.Join(values, ", ")+")")
	}

	stmt := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES\n%s;",
		tableName,
		strings.Join(quotedCols, ", "),
		strings.Join(valueGroups, ",\n"),
	)

	_, err := io.WriteString(writer, stmt+"\n")
	return err
}

// needsQuoting returns true when the SQL column type expects string-quoted values.
// This prevents int64/float64 PK values from being written unquoted when the FK
// column is a string type (varchar, text, etc.).
func needsQuoting(columnType string) bool {
	switch columnType {
	case "int", "int4", "int8", "bigint", "smallint",
		"serial", "bigserial", "smallserial",
		"decimal", "numeric", "float4", "float8", "real", "double", "double precision",
		"boolean":
		return false
	}
	return true
}

// formatSQLValue formats a single value for SQL INSERT.
// The columnType parameter ensures int64/float64 values are quoted when the
// target column is a string type (e.g. a varchar FK referencing an int PK).
func formatSQLValue(value any, columnType string) string {
	if value == nil {
		return "NULL"
	}

	switch v := value.(type) {
	case int64:
		if needsQuoting(columnType) {
			return "'" + escapeSQLString(fmt.Sprintf("%d", v)) + "'"
		}
		return fmt.Sprintf("%d", v)
	case float64:
		if needsQuoting(columnType) {
			return "'" + escapeSQLString(fmt.Sprintf("%.2f", v)) + "'"
		}
		return fmt.Sprintf("%.2f", v)
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case string:
		return "'" + escapeSQLString(v) + "'"
	default:
		return fmt.Sprintf("'%v'", escapeSQLString(fmt.Sprintf("%v", v)))
	}
}

// escapeSQLString escapes single quotes and backslashes in a SQL string.
func escapeSQLString(value string) string {
	value = strings.ReplaceAll(value, "'", "''")
	return value
}

// columnNames returns the column names for a table from its schema definition.
func columnNames(table *schema.Table) []string {
	names := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		names[i] = col.Name
	}
	return names
}

// columnTypeMap returns a map of column name → SQL type for efficient lookups
// during value formatting.
func columnTypeMap(table *schema.Table) map[string]string {
	types := make(map[string]string, len(table.Columns))
	for _, col := range table.Columns {
		types[col.Name] = col.Type
	}
	return types
}
