package generator

import "synthgraph/internal/planner"

// Generate consumes a GenerationPlan and GenerationContext and produces a
// complete Dataset with all rows for every table.
//
// The generation follows the plan's order exactly. For acyclic tables,
// referenced tables are already populated when FK-dependent tables are
// generated. For cyclic tables, the breakpoint FK column is inserted as
// NULL and backfilled in the final phase.
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
			return nil, err
		}
		dataset.Tables = append(dataset.Tables, generatedTable)

		// Collect PK values for FK resolution by downstream tables.
		pkValues := extractPKValues(generatedTable, tablePlan.Table)
		tablePKs[tablePlan.TableName] = pkValues
	}

	// Phase 2: Backfill deferred FK columns.
	if len(plan.DeferredFKs) > 0 {
		if err := backfillDeferredFKs(dataset, plan.DeferredFKs, tablePKs, ctx); err != nil {
			return nil, err
		}
	}

	return dataset, nil
}
