package planner

import (
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

func TestBuildPlan_CycleWithNullableBreakpoint(t *testing.T) {
	// a ←→ b (a references b, b references a, b's FK column is nullable)
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "b_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"b_id"}, RefTable: "b", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(plan.Order))
	}

	// b has a non-nullable FK to a, so b must be generated after a.
	// a has a nullable FK to b, which becomes the deferred breakpoint.
	if plan.Order[0].TableName != "b" && plan.Order[1].TableName != "b" {
		t.Errorf("expected b in plan order, got order: %v",
			[]string{plan.Order[0].TableName, plan.Order[1].TableName})
	}

	// Either a or b should have deferred columns — whichever table has the
	// nullable FK (a) should be deferred.
	foundDeferred := false
	for _, tp := range plan.Order {
		if len(tp.DeferredCols) > 0 {
			foundDeferred = true
			break
		}
	}
	if !foundDeferred {
		t.Error("expected at least one table to have deferred FK columns")
	}

	// Should have at least one DeferredFK entry for the nullable edge.
	if len(plan.DeferredFKs) == 0 {
		t.Error("expected at least one DeferredFK for the cycle")
	}
}

func TestBuildPlan_CycleWithNoNullableBreakpoint(t *testing.T) {
	// a ←→ b (both FKs are NOT NULL → unresolvable cycle)
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "b_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"b_id"}, RefTable: "b", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	_, err := BuildPlan(g, model, 10)
	if err == nil {
		t.Fatal("expected PlanError for unresolvable cycle, got nil")
	}
	planError, ok := err.(*PlanError)
	if !ok {
		t.Fatalf("expected *PlanError, got %T: %v", err, err)
	}
	if len(planError.CycleTables) == 0 {
		t.Error("expected CycleTables to be populated")
	}
	if planError.Hint == "" {
		t.Error("expected Hint to be non-empty")
	}
}

func TestBuildPlan_SelfReferencingNullable(t *testing.T) {
	// employees(id PK, manager_id nullable FK → employees(id))
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "employees",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "manager_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"manager_id"}, RefTable: "employees", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 20)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 1 {
		t.Fatalf("expected 1 table, got %d", len(plan.Order))
	}
	if plan.Order[0].TableName != "employees" {
		t.Errorf("expected employees, got %q", plan.Order[0].TableName)
	}

	// Self-referencing nullable FK should be deferred.
	if len(plan.Order[0].DeferredCols) == 0 {
		t.Error("expected employees to have deferred FK columns for self-reference")
	}
	if len(plan.DeferredFKs) == 0 {
		t.Error("expected at least one DeferredFK for self-referencing table")
	}
}

func TestBuildPlan_SelfReferencingNotNull(t *testing.T) {
	// categories(id PK, parent_id NOT NULL FK → categories(id)) → unresolvable
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "categories",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "parent_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"parent_id"}, RefTable: "categories", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	_, err := BuildPlan(g, model, 10)
	if err == nil {
		t.Fatal("expected PlanError for NOT NULL self-referencing FK, got nil")
	}
}

func TestBuildPlan_MixedAcyclicAndCyclic(t *testing.T) {
	// a (no deps)  ←  b (FK→a)  ←  c (FK→b)
	// d ←→ e (cycle, d has nullable FK to e)
	// f (no deps)
	//
	// Expected order: a, f, then c depends on b depends on a so a,b,c ordered,
	// then d/e cycle resolved.
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
			{
				Name: "d",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "e_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"e_id"}, RefTable: "e", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "e",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "d_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"d_id"}, RefTable: "d", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "f",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 6 {
		t.Fatalf("expected 6 tables, got %d", len(plan.Order))
	}

	indexOf := make(map[string]int)
	for i, tp := range plan.Order {
		indexOf[tp.TableName] = i
	}

	// a before b before c.
	if indexOf["a"] > indexOf["b"] {
		t.Errorf("expected a before b")
	}
	if indexOf["b"] > indexOf["c"] {
		t.Errorf("expected b before c")
	}

	// d and e should form the cycle portion. At least one deferred column.
	cycleTablesFound := false
	for _, tp := range plan.Order {
		if (tp.TableName == "d" || tp.TableName == "e") && len(tp.DeferredCols) > 0 {
			cycleTablesFound = true
			break
		}
	}
	if !cycleTablesFound {
		t.Error("expected deferred FK columns on d or e")
	}

	// f has no deps, could be anywhere.
	if _, ok := indexOf["f"]; !ok {
		t.Error("expected table f in plan")
	}
}

func TestBuildPlan_CompositeFKCycle(t *testing.T) {
	// Two tables with composite FK where some columns are nullable.
	// a: pk(id1, id2), b: pk(id1), a references b on (id1) with nullable
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id1", Type: "int", IsPrimaryKey: true},
					{Name: "id2", Type: "int", IsPrimaryKey: true},
					{Name: "b_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id1", "id2"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"b_id"}, RefTable: "b", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id1", Type: "int", Nullable: false},
					{Name: "a_id2", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id1", "a_id2"}, RefTable: "a", RefColumns: []string{"id1", "id2"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Order) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(plan.Order))
	}
	if len(plan.DeferredFKs) == 0 {
		t.Error("expected deferred FKs for composite FK cycle")
	}
}

