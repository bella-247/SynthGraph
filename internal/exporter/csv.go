package exporter

import (
	"fmt"
	"io"
	"strings"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// writeCSVTable writes a single table in CSV format.
// Column order follows the schema definition for determinism.
// If IncludeHeader is true, the first row contains column names.
func writeCSVTable(writer io.Writer, table *generator.GeneratedTable, model *schema.Model, config *ExportConfig) error {
	columns := columnNames(model, table.TableName)
	if len(columns) == 0 {
		return fmt.Errorf("table %q not found in schema model", table.TableName)
	}
	if len(table.Rows) == 0 {
		return nil
	}

	// Write header if requested.
	if config.IncludeHeader {
		header := make([]string, len(columns))
		for i, col := range columns {
			header[i] = escapeCSVField(col)
		}
		if _, err := io.WriteString(writer, strings.Join(header, ",")+"\n"); err != nil {
			return err
		}
	}

	// Write data rows.
	for _, row := range table.Rows {
		fields := make([]string, len(columns))
		for i, col := range columns {
			fields[i] = formatCSVValue(row[col])
		}
		if _, err := io.WriteString(writer, strings.Join(fields, ",")+"\n"); err != nil {
			return err
		}
	}

	return nil
}

// formatCSVValue formats a single value for CSV output.
//
//	nil       → empty string
//	string    → double-quoted, with internal quotes escaped
//	bool      → "true" / "false"
//	numeric   → formatted number (no quotes)
func formatCSVValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return escapeCSVField(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	default:
		return escapeCSVField(fmt.Sprintf("%v", v))
	}
}

// escapeCSVField double-quotes a string and escapes any internal double quotes.
func escapeCSVField(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}
