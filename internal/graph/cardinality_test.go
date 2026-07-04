package graph_test

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// ── Cardinality inference ───────────────────────────────────────────────────

func TestBuild_CardinalityOneToOne(t *testing.T) {
	// user_profiles has PK = user_id, which is also the FK → users.
	// This means one_to_one: each user has at most one profile.
	userProfilesTable := &schema.Table{
		Name:       "user_profiles",
		PrimaryKey: []string{"user_id"},
		Columns: []schema.Column{
			makePKColumn("user_id", "int"),
			makeColumn("bio", "text"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, userProfilesTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	// Check references edge cardinality.
	fkEdge := findEdge(schemaGraph, "table:user_profiles", "table:users", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:user_profiles to table:users")
	}
	fkMetadata := fkEdge.Metadata.(*graph.FKMetadata)
	if fkMetadata.Cardinality != graph.CardinalityOneToOne {
		t.Errorf("wanted cardinality %q, got %q", graph.CardinalityOneToOne, fkMetadata.Cardinality)
	}

	// referenced_by and depends_on edges should also carry cardinality.
	refByEdge := findEdge(schemaGraph, "table:users", "table:user_profiles", graph.EdgeKindReferencedBy)
	if refByEdge == nil {
		t.Fatal("expected referenced_by edge from table:users to table:user_profiles")
	}
	refByMetadata := refByEdge.Metadata.(*graph.FKMetadata)
	if refByMetadata.Cardinality != graph.CardinalityOneToOne {
		t.Errorf("referenced_by: wanted cardinality %q, got %q", graph.CardinalityOneToOne, refByMetadata.Cardinality)
	}
}

func TestBuild_CardinalityOneToMany(t *testing.T) {
	// orders has FK user_id → users.id, but PK is order's own 'id' column.
	// This is the default one_to_many: one user has many orders.
	ordersTable := &schema.Table{
		Name:       "orders",
		PrimaryKey: []string{"id"},
		Columns: []schema.Column{
			makePKColumn("id", "int"),
			makeColumn("user_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	fkEdge := findEdge(schemaGraph, "table:orders", "table:users", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:orders to table:users")
	}
	fkMetadata := fkEdge.Metadata.(*graph.FKMetadata)
	if fkMetadata.Cardinality != graph.CardinalityOneToMany {
		t.Errorf("wanted cardinality %q, got %q", graph.CardinalityOneToMany, fkMetadata.Cardinality)
	}
}

func TestBuild_CardinalityManyToMany(t *testing.T) {
	// product_categories is a junction table with composite PK (product_id, category_id).
	// Both PK columns are also FK columns, so cardinality should be many_to_many.
	productCategoriesTable := &schema.Table{
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
	model := makeModel([]*schema.Table{productsTable, categoriesTable, productCategoriesTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	// First FK: product_id → products.id
	fkEdge1 := findEdge(schemaGraph, "table:product_categories", "table:products", graph.EdgeKindReferences)
	if fkEdge1 == nil {
		t.Fatal("expected references edge from table:product_categories to table:products")
	}
	fkMeta1 := fkEdge1.Metadata.(*graph.FKMetadata)
	if fkMeta1.Cardinality != graph.CardinalityManyToMany {
		t.Errorf("product FK: wanted cardinality %q, got %q", graph.CardinalityManyToMany, fkMeta1.Cardinality)
	}

	// Second FK: category_id → categories.id
	fkEdge2 := findEdge(schemaGraph, "table:product_categories", "table:categories", graph.EdgeKindReferences)
	if fkEdge2 == nil {
		t.Fatal("expected references edge from table:product_categories to table:categories")
	}
	fkMeta2 := fkEdge2.Metadata.(*graph.FKMetadata)
	if fkMeta2.Cardinality != graph.CardinalityManyToMany {
		t.Errorf("category FK: wanted cardinality %q, got %q", graph.CardinalityManyToMany, fkMeta2.Cardinality)
	}
}

func TestBuild_CardinalityCompositePKSingleColumnFK(t *testing.T) {
	// A table with composite PK (id_a, id_b) but an FK on only one of them.
	// The FK is not the full PK, so it should be one_to_many, not one_to_one.
	detailTable := &schema.Table{
		Name:       "details",
		PrimaryKey: []string{"id_a", "id_b"},
		Columns: []schema.Column{
			makePKColumn("id_a", "int"),
			makePKColumn("id_b", "int"),
			makeColumn("group_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"group_id"}, "groups", []string{"id"}),
		},
	}
	groupsTable := makeTable("groups", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{groupsTable, detailTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	fkEdge := findEdge(schemaGraph, "table:details", "table:groups", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:details to table:groups")
	}
	fkMetadata := fkEdge.Metadata.(*graph.FKMetadata)
	if fkMetadata.Cardinality != graph.CardinalityOneToMany {
		t.Errorf("single-column FK on composite PK: wanted cardinality %q, got %q",
			graph.CardinalityOneToMany, fkMetadata.Cardinality)
	}
}

func TestBuild_CardinalityNoPK(t *testing.T) {
	// A table with no primary key but with FKs must not panic and should default
	// to one_to_many (since the FK cannot be the PK if there is no PK).
	tableNoPK := &schema.Table{
		Name:    "sessions",
		Columns: []schema.Column{makeColumn("user_id", "int")},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, tableNoPK}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	fkEdge := findEdge(schemaGraph, "table:sessions", "table:users", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:sessions to table:users")
	}
	fkMetadata := fkEdge.Metadata.(*graph.FKMetadata)
	if fkMetadata.Cardinality != graph.CardinalityOneToMany {
		t.Errorf("FK with no PK: wanted cardinality %q, got %q", graph.CardinalityOneToMany, fkMetadata.Cardinality)
	}
}

func TestBuild_CardinalityAllEdgeKinds(t *testing.T) {
	// Verify that cardinality is set on all three edge kinds from the same FK:
	// references, referenced_by, depends_on — all should carry the same value.
	ordersTable := &schema.Table{
		Name:       "orders",
		PrimaryKey: []string{"id"},
		Columns: []schema.Column{
			makePKColumn("id", "int"),
			makeColumn("user_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	expectedCardinality := graph.CardinalityOneToMany

	checkEdge := func(from, to string, kind graph.EdgeKind) {
		edge := findEdge(schemaGraph, from, to, kind)
		if edge == nil {
			t.Errorf("expected edge %s → %s (kind=%s)", from, to, kind)
			return
		}
		fkMeta := edge.Metadata.(*graph.FKMetadata)
		if fkMeta.Cardinality != expectedCardinality {
			t.Errorf("edge %s (%s → %s): wanted cardinality %q, got %q",
				kind, from, to, expectedCardinality, fkMeta.Cardinality)
		}
	}

	checkEdge("table:orders", "table:users", graph.EdgeKindReferences)
	checkEdge("table:users", "table:orders", graph.EdgeKindReferencedBy)
	checkEdge("table:orders", "table:users", graph.EdgeKindDependsOn)
}
