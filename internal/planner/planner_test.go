package planner

import (
	"fmt"
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// setupGraph builds a graph.Graph from a schema.Model for testing.
func setupGraph(t *testing.T, model *schema.Model) *graph.Graph {
	t.Helper()
	g, err := graph.Build(model)
	if err != nil {
		t.Fatalf("graph.Build failed: %v", err)
	}
	return g
}

// ── Basic acyclic tests ──────────────────────────────────────────────────────

func TestBuildPlan_EmptyGraph(t *testing.T) {
	model := &schema.Model{}
	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 0 {
		t.Errorf("expected empty order, got %d tables", len(plan.Order))
	}
	if len(plan.DeferredFKs) != 0 {
		t.Errorf("expected no deferred FKs, got %d", len(plan.DeferredFKs))
	}
}

func TestBuildPlan_SingleTable(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 50)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 1 {
		t.Fatalf("expected 1 table, got %d", len(plan.Order))
	}
	if plan.Order[0].TableName != "users" {
		t.Errorf("expected table 'users', got %q", plan.Order[0].TableName)
	}
	if plan.Order[0].RowCount != 50 {
		t.Errorf("expected rowCount 50, got %d", plan.Order[0].RowCount)
	}
	if len(plan.Order[0].DeferredCols) != 0 {
		t.Errorf("expected no deferred cols, got %v", plan.Order[0].DeferredCols)
	}
}

func TestBuildPlan_IsolatedTables(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "c",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 1)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 3 {
		t.Fatalf("expected 3 tables, got %d", len(plan.Order))
	}
	// All isolated nodes can be generated in any order.
	names := make(map[string]bool, 3)
	for _, tp := range plan.Order {
		names[tp.TableName] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !names[want] {
			t.Errorf("missing table %q in plan", want)
		}
	}
}

func TestBuildPlan_LinearChain(t *testing.T) {
	// a ← b ← c  (c depends on b, b depends on a)
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "c",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "b_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"b_id"}, RefTable: "b", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 3 {
		t.Fatalf("expected 3 tables, got %d", len(plan.Order))
	}

	// Check generation order: a before b before c.
	indexOf := make(map[string]int)
	for i, tp := range plan.Order {
		indexOf[tp.TableName] = i
	}
	if indexOf["a"] > indexOf["b"] {
		t.Errorf("expected a before b, got a=%d b=%d", indexOf["a"], indexOf["b"])
	}
	if indexOf["b"] > indexOf["c"] {
		t.Errorf("expected b before c, got b=%d c=%d", indexOf["b"], indexOf["c"])
	}
	if len(plan.DeferredFKs) != 0 {
		t.Errorf("expected no deferred FKs, got %d", len(plan.DeferredFKs))
	}
}

func TestBuildPlan_ForkedDependencies(t *testing.T) {
	// a ← b and a ← c  (both b and c depend on a, but not on each other)
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "c",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 5)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 3 {
		t.Fatalf("expected 3 tables, got %d", len(plan.Order))
	}

	indexOf := make(map[string]int)
	for i, tp := range plan.Order {
		indexOf[tp.TableName] = i
	}
	// a must be before both b and c.
	if indexOf["a"] > indexOf["b"] {
		t.Errorf("expected a before b, got a=%d b=%d", indexOf["a"], indexOf["b"])
	}
	if indexOf["a"] > indexOf["c"] {
		t.Errorf("expected a before c, got a=%d c=%d", indexOf["a"], indexOf["c"])
	}
}

func BenchmarkTopologicalSort(b *testing.B) {
	g := &graph.Graph{Nodes: make(map[string]*graph.Node)}
	tableNodes := make(map[string]*graph.Node)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("table:t%d", i)
		n := &graph.Node{ID: id, Kind: graph.NodeKindTable}
		g.Nodes[id] = n
		g.NodeList = append(g.NodeList, n)
		tableNodes[id] = n
		if i > 0 {
			g.Edges = append(g.Edges, &graph.Edge{
				From: id, To: fmt.Sprintf("table:t%d", i-1), Kind: graph.EdgeKindReferences,
			})
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topologicalSort(g, tableNodes)
	}
}
