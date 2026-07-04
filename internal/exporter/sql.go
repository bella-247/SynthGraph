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
	columns := columnNames(model, table.TableName)
	if len(columns) == 0 {
		return fmt.Errorf("table %q not found in schema model", table.TableName)
	}
	if len(table.Rows) == 0 {
		return nil
	}

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
			values[i] = formatSQLValue(row[col])
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

// formatSQLValue formats a single value for SQL INSERT.
func formatSQLValue(value any) string {
	if value == nil {
		return "NULL"
	}

	switch v := value.(type) {
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
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
func columnNames(model *schema.Model, tableName string) []string {
	table := model.TableMap[tableName]
	if table == nil {
		return nil
	}
	names := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		names[i] = col.Name
	}
	return names
}
