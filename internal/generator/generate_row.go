package generator

import (
	"fmt"

	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
)

// generateTable generates all rows for a single table from the plan.
func generateTable(
	tablePlan planner.TablePlan,
	ctx *GenerationContext,
	fkMap map[string][]FKRefInfo,
	enumValues map[string][]string,
	tablePKs map[string][]any,
) (*GeneratedTable, error) {
	tableName := tablePlan.TableName
	table := tablePlan.Table
	rowCount := tablePlan.RowCount
	deferredCols := graph.StringSet(tablePlan.DeferredCols)

	rng := newTableRNG(ctx.GlobalSeed, tableName)

	generated := &GeneratedTable{
		TableName: tableName,
		Rows:      make([]GeneratedRow, 0, rowCount),
	}

	// Build column-level FK ref info for this table.
	tableFKCols := fkMap[tableName]

	// Track generated values for UNIQUE constraint enforcement.
	tracker := newUniqueTracker(table)

	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		row := make(GeneratedRow, len(table.Columns))

		for _, column := range table.Columns {
			// Deferred FK columns: insert as NULL.
			if deferredCols[column.Name] {
				row[column.Name] = nil
				continue
			}

			// FK columns: pick a PK value from the referenced table.
			if fkInfo, isFK := isFKColumn(tableFKCols, column.Name); isFK {
				pkValues, referencedAvailable := tablePKs[fkInfo.RefTable]
				if !referencedAvailable || len(pkValues) == 0 {
					return nil, &GenError{
						Table:   tableName,
						Row:     rowIndex,
						Column:  column.Name,
						Message: fmt.Sprintf("referenced table %q has no generated PK values", fkInfo.RefTable),
					}
				}
				value := pkValues[rng.IntN(len(pkValues))]
				row[column.Name] = value
				continue
			}

			// Regular column: generate from type generator.
			generator := typeGeneratorFor(column.Type, ctx.Model, enumValues)
			value, err := generator.Generate(&column, rowIndex, rng)
			if err != nil {
				return nil, &GenError{
					Table:   tableName,
					Row:     rowIndex,
					Column:  column.Name,
					Message: err.Error(),
				}
			}

			// Retry on UNIQUE violation.
			if tracker.isUniqueColumn(column.Name) {
				const maxUniqueRetries = 100
				for attempts := 0; attempts < maxUniqueRetries; attempts++ {
					if !tracker.checkSeen(column.Name, value) {
						break
					}
					value, err = generator.Generate(&column, rowIndex+attempts+1, rng)
					if err != nil {
						return nil, &GenError{
							Table:   tableName,
							Row:     rowIndex,
							Column:  column.Name,
							Message: err.Error(),
						}
					}
				}
				tracker.record(column.Name, value)
			}

			// NOT NULL safeguard: if the value is nil and the column is NOT NULL,
			// generate a placeholder to avoid constraint violations.
			if value == nil && !column.Nullable {
				colLower := column.Name
				switch {
				case colLower == "id":
					value = int64(0)
				default:
					value = "PLACEHOLDER"
				}
			}

			row[column.Name] = value
		}

		generated.Rows = append(generated.Rows, row)
	}

	return generated, nil
}
