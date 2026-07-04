package generator

import (
	"fmt"

	"synthgraph/internal/planner"
)

// backfillDeferredFKs replaces NULL values in deferred FK columns with random
// PK values from the referenced tables.
func backfillDeferredFKs(
	dataset *Dataset,
	deferredFKs []planner.DeferredFK,
	tablePKs map[string][]any,
	ctx *GenerationContext,
) error {
	for _, dfk := range deferredFKs {
		refPKs, ok := tablePKs[dfk.References]
		if !ok || len(refPKs) == 0 {
			return &GenError{
				Table:   dfk.Table,
				Column:  dfk.Column,
				Message: fmt.Sprintf("referenced table %q has no generated PK values for backfill", dfk.References),
			}
		}

		tableRNG := newTableRNG(ctx.GlobalSeed, "backfill:"+dfk.Table+":"+dfk.Column)

		table := findGeneratedTable(dataset, dfk.Table)
		if table == nil {
			return &GenError{
				Table:   dfk.Table,
				Message: fmt.Sprintf("table %q not found in generated dataset", dfk.Table),
			}
		}

		for rowIndex := range table.Rows {
			if table.Rows[rowIndex][dfk.Column] == nil {
				table.Rows[rowIndex][dfk.Column] = refPKs[tableRNG.IntN(len(refPKs))]
			}
		}
	}

	return nil
}

// findGeneratedTable finds a GeneratedTable by name in the dataset.
func findGeneratedTable(dataset *Dataset, tableName string) *GeneratedTable {
	for _, table := range dataset.Tables {
		if table.TableName == tableName {
			return table
		}
	}
	return nil
}
