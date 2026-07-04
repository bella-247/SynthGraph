package validator

import (
	"fmt"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// checkPrimaryKey validates that:
//   - Every PK column is non-nil in every row.
//   - Every PK value combination is unique (no duplicates).
func checkPrimaryKey(table *generator.GeneratedTable, tableDef *schema.Table, result *ValidationResult) {
	if len(tableDef.PrimaryKey) == 0 {
		return
	}

	seen := make(map[string]int) // PK string → first row index where seen

	for rowIndex, row := range table.Rows {
		// Check for nil values in PK columns.
		for _, pkCol := range tableDef.PrimaryKey {
			value, exists := row[pkCol]
			if !exists || value == nil {
				result.addError(
					table.TableName, rowIndex, pkCol,
					"PRIMARY KEY",
					"primary key column has nil value",
				)
			}
		}

		// Check for duplicate PK combination.
		key := pkString(row, tableDef.PrimaryKey)
		if firstRow, seenBefore := seen[key]; seenBefore {
			result.addError(
				table.TableName, rowIndex, "",
				"PRIMARY KEY",
				fmt.Sprintf("duplicate primary key %q (first seen at row %d)", key, firstRow),
			)
		} else {
			seen[key] = rowIndex
		}
	}
}
