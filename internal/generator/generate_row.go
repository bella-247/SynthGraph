package generator

import (
	"context"
	"fmt"
	"math/rand/v2"

	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/semantic"
)

const maxUniqueRetries = 100

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

	// Look up the table's semantic node for temporal pattern and role info.
	semNode := ctx.SemanticGraph.Nodes["table:"+tableName]

	generated := &GeneratedTable{
		TableName: tableName,
		Rows:      make([]GeneratedRow, 0, rowCount),
	}

	// Build column-level FK ref info for this table.
	tableFKCols := fkMap[tableName]

	// Track generated values for UNIQUE constraint enforcement.
	tracker := newUniqueTracker(table)

	// Resolve the semantic generator registry: use context's if set,
	// otherwise fall back to the package-level default.
	registry := defaultRegistry
	if ctx.Registry != nil {
		registry = ctx.Registry
	}

	// Build the set of aggregation FK columns (nullable, optional relationships).
	// These columns are left NULL with ~20% probability to reflect real-world
	// optionality instead of always picking a FK value.
	aggFKs := buildAggregationFKSet(ctx.SemanticGraph, tableName)

	cancelCtx := ctx.Context
	if cancelCtx == nil {
		cancelCtx = context.Background()
	}

	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		select {
		case <-cancelCtx.Done():
			return generated, nil
		default:
		}

		row := make(GeneratedRow, len(table.Columns))

		rowValues := precomputeRowValues(table, rng)
		if semNode != nil {
			temporalValues := precomputeTemporalValues(table, semNode.Temporal, rowIndex, rng)
			if len(temporalValues) > 0 {
				if rowValues == nil {
					rowValues = make(map[string]any, len(temporalValues))
				}
				for colName, val := range temporalValues {
					rowValues[colName] = val
				}
			}
		}

		for _, column := range table.Columns {
			if val, ok := rowValues[column.Name]; ok {
				if tracker.isUniqueColumn(column.Name) && tracker.checkSeen(column.Name, val) {
				} else {
					if tracker.isUniqueColumn(column.Name) {
						tracker.record(column.Name, val)
					}
					row[column.Name] = val
					continue
				}
			}
			if deferredCols[column.Name] {
				row[column.Name] = nil
				continue
			}

			if fkInfo, isFK := isFKColumn(tableFKCols, column.Name); isFK {
				if aggFKs[column.Name] && rng.Float64() < aggFKNullProbability {
					row[column.Name] = nil
					continue
				}
				pkValues, referencedAvailable := tablePKs[fkInfo.RefTable]
				if !referencedAvailable || len(pkValues) == 0 {
					return nil, &GenError{
						Table:   tableName,
						Row:     rowIndex,
						Column:  column.Name,
						Message: fmt.Sprintf("referenced table %q has no generated PK values", fkInfo.RefTable),
					}
				}
				value := pickUniqueFKValue(pkValues, row, column.Name, fkInfo, rng, tracker)
				row[column.Name] = value
				continue
			}

			var columnGen TypeGenerator
			if _, isEnum := enumValues[column.Type]; isEnum {
				columnGen = typeGeneratorFor(column.Type, ctx.Model, enumValues)
			} else if semGen, hasGen := registry.GeneratorFor(column.Semantic); hasGen {
				columnGen = semGen
			} else {
				columnGen = typeGeneratorFor(column.Type, ctx.Model, enumValues)
			}

			value, err := columnGen.Generate(&column, rowIndex, rng)
			if err != nil {
				return nil, &GenError{
					Table:   tableName,
					Row:     rowIndex,
					Column:  column.Name,
					Message: err.Error(),
				}
			}

			if strVal, ok := value.(string); ok && column.Length > 0 && len(strVal) > column.Length {
				if column.Type != "decimal" && column.Type != "numeric" {
					value = strVal[:column.Length]
				}
			}

			if tracker.isUniqueColumn(column.Name) {
				exhausted := true
				for attempts := 0; attempts < maxUniqueRetries; attempts++ {
					if !tracker.checkSeen(column.Name, value) {
						exhausted = false
						break
					}
					value, err = columnGen.Generate(&column, rowIndex+attempts+1, rng)
					if err != nil {
						return nil, &GenError{
							Table:   tableName,
							Row:     rowIndex,
							Column:  column.Name,
							Message: err.Error(),
						}
					}
				}
				if exhausted {
					return nil, &GenError{
						Table:   tableName,
						Row:     rowIndex,
						Column:  column.Name,
						Message: fmt.Sprintf("could not generate unique value after %d attempts — value pool exhausted", maxUniqueRetries),
					}
				}
				tracker.record(column.Name, value)
			}

			if value == nil && !column.Nullable {
				return nil, &GenError{
					Table:   tableName,
					Row:     rowIndex,
					Column:  column.Name,
					Message: fmt.Sprintf("not-null column %q produced nil value — generator cannot satisfy NOT NULL constraint", column.Name),
				}
			}

			row[column.Name] = value
		}

		tracker.recordRowConstraints(row, rowIndex)
		generated.Rows = append(generated.Rows, row)
	}

	return generated, nil
}

// buildAggregationFKSet returns the set of FK column names whose semantic
// relationship kind is aggregation (nullable, optional). These columns are
// left NULL with a configurable probability in the generation loop.
func buildAggregationFKSet(semGraph *semantic.SemanticGraph, tableName string) map[string]bool {
	if semGraph == nil {
		return nil
	}
	tableNodeID := "table:" + tableName
	aggFKs := make(map[string]bool)
	for _, rel := range semGraph.Relationships {
		if rel.From == tableNodeID && rel.Kind == semantic.RelationshipKindAggregation {
			if fkMeta, ok := rel.Metadata.(*graph.FKMetadata); ok {
				for _, col := range fkMeta.LocalColumns {
					aggFKs[col] = true
				}
			}
		}
	}
	if len(aggFKs) == 0 {
		return nil
	}
	return aggFKs
}

// aggFKNullProbability is the fraction of rows where an aggregation FK column
// is left NULL instead of populated with a referenced PK value, reflecting
// real-world optionality in nullable foreign key relationships.
const aggFKNullProbability = 0.2

// pickUniqueFKValue picks a FK value from the available pool, preferring values
// that do not violate composite uniqueness constraints (e.g. UNIQUE(user_id, product_id)
// or a composite primary key). Linear probing is used instead of extra RNG calls
// to preserve determinism — the same seed always produces the same output regardless
// of how many constraints are present.
func pickUniqueFKValue(pkValues []any, row GeneratedRow, colName string, fkInfo FKRefInfo, rng *rand.Rand, tracker *uniqueTracker) any {
	group, idx, isComposite := tracker.compositeGroupForColumn(colName)
	if !isComposite || idx == 0 {
		return pkValues[rng.IntN(len(pkValues))]
	}

	startIdx := rng.IntN(len(pkValues))
	for offset := 0; offset < len(pkValues); offset++ {
		candidate := pkValues[(startIdx+offset)%len(pkValues)]
		row[colName] = candidate
		key := serializeRowKey(row, group)
		if !tracker.isCompositeKeySeen(key) {
			return candidate
		}
	}
	return pkValues[startIdx]
}

