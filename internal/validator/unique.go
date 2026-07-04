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
			key := compositeKey(row, uniqueColumns)

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
