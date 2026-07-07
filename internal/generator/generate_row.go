package generator

import (
	"fmt"

	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/semantic"
)

const maxUniqueRetries = 100

const placeholderValue = "PLACEHOLDER"

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

	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		row := make(GeneratedRow, len(table.Columns))

		// Phase 0: Pre-compute cross-column correlated values for this row
		// (e.g. city/state/zip consistency, first/last/full_name consistency,
		// temporal coherence: created_at < updated_at, deleted_at distribution).
		rowValues := precomputeRowValues(table, rng)
		if semNode != nil {
			for colName, val := range precomputeTemporalValues(table, semNode.Temporal, rowIndex, rng) {
				rowValues[colName] = val
			}
		}

		for _, column := range table.Columns {
			// Use pre-computed correlated values if available.
			if val, ok := rowValues[column.Name]; ok {
				row[column.Name] = val
				continue
			}
			// Deferred FK columns: insert as NULL.
			if deferredCols[column.Name] {
				row[column.Name] = nil
				continue
			}

			// FK columns: pick a PK value from the referenced table.
			if fkInfo, isFK := isFKColumn(tableFKCols, column.Name); isFK {
				// Aggregation (optional) FK: leave NULL ~20% of the time.
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
				value := pkValues[rng.IntN(len(pkValues))]
				row[column.Name] = value
				continue
			}

			// Semantic-aware generation: use column.Semantic (pre-populated during
			// semantic analysis) to pick a domain-specific generator (name, email,
			// phone, etc.) before falling back to the type-based generator.
			if semGen, hasGen := registry.GeneratorFor(column.Semantic); hasGen {
				value, err := semGen.Generate(&column, rowIndex, rng)
				if err != nil {
					return nil, &GenError{
						Table:   tableName,
						Row:     rowIndex,
						Column:  column.Name,
						Message: err.Error(),
					}
				}
				row[column.Name] = value
				continue
			}

			// Type-based fallback: generate from the column's database type.
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

			// Length constraint: truncate string values that exceed column length.
			// Decimal types are excluded — Length represents total digit count, not string length.
			if strVal, ok := value.(string); ok && column.Length > 0 && len(strVal) > column.Length {
				if column.Type != "decimal" && column.Type != "numeric" {
					value = strVal[:column.Length]
				}
			}

			// Retry on UNIQUE violation.
			if tracker.isUniqueColumn(column.Name) {
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
				if column.Name == "id" {
					value = int64(0)
				} else {
					value = placeholderValue
				}
			}

			row[column.Name] = value
		}

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

