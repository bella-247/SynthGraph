package validator

import (
	"fmt"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// checkUnique validates that every column set declared as UNIQUE in the schema
// has no duplicate value combinations across all generated rows.
func checkUnique(table *generator.GeneratedTable, tableDef *schema.Table, result *ValidationResult) {
	for _, uniqueColumns := range tableDef.Unique {
		if len(uniqueColumns) == 0 {
			continue
		}
		seen := make(map[string]int)

		for rowIndex, row := range table.Rows {
			// Build the key for this unique constraint's column set.
			var key string
			if len(uniqueColumns) == 1 {
				key = fmt.Sprintf("%v", row[uniqueColumns[0]])
			} else {
				for i, col := range uniqueColumns {
					if i > 0 {
						key += "::"
					}
					key += fmt.Sprintf("%v", row[col])
				}
			}

			if firstRow, seenBefore := seen[key]; seenBefore {
				result.addError(
					table.TableName, rowIndex, "",
					"UNIQUE",
					fmt.Sprintf("duplicate value %q for unique constraint on columns %v (first seen at row %d)",
						key, uniqueColumns, firstRow),
				)
			} else {
				seen[key] = rowIndex
			}
		}
	}
}
