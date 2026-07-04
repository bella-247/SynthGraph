package validator

import (
	"fmt"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// checkForeignKeys validates that every foreign key column value references
// an existing primary key value in the referenced table.
func checkForeignKeys(
	table *generator.GeneratedTable,
	tableDef *schema.Table,
	pkIndex map[string]map[string]bool,
	result *ValidationResult,
) {
	for _, fk := range tableDef.ForeignKeys {
		if len(fk.Columns) == 0 {
			continue
		}

		refPKs, referencedExists := pkIndex[fk.RefTable]
		if !referencedExists {
			result.addError(
				table.TableName, -1, "",
				"FOREIGN KEY",
				fmt.Sprintf("referenced table %q has no primary key or no generated rows", fk.RefTable),
			)
			continue
		}

		for rowIndex, row := range table.Rows {
			// Build the composite FK value key.
			var fkValue string
			if len(fk.Columns) == 1 {
				fkValue = fmt.Sprintf("%v", row[fk.Columns[0]])
			} else {
				for i, col := range fk.Columns {
					if i > 0 {
						fkValue += "::"
					}
					fkValue += fmt.Sprintf("%v", row[col])
				}
			}

			// Nil FK values are valid (even for NOT NULL columns that were
			// deferred for cycle resolution and backfilled). Check if the
			// value still ends up nil.
			if row[fk.Columns[0]] == nil {
				result.addError(
					table.TableName, rowIndex, fk.Columns[0],
					"FOREIGN KEY",
					"foreign key column is nil after generation",
				)
				continue
			}

			if !refPKs[fkValue] {
				colDesc := fmt.Sprintf("%v", fk.Columns)
				result.addError(
					table.TableName, rowIndex, fk.Columns[0],
					"FOREIGN KEY",
					fmt.Sprintf("value %q in columns %s references non-existent key in table %q",
						fkValue, colDesc, fk.RefTable),
				)
			}
		}
	}
}
