package validator

import (
	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// checkNotNull validates that every column marked non-nullable in the schema
// has a non-nil value in every generated row.
func checkNotNull(table *generator.GeneratedTable, tableDef *schema.Table, result *ValidationResult) {
	for _, columnDef := range tableDef.Columns {
		if columnDef.Nullable {
			continue
		}
		for rowIndex, row := range table.Rows {
			value, exists := row[columnDef.Name]
			if !exists || value == nil {
				result.addError(
					table.TableName, rowIndex, columnDef.Name,
					"NOT NULL",
					"column has nil value but is declared NOT NULL",
				)
			}
		}
	}
}
