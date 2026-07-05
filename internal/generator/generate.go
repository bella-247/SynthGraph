package generator

import "synthgraph/internal/planner"

// Generate consumes a GenerationPlan and GenerationContext and produces a
// Dataset with all rows for every table. If a single table fails, generation
// continues with the remaining tables and the error is recorded in
// Dataset.Errors. Only a complete failure of every table causes Generate to
// return an error.
func Generate(plan *planner.GenerationPlan, ctx *GenerationContext) (*Dataset, error) {
	// Pre-compute FK column → referenced table mapping for efficient lookups.
	fkMap := buildFKColumnMap(ctx.Graph)

	// Build enum name → values map for enum column generation.
	enumValues := buildEnumValues(ctx.Model)

	// Track generated PK values per table for FK resolution.
	tablePKs := make(map[string][]any)

	dataset := &Dataset{
		Tables: make([]*GeneratedTable, 0, len(plan.Order)),
	}

	// Phase 1: Generate each table in planner order.
	for _, tablePlan := range plan.Order {
		generatedTable, err := generateTable(tablePlan, ctx, fkMap, enumValues, tablePKs)
		if err != nil {
			dataset.Errors = append(dataset.Errors, PartialError{
				Table: tablePlan.TableName,
				Err:   err,
			})
			continue
		}
		dataset.Tables = append(dataset.Tables, generatedTable)

		// Collect PK values for FK resolution by downstream tables.
		pkValues := extractPKValues(generatedTable, tablePlan.Table)
		tablePKs[tablePlan.TableName] = pkValues
	}

	// Phase 2: Backfill deferred FK columns.
	if len(plan.DeferredFKs) > 0 {
		if err := backfillDeferredFKs(dataset, plan.DeferredFKs, tablePKs, ctx); err != nil {
			dataset.Errors = append(dataset.Errors, PartialError{
				Table: "(backfill)",
				Err:   err,
			})
		}
	}

	// If no tables succeeded, report the first error.
	if len(dataset.Tables) == 0 && len(dataset.Errors) > 0 {
		return dataset, dataset.Errors[0].Err
	}

	return dataset, nil
}
