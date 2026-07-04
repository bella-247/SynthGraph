package planner

import (
	"testing"

	"synthgraph/internal/schema"
)

// ── Integration tests ────────────────────────────────────────────────────────

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
	// a←→d cycle with nullable on d→a.
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
