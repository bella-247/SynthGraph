package planner

import (
	"testing"

	"synthgraph/internal/schema"
)

// ── Cycle tests ─────────────────────────────────────────────────────────────

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
	// a: pk(id1, id2), b: pk(id), a references b on (id) with nullable
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
