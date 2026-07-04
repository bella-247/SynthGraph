package graph_test

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// makeModel is a convenience constructor for building a schema.Model in tests.
func makeModel(tables []*schema.Table, enums []schema.EnumType) *schema.Model {
	tableMap := make(map[string]*schema.Table, len(tables))
	for _, table := range tables {
		tableMap[table.Name] = table
	}
	return &schema.Model{
		Tables:   tables,
		TableMap: tableMap,
		Enums:    enums,
	}
}

// makeTable builds a schema.Table with the given name and columns.
func makeTable(name string, columns ...schema.Column) *schema.Table {
	return &schema.Table{
		Name:    name,
		Columns: columns,
	}
}

// makeColumn builds a schema.Column with the given name and type.
func makeColumn(name, typeName string) schema.Column {
	return schema.Column{Name: name, Type: typeName, Nullable: true}
}

// makePKColumn builds a schema.Column that is part of the primary key.
func makePKColumn(name, typeName string) schema.Column {
	return schema.Column{Name: name, Type: typeName, Nullable: false, IsPrimaryKey: true}
}

// makeFK builds a schema.ForeignKey from the given parameters.
func makeFK(localColumns []string, refTable string, refColumns []string) schema.ForeignKey {
	return schema.ForeignKey{
		Columns:    localColumns,
		RefTable:   refTable,
		RefColumns: refColumns,
	}
}

// countEdgesOfKind counts how many edges in the graph have the given kind.
func countEdgesOfKind(schemaGraph *graph.Graph, kind graph.EdgeKind) int {
	count := 0
	for _, edge := range schemaGraph.Edges {
		if edge.Kind == kind {
			count++
		}
	}
	return count
}

// findEdge returns the first edge matching the given From, To, and Kind, or nil.
func findEdge(schemaGraph *graph.Graph, from, to string, kind graph.EdgeKind) *graph.Edge {
	for _, edge := range schemaGraph.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return edge
		}
	}
	return nil
}

// ── Phase 1: Empty schema ─────────────────────────────────────────────────────

func TestBuild_EmptySchema(t *testing.T) {
	emptyModel := makeModel(nil, nil)

	schemaGraph, err := graph.Build(emptyModel)

	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}
	if len(schemaGraph.NodeList) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(schemaGraph.NodeList))
	}
	if len(schemaGraph.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(schemaGraph.Edges))
	}
	if schemaGraph.Nodes == nil {
		t.Error("Nodes map must not be nil, even for an empty graph")
	}
}