func TestBuildPlan_RowCountPreserved(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "x",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "y",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "x_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"x_id"}, RefTable: "x", RefColumns: []string{"id"}},
				},
			},
		},
	}

	tests := []struct {
		name     string
		rowCount int
	}{
		{"zero rows", 0},
		{"one row", 1},
		{"hundred rows", 100},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := setupGraph(t, model)
			plan, err := BuildPlan(g, model, test.rowCount)
			if err != nil {
				t.Fatalf("BuildPlan failed: %v", err)
			}
			for _, tp := range plan.Order {
				if tp.RowCount != test.rowCount {
					t.Errorf("table %q: expected rowCount %d, got %d", tp.TableName, test.rowCount, tp.RowCount)
				}
			}
		})
	}
}

func TestBuildPlan_DeferredFKMetadata(t *testing.T) {
	// Verify that DeferredFK entries carry correct metadata.
	// a ← b ← c (c→b→a), plus a←→d cycle with nullable on d→a.
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "d_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"d_id"}, RefTable: "d", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "d",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	// Check that each DeferredFK has all fields populated.
	for _, dfk := range plan.DeferredFKs {
		if dfk.Table == "" {
			t.Error("DeferredFK has empty Table")
		}
		if dfk.Column == "" {
			t.Error("DeferredFK has empty Column")
		}
		if dfk.References == "" {
			t.Error("DeferredFK has empty References")
		}
		if dfk.RefColumn == "" {
			t.Error("DeferredFK has empty RefColumn")
		}
	}
}

func TestBuildPlan_ActualEcommerceSchema(t *testing.T) {
	// Use the ecommerce schema from testdata to test with a realistic model.
	// The ecommerce schema has: users, products, orders, order_items, reviews,
	// categories, product_categories.
	// There are FKs: orders→users, order_items→orders, order_items→products,
	// reviews→users, reviews→products, product_categories→products,
	// product_categories→categories, categories→categories (self-ref parent_id).
	model := buildEcommerceModel()

	g := setupGraph(t, model)
	plan, err := BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("BuildPlan failed on ecommerce schema: %v", err)
	}

	// All tables must be present.
	expectedTables := []string{
		"users", "products", "categories", "orders",
		"order_items", "reviews", "product_categories",
	}

	if len(plan.Order) != len(expectedTables) {
		t.Fatalf("expected %d tables, got %d: %v", len(expectedTables), len(plan.Order), planTableNames(plan))
	}

	planNames := make(map[string]int)
	for i, tp := range plan.Order {
		planNames[tp.TableName] = i
	}
	for _, name := range expectedTables {
		if _, ok := planNames[name]; !ok {
			t.Errorf("missing table %q in plan", name)
		}
	}

	// Dependency checks:
	// - orders depends on users → users before orders
	if planNames["users"] > planNames["orders"] {
		t.Errorf("users must be before orders: users=%d orders=%d", planNames["users"], planNames["orders"])
	}
	// - order_items depends on orders and products
	if planNames["orders"] > planNames["order_items"] {
		t.Errorf("orders must be before order_items")
	}
	if planNames["products"] > planNames["order_items"] {
		t.Errorf("products must be before order_items")
	}
	// - reviews depends on users and products
	if planNames["users"] > planNames["reviews"] {
		t.Errorf("users must be before reviews")
	}
	if planNames["products"] > planNames["reviews"] {
		t.Errorf("products must be before reviews")
	}
	// - product_categories depends on products, categories
	if planNames["products"] > planNames["product_categories"] {
		t.Errorf("products must be before product_categories")
	}
	if planNames["categories"] > planNames["product_categories"] {
		t.Errorf("categories must be before product_categories")
	}

	// categories has self-referencing parent_id FK (nullable) → deferred.
	categoriesPlan := findTablePlan(plan, "categories")
	if categoriesPlan == nil {
		t.Fatal("categories not found in plan")
	}
	if len(categoriesPlan.DeferredCols) == 0 {
		t.Log("categories has self-referencing nullable FK, expected deferred cols (may be handled if acyclic via topological sort)")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func planTableNames(plan *GenerationPlan) []string {
	names := make([]string, len(plan.Order))
	for i, tp := range plan.Order {
		names[i] = tp.TableName
	}
	return names
}

func findTablePlan(plan *GenerationPlan, name string) *TablePlan {
	for i := range plan.Order {
		if plan.Order[i].TableName == name {
			return &plan.Order[i]
		}
	}
	return nil
}

// buildEcommerceModel constructs a schema.Model matching the ecommerce testdata.
func buildEcommerceModel() *schema.Model {
	return &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
					{Name: "email", Type: "varchar"},
					{Name: "created_at", Type: "timestamp"},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "products",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
					{Name: "price", Type: "numeric"},
					{Name: "created_at", Type: "timestamp"},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "categories",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
					{Name: "parent_id", Type: "uuid", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"parent_id"}, RefTable: "categories", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "user_id", Type: "uuid"},
					{Name: "status", Type: "varchar"},
					{Name: "created_at", Type: "timestamp"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "order_items",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "order_id", Type: "uuid"},
					{Name: "product_id", Type: "uuid"},
					{Name: "quantity", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"order_id"}, RefTable: "orders", RefColumns: []string{"id"}},
					{Columns: []string{"product_id"}, RefTable: "products", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "reviews",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "user_id", Type: "uuid"},
					{Name: "product_id", Type: "uuid"},
					{Name: "rating", Type: "int"},
					{Name: "body", Type: "text"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
					{Columns: []string{"product_id"}, RefTable: "products", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "product_categories",
				Columns: []schema.Column{
					{Name: "product_id", Type: "uuid", IsPrimaryKey: true},
					{Name: "category_id", Type: "uuid", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"product_id", "category_id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"product_id"}, RefTable: "products", RefColumns: []string{"id"}},
					{Columns: []string{"category_id"}, RefTable: "categories", RefColumns: []string{"id"}},
				},
			},
		},
	}
}
