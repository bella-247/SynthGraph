package mermaid_test

import (
	"strings"
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/render/mermaid"
	"synthgraph/internal/schema"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

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

func makeTable(name string, columns ...schema.Column) *schema.Table {
	return &schema.Table{
		Name:    name,
		Columns: columns,
	}
}

func makeColumn(name, typeName string) schema.Column {
	return schema.Column{Name: name, Type: typeName, Nullable: true}
}

func makePKColumn(name, typeName string) schema.Column {
	return schema.Column{Name: name, Type: typeName, Nullable: false, IsPrimaryKey: true}
}

func makeFK(localColumns []string, refTable string, refColumns []string) schema.ForeignKey {
	return schema.ForeignKey{
		Columns:    localColumns,
		RefTable:   refTable,
		RefColumns: refColumns,
	}
}

func buildGraph(t *testing.T, model *schema.Model) *graph.Graph {
	t.Helper()
	g, err := graph.Build(model)
	if err != nil {
		t.Fatalf("graph.Build() failed: %v", err)
	}
	return g
}

// contains checks if s contains the given substring.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ── Basic rendering ─────────────────────────────────────────────────────────

func TestRender_EmptyGraph(t *testing.T) {
	model := makeModel(nil, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !strings.HasPrefix(out, "erDiagram\n") {
		t.Errorf("output should start with 'erDiagram', got %q", out[:min(len(out), 20)])
	}
	// No entities or relationships
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected only the preamble line, got %d lines", len(lines))
	}
}

func TestRender_SimpleTable(t *testing.T) {
	model := makeModel([]*schema.Table{
		makeTable("users",
			makePKColumn("id", "int"),
			makeColumn("name", "varchar"),
		),
	}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !contains(out, "erDiagram") {
		t.Error("output missing erDiagram preamble")
	}
	if !contains(out, "users {") {
		t.Error("output missing 'users {' entity block")
	}
	if !contains(out, "int id PK") {
		t.Error("output missing PK column 'int id PK'")
	}
	if !contains(out, "varchar name") {
		t.Error("output missing column 'varchar name'")
	}
	if !contains(out, "}") {
		t.Error("output missing closing brace")
	}
}

// ── Enum rendering ──────────────────────────────────────────────────────────

func TestRender_Enum(t *testing.T) {
	enums := []schema.EnumType{
		{Name: "order_status", Values: []string{"pending", "shipped", "delivered"}},
	}
	model := makeModel(nil, enums)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g, mermaid.WithEnums(true))
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !contains(out, "order_status {") {
		t.Error("output missing enum entity 'order_status {'")
	}
	if !contains(out, "string pending") {
		t.Error("output missing enum value 'string pending'")
	}
	if !contains(out, "string shipped") {
		t.Error("output missing enum value 'string shipped'")
	}
	if !contains(out, "string delivered") {
		t.Error("output missing enum value 'string delivered'")
	}
}

func TestRender_EnumsDisabled(t *testing.T) {
	enums := []schema.EnumType{
		{Name: "order_status", Values: []string{"pending", "shipped"}},
	}
	model := makeModel(nil, enums)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g, mermaid.WithEnums(false))
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if contains(out, "order_status") {
		t.Error("expected no enum entity when enums disabled")
	}
}

// ── FK rendering ────────────────────────────────────────────────────────────

