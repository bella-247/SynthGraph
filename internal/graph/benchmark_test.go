package graph_test

import (
	"fmt"
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// makeBigSchema creates a model with n tables forming a chain of FK references.
func makeBigSchema(tableCount int) *schema.Model {
	tables := make([]*schema.Table, tableCount)
	for i := range tableCount {
		name := fmt.Sprintf("table_%d", i)
		columns := []schema.Column{
			{Name: "id", Type: "int", IsPrimaryKey: true},
			{Name: "val", Type: "varchar"},
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
			Name:        name,
			Columns:     columns,
			PrimaryKey:  []string{"id"},
			ForeignKeys: fks,
		}
	}
	return &schema.Model{Tables: tables}
}

func BenchmarkGraph_Build(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, count := range sizes {
		model := makeBigSchema(count)
		b.Run(fmt.Sprintf("tables_%d", count), func(b *testing.B) {
			for range b.N {
				_, err := graph.Build(model)
				if err != nil {
					b.Fatalf("graph.Build: %v", err)
				}
			}
		})
	}
}

func BenchmarkGraph_Edges(b *testing.B) {
	g, _ := graph.Build(makeBigSchema(100))
	b.ResetTimer()
	for range b.N {
		for range g.Edges {
		}
	}
}
