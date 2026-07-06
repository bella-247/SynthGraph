package generator

import (
	"fmt"
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// makeLargeSchema creates a model with n tables, each with 5 columns.
// Tables are FK-chained linearly: table_i → table_{i-1} (except table_0).
func makeLargeSchema(tableCount int) *schema.Model {
	tables := make([]*schema.Table, tableCount)
	for i := range tableCount {
		columns := []schema.Column{
			{Name: "id", Type: "int", IsPrimaryKey: true},
			{Name: "name", Type: "varchar"},
			{Name: "email", Type: "varchar"},
			{Name: "score", Type: "decimal"},
			{Name: "active", Type: "boolean"},
		}
		fks := []schema.ForeignKey{}
		if i > 0 {
			fks = append(fks, schema.ForeignKey{
				Columns:    []string{"parent_id"},
				RefTable:   fmt.Sprintf("table_%d", i-1),
				RefColumns: []string{"id"},
			})
			columns = append(columns, schema.Column{Name: "parent_id", Type: "int"})
		}
		tables[i] = &schema.Table{
			Name:       fmt.Sprintf("table_%d", i),
			Columns:    columns,
			PrimaryKey: []string{"id"},
			ForeignKeys: fks,
		}
	}
	return &schema.Model{Tables: tables}
}

// makeWideSchema creates a model with one table containing n columns.
func makeWideSchema(columnCount int) *schema.Model {
	columns := make([]schema.Column, columnCount)
	columns[0] = schema.Column{Name: "id", Type: "int", IsPrimaryKey: true}
	for i := 1; i < columnCount; i++ {
		columns[i] = schema.Column{Name: fmt.Sprintf("col_%d", i), Type: "varchar"}
	}
	return &schema.Model{
		Tables: []*schema.Table{
			{
				Name:       "wide",
				Columns:    columns,
				PrimaryKey: []string{"id"},
			},
		},
	}
}

// BenchmarkGenerate_FullPipeline benchmarks the entire pipeline end-to-end.
func BenchmarkGenerate_FullPipeline(b *testing.B) {
	sizes := []struct {
		name       string
		tableCount int
		rowCount   int
	}{
		{"tables_10_rows_100", 10, 100},
		{"tables_50_rows_100", 50, 100},
		{"tables_50_rows_1000", 50, 1000},
		{"tables_100_rows_100", 100, 100},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			model := makeLargeSchema(size.tableCount)
			g, err := graph.Build(model)
			if err != nil {
				b.Fatalf("graph.Build: %v", err)
			}
			semantic.ResolveColumns(model)
			sg, err := semantic.Build(g)
			if err != nil {
				b.Fatalf("semantic.Build: %v", err)
			}
			plan, err := planner.BuildPlan(g, model, size.rowCount)
			if err != nil {
				b.Fatalf("planner.BuildPlan: %v", err)
			}
			ctx := &GenerationContext{GlobalSeed: 42, Model: model, Graph: g, SemanticGraph: sg}

			b.ResetTimer()
			for range b.N {
				_, err := Generate(plan, ctx)
				if err != nil {
					b.Fatalf("Generate: %v", err)
				}
			}
		})
	}
}

// BenchmarkGraphBuild benchmarks graph building with varying table counts.
func BenchmarkGraphBuild(b *testing.B) {
	tableCounts := []int{10, 50, 100, 500}
	for _, count := range tableCounts {
		model := makeLargeSchema(count)
		b.Run(fmt.Sprintf("tables_%d", count), func(b *testing.B) {
			for range b.N {
				graph.Build(model)
			}
		})
	}
}

// BenchmarkSemanticBuild benchmarks semantic graph inference.
func BenchmarkSemanticBuild(b *testing.B) {
	tableCounts := []int{10, 50, 100}
	for _, count := range tableCounts {
		model := makeLargeSchema(count)
		g, _ := graph.Build(model)
		b.Run(fmt.Sprintf("tables_%d", count), func(b *testing.B) {
			for range b.N {
				semantic.Build(g)
			}
		})
	}
}

// BenchmarkPlannerBuild benchmarks the planner with varying table counts.
func BenchmarkPlannerBuild(b *testing.B) {
	tableCounts := []int{10, 50, 100, 500}
	for _, count := range tableCounts {
		model := makeLargeSchema(count)
		g, _ := graph.Build(model)
		b.Run(fmt.Sprintf("tables_%d", count), func(b *testing.B) {
			for range b.N {
				planner.BuildPlan(g, model, 100)
			}
		})
	}
}

// BenchmarkGenerate_WideTable benchmarks generation of a wide table.
func BenchmarkGenerate_WideTable(b *testing.B) {
	columnCounts := []int{10, 50, 100}
	for _, count := range columnCounts {
		model := makeWideSchema(count)
		g, _ := graph.Build(model)
		semantic.ResolveColumns(model)
		sg, _ := semantic.Build(g)
		plan, _ := planner.BuildPlan(g, model, 100)
		ctx := &GenerationContext{GlobalSeed: 42, Model: model, Graph: g, SemanticGraph: sg}

		b.Run(fmt.Sprintf("columns_%d", count), func(b *testing.B) {
			for range b.N {
				Generate(plan, ctx)
			}
		})
	}
}