func TestRender_ForeignKey(t *testing.T) {
	ordersTable := &schema.Table{
		Name:    "orders",
		Columns: []schema.Column{makeColumn("user_id", "int")},
		ForeignKeys: []schema.ForeignKey{makeFK([]string{"user_id"}, "users", []string{"id"})},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	// FK annotation on column
	if !contains(out, "int user_id FK") {
		t.Error("output missing FK annotation on column 'int user_id FK'")
	}

	// Relationship: users ||--o{ orders : "FK user_id"
	if !contains(out, "||--o{") {
		t.Error("output missing one-to-many cardinality '||--o{'")
	}
	if !contains(out, "FK user_id") {
		t.Error("output missing FK label 'FK user_id'")
	}
	if !contains(out, "users ") {
		t.Error("output missing parent entity name 'users'")
	}
	if !contains(out, " orders ") {
		t.Error("output missing child entity name 'orders'")
	}
}

func TestRender_OneToOneRelation(t *testing.T) {
	// user_profiles PK = user_id, FK → users.id ⇒ one_to_one
	profileTable := &schema.Table{
		Name:       "user_profiles",
		PrimaryKey: []string{"user_id"},
		Columns:    []schema.Column{makePKColumn("user_id", "int")},
		ForeignKeys: []schema.ForeignKey{makeFK([]string{"user_id"}, "users", []string{"id"})},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, profileTable}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !contains(out, "||--||") {
		t.Error("expected one-to-one cardinality '||--||'")
	}
	// FK annotation should NOT appear because PK takes precedence
	if !contains(out, "int user_id PK") {
		t.Error("expected PK annotation, not FK, for one-to-one column")
	}
}

func TestRender_ManyToManyRelation(t *testing.T) {
	// product_categories: composite PK (product_id, category_id), both FKs → junction table.
	// Each individual FK from the junction table to a parent is one-to-many.
	jcTable := &schema.Table{
		Name:       "product_categories",
		PrimaryKey: []string{"product_id", "category_id"},
		Columns: []schema.Column{
			makePKColumn("product_id", "int"),
			makePKColumn("category_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"product_id"}, "products", []string{"id"}),
			makeFK([]string{"category_id"}, "categories", []string{"id"}),
		},
	}
	productsTable := makeTable("products", makePKColumn("id", "int"))
	categoriesTable := makeTable("categories", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{productsTable, categoriesTable, jcTable}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	// Each individual FK is one-to-many — no }o--o{ notation from individual FKs.
	if !contains(out, "||--o{") {
		t.Error("expected one-to-many cardinality '||--o{' for junction table FKs")
	}
	if contains(out, "}o--o{") {
		t.Error("unexpected many-to-many notation '}o--o{' — each FK is a separate one-to-many")
	}

	// Both FK relationships should appear
	if !contains(out, "FK product_id") {
		t.Error("output missing FK product_id label")
	}
	if !contains(out, "FK category_id") {
		t.Error("output missing FK category_id label")
	}
}

func TestRender_SelfRefForeignKey(t *testing.T) {
	empTable := &schema.Table{
		Name:    "employees",
		Columns: []schema.Column{makePKColumn("id", "int"), makeColumn("manager_id", "int")},
		ForeignKeys: []schema.ForeignKey{makeFK([]string{"manager_id"}, "employees", []string{"id"})},
	}
	model := makeModel([]*schema.Table{empTable}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	// Self-referencing: employees ||--o{ employees : "FK manager_id"
	if !contains(out, "employees ||--o{ employees") {
		t.Error("expected self-referencing relationship")
	}
	if !contains(out, "FK manager_id") {
		t.Error("expected 'FK manager_id' label")
	}
}

// ── Type formatting ─────────────────────────────────────────────────────────

func TestRender_ColumnTypeLength(t *testing.T) {
	model := makeModel([]*schema.Table{
		makeTable("items",
			makePKColumn("id", "int"),
			schema.Column{Name: "title", Type: "varchar", Length: 255, Nullable: false},
			schema.Column{Name: "price", Type: "decimal", Length: 10, Precision: 2, Nullable: true},
		),
	}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !contains(out, "varchar(255) title") {
		t.Error("expected 'varchar(255) title'")
	}
	if !contains(out, "decimal(10,2) price") {
		t.Error("expected 'decimal(10,2) price'")
	}
}

// ── Schema-qualified names ──────────────────────────────────────────────────

func TestRender_SchemaQualifiedName(t *testing.T) {
	model := makeModel([]*schema.Table{
		makeTable("inventory.products", makePKColumn("id", "int")),
	}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if contains(out, "inventory.products") {
		t.Error("entity name should not contain dots; expected underscore replacement")
	}
	if !contains(out, "inventory_products") {
		t.Error("entity name should use underscores instead of dots")
	}
}

// ── Render options ──────────────────────────────────────────────────────────

func TestRender_ColumnNullAnnotation(t *testing.T) {
	model := makeModel([]*schema.Table{
		makeTable("users",
			makePKColumn("id", "int"),
			makeColumn("email", "varchar"),
		),
	}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g, mermaid.WithColumnNull(true))
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !contains(out, "int id PK") {
		t.Error("PK column should have PK annotation")
	}
	if !contains(out, "NULL") {
		t.Error("nullable column should show NULL annotation")
	}
}

func TestRender_ColumnDefaultAnnotation(t *testing.T) {
	now := "now()"
	model := makeModel([]*schema.Table{
		makeTable("orders",
			schema.Column{Name: "created_at", Type: "timestamp", Nullable: true, Default: &now},
		),
	}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g, mermaid.WithColumnDefault(true))
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if !contains(out, "DEFAULT now()") {
		t.Error("expected 'DEFAULT now()' annotation")
	}
}

// ── Determinism ─────────────────────────────────────────────────────────────

func TestRender_Determinism(t *testing.T) {
	model := makeModel([]*schema.Table{
		makeTable("a", makePKColumn("id", "int")),
		makeTable("b", makeColumn("val", "text")),
	}, []schema.EnumType{{Name: "e", Values: []string{"x"}}})

	g1 := buildGraph(t, model)
	g2 := buildGraph(t, model)

	out1, err := mermaid.Render(g1)
	if err != nil {
		t.Fatalf("first Render() failed: %v", err)
	}
	out2, err := mermaid.Render(g2)
	if err != nil {
		t.Fatalf("second Render() failed: %v", err)
	}

	if out1 != out2 {
		t.Error("Render() is not deterministic — two identical graphs produced different output")
	}
}

// ── Integration: FK actions ─────────────────────────────────────────────────

func TestRender_FKWithActions(t *testing.T) {
	ordersTable := &schema.Table{
		Name:    "orders",
		Columns: []schema.Column{makeColumn("user_id", "int")},
		ForeignKeys: []schema.ForeignKey{
			{
				Columns:    []string{"user_id"},
				RefTable:   "users",
				RefColumns: []string{"id"},
				OnDelete:   schema.FKCascade,
				OnUpdate:   schema.FKRestrict,
			},
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)
	g := buildGraph(t, model)

	out, err := mermaid.Render(g)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	// Actions are not rendered in the relationship label (by design).
	// The label should just show the FK column mapping.
	if !contains(out, "FK user_id") {
		t.Error("expected FK column label in relationship line")
	}
}

// ── Output well-formedness ──────────────────────────────────────────────────

// TestRender_WellFormed checks that every { has a matching } and that no
// content appears outside entity blocks or relationship lines.
func TestRender_WellFormed(t *testing.T) {
	// Build a realistic graph with tables, FKs, and enums.
	enumStatus := schema.EnumType{Name: "order_status", Values: []string{"pending", "shipped", "delivered"}}
	users := makeTable("users", makePKColumn("id", "bigint"), makeColumn("email", "varchar"))
	orders := &schema.Table{
		Name:    "orders",
		Columns: []schema.Column{makePKColumn("id", "bigint"), makeColumn("user_id", "bigint")},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
		},
	}
	model := makeModel([]*schema.Table{users, orders}, []schema.EnumType{enumStatus})
	g := buildGraph(t, model)

	out, err := mermaid.Render(g, mermaid.WithEnums(true))
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	// Count opening and closing braces in entity blocks only
	// (relationship lines like ||--o{ contain { but those are not entity blocks).
	lines := strings.Split(out, "\n")
	openCount := 0
	closeCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "erDiagram") {
			continue
		}
		// Skip relationship lines (they contain cardinality notation like ||--o{).
		if strings.Contains(trimmed, "||--") || strings.Contains(trimmed, "}o--") {
			continue
		}
		if strings.HasSuffix(trimmed, "{") {
			openCount++
		}
		if trimmed == "}" || strings.HasSuffix(trimmed, "}") {
			closeCount++
		}
	}
	if openCount != closeCount {
		t.Errorf("mismatched entity braces: %d opening vs %d closing\noutput:\n%s", openCount, closeCount, out)
	}

	// Verify entity block structure (no nesting, well-formed).
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "erDiagram") {
			continue
		}
		if strings.Contains(trimmed, "||--") || strings.Contains(trimmed, "}o--") {
			continue
		}
		if strings.HasSuffix(trimmed, "{") && strings.Contains(trimmed, "}") {
			t.Errorf("braces on same line: %q", trimmed)
		}
		if strings.HasSuffix(trimmed, "{") {
			if inBlock {
				t.Errorf("nested block at: %q", trimmed)
			}
			inBlock = true
		}
		if trimmed == "}" || strings.HasSuffix(trimmed, "}") {
			if !inBlock {
				t.Errorf("unexpected closing brace at: %q", trimmed)
			}
			inBlock = false
		}
	}
	if inBlock {
		t.Error("unclosed block — still inside a block at end of output")
	}

	// Verify no duplicate entity blocks (same entity name appearing more than once).
	entityNames := make(map[string]int)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "{") {
			name := strings.TrimSuffix(trimmed, "{")
			name = strings.TrimSpace(name)
			entityNames[name]++
		}
	}
	for name, count := range entityNames {
		if count > 1 {
			t.Errorf("duplicate entity %q appears %d times", name, count)
		}
	}
}
