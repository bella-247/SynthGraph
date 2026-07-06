package generator

import (
	"context"
	"errors"

	"synthgraph/internal/planner"
)

// ErrCancelled is returned by Generate when the context is cancelled.
// The caller may still use the partial Dataset returned alongside this error.
var ErrCancelled = errors.New("generation cancelled")

// Generate consumes a GenerationPlan and GenerationContext and produces a
// Dataset with all rows for every table.
//
// The generation follows the plan's order exactly. For acyclic tables,
// referenced tables are already populated when FK-dependent tables are
// generated. For cyclic tables, the breakpoint FK column is inserted as
// NULL and backfilled in the final phase.
//
// If ctx.Context is cancelled during generation, Generate returns the
// partial Dataset along with ErrCancelled. The caller may still export
// the partial data.
//
// If a single table fails, generation continues with the remaining tables
// and the error is recorded in Dataset.Errors. Only a complete failure of
// every table causes Generate to return an error.
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

	totalTables := len(plan.Order)

	// Phase 1: Generate each table in planner order.
	for tableIndex, tablePlan := range plan.Order {
		if isCancelled(ctx.Context) {
			return dataset, ErrCancelled
		}

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

		// Report progress if a callback is registered.
		if ctx.Progress != nil {
			ctx.Progress(tablePlan.TableName, tableIndex+1, totalTables)
		}
	}

	// Phase 2: Backfill deferred FK columns.
	if len(plan.DeferredFKs) > 0 {
		if isCancelled(ctx.Context) {
			return dataset, ErrCancelled
		}
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

// isCancelled returns true when the context has been cancelled.
// A nil context is never cancelled (backward compatible).
func isCancelled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return ctx.Err() != nil
}
