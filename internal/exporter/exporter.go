// Package exporter serialises a generated Dataset into SQL INSERT statements
// and CSV files — the final output stage of the SynthGraph pipeline.
//
// # Architecture position
//
//	Parser → schema.Model → Graph → planner.BuildPlan → GenerationPlan
//	                                                          │
//	                                                          ▼
//	                                                    generator.Generate
//	                                                          │
//	                                                          ▼
//	                                                    Dataset
//	                                                          │
//	                                                          ▼
//	                                                    validator.Validate
//	                                                          │
//	                                                          ▼
//	                                                    exporter.Export
//	                                                          │
//	                                      ┌───────────────────┴──────────────────┐
//	                                      ▼                                      ▼
//	                                   SQL INSERT                              CSV
package exporter

import (
	"fmt"
	"io"
	"strings"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// ExportConfig controls the output format and behaviour.
type ExportConfig struct {
	// SchemaName is an optional schema prefix for SQL (e.g. "public").
	// When set, table names are rendered as "schema.table".
	SchemaName string

	// IncludeHeader for CSV writes a column-name header row.
	IncludeHeader bool
}

// ExportSQL writes every table in the dataset as a batch of INSERT statements.
// Each table's rows are written as a single INSERT with multiple value tuples.
//
//	String values are single-quote escaped.
//	NULL values render as the SQL keyword NULL.
//	Boolean values render as TRUE / FALSE.
func ExportSQL(writer io.Writer, dataset *generator.Dataset, model *schema.Model, config *ExportConfig) error {
	for _, table := range dataset.Tables {
		if err := writeSQLTable(writer, table, model, config); err != nil {
			return fmt.Errorf("export sql: %w", err)
		}
	}
	return nil
}

// ExportCSV writes every table in the dataset in CSV format, separated by
// blank lines. Each table starts with a comment line: "-- table_name".
//
//	String values are CSV-quoted (double quotes escaped).
//	NULL values render as empty fields.
//	Boolean values render as true / false.
func ExportCSV(writer io.Writer, dataset *generator.Dataset, model *schema.Model, config *ExportConfig) error {
	for i, table := range dataset.Tables {
		if i > 0 {
			if _, err := io.WriteString(writer, "\n"); err != nil {
				return fmt.Errorf("export csv: %w", err)
			}
		}
		if err := writeCSVTable(writer, table, model, config); err != nil {
			return fmt.Errorf("export csv: %w", err)
		}
	}
	return nil
}

// quoteIdentifier double-quotes a SQL identifier, escaping embedded quotes.
func quoteIdentifier(name string) string {
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}
