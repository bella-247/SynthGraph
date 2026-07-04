package graph_test

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// ── Phase 5: Contains edges ───────────────────────────────────────────────────

func TestBuild_ContainsEdges(t *testing.T) {
	usersTable := makeTable("users",
		makeColumn("id", "int"),
		makeColumn("name", "varchar"),
	)
	model := makeModel([]*schema.Table{usersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	containsEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindContains)
	if containsEdges != 2 {
		t.Errorf("wanted 2 contains edges, got %d", containsEdges)
	}

	idEdge := findEdge(schemaGraph, "table:users", "column:users.id", graph.EdgeKindContains)
	if idEdge == nil {
		t.Error("expected contains edge from table:users to column:users.id")
	}
	nameEdge := findEdge(schemaGraph, "table:users", "column:users.name", graph.EdgeKindContains)
	if nameEdge == nil {
		t.Error("expected contains edge from table:users to column:users.name")
	}
}

func TestBuild_ContainsEdgesAcrossMultipleTables(t *testing.T) {
	usersTable := makeTable("users", makeColumn("id", "int"), makeColumn("name", "varchar"))
	ordersTable := makeTable("orders", makeColumn("id", "int"), makeColumn("total", "numeric"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	containsEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindContains)
	// 2 columns in users + 2 columns in orders = 4 contains edges
	if containsEdges != 4 {
		t.Errorf("wanted 4 contains edges for 4 total columns, got %d", containsEdges)
	}
}

// ── Phase 6: References edges (Foreign keys) ──────────────────────────────────

func TestBuild_ForeignKey(t *testing.T) {
	ordersTable := &schema.Table{
		Name:        "orders",
		Columns:     []schema.Column{makeColumn("user_id", "int")},
		ForeignKeys: []schema.ForeignKey{makeFK([]string{"user_id"}, "users", []string{"id"})},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	referencesEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindReferences)
	if referencesEdges != 1 {
		t.Errorf("wanted 1 references edge, got %d", referencesEdges)
	}

	fkEdge := findEdge(schemaGraph, "table:orders", "table:users", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:orders to table:users")
	}
	fkMetadata, ok := fkEdge.Metadata.(*graph.FKMetadata)
	if !ok {
		t.Fatalf("expected edge Metadata to be *FKMetadata, got %T", fkEdge.Metadata)
	}
	if len(fkMetadata.LocalColumns) != 1 || fkMetadata.LocalColumns[0] != "user_id" {
		t.Errorf("FKMetadata.LocalColumns: wanted [user_id], got %v", fkMetadata.LocalColumns)
	}
	if len(fkMetadata.ForeignColumns) != 1 || fkMetadata.ForeignColumns[0] != "id" {
		t.Errorf("FKMetadata.ForeignColumns: wanted [id], got %v", fkMetadata.ForeignColumns)
	}
}

func TestBuild_ForeignKeyActions(t *testing.T) {
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

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	fkEdge := findEdge(schemaGraph, "table:orders", "table:users", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:orders to table:users")
	}
	fkMetadata := fkEdge.Metadata.(*graph.FKMetadata)
	if fkMetadata.OnDelete != schema.FKCascade {
		t.Errorf("FKMetadata.OnDelete: wanted CASCADE, got %q", fkMetadata.OnDelete)
	}
	if fkMetadata.OnUpdate != schema.FKRestrict {
		t.Errorf("FKMetadata.OnUpdate: wanted RESTRICT, got %q", fkMetadata.OnUpdate)
	}
}

func TestBuild_CompositeForeignKey(t *testing.T) {
	// A composite FK maps (order_id, product_id) → order_items.(order_id, product_id).
	// It must produce exactly ONE edge, not two separate edges.
	orderItemsTable := &schema.Table{
		Name: "order_details",
		Columns: []schema.Column{
			makeColumn("order_id", "int"),
			makeColumn("product_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"order_id", "product_id"}, "order_items", []string{"order_id", "product_id"}),
		},
	}
	orderItemsParent := makeTable("order_items",
		makePKColumn("order_id", "int"),
		makePKColumn("product_id", "int"),
	)
	model := makeModel([]*schema.Table{orderItemsParent, orderItemsTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	referencesEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindReferences)
	if referencesEdges != 1 {
		t.Errorf("composite FK must produce exactly 1 references edge, got %d", referencesEdges)
	}

	fkEdge := findEdge(schemaGraph, "table:order_details", "table:order_items", graph.EdgeKindReferences)
	if fkEdge == nil {
		t.Fatal("expected references edge from table:order_details to table:order_items")
	}
	fkMetadata := fkEdge.Metadata.(*graph.FKMetadata)
	if len(fkMetadata.LocalColumns) != 2 {
		t.Errorf("composite FK edge must carry 2 local columns, got %d", len(fkMetadata.LocalColumns))
	}
}

func TestBuild_SelfReferencingForeignKey(t *testing.T) {
	// An employee table where manager_id references the same table's id column.
	employeesTable := &schema.Table{
		Name: "employees",
		Columns: []schema.Column{
			makePKColumn("id", "int"),
			makeColumn("manager_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"manager_id"}, "employees", []string{"id"}),
		},
	}
	model := makeModel([]*schema.Table{employeesTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error on self-referencing FK: %v", err)
	}

	selfRefEdge := findEdge(schemaGraph, "table:employees", "table:employees", graph.EdgeKindReferences)
	if selfRefEdge == nil {
		t.Error("expected self-referencing edge from table:employees to table:employees")
	}
	fkMetadata := selfRefEdge.Metadata.(*graph.FKMetadata)
	if len(fkMetadata.LocalColumns) != 1 || fkMetadata.LocalColumns[0] != "manager_id" {
		t.Errorf("self-ref FK: wanted local column [manager_id], got %v", fkMetadata.LocalColumns)
	}
}

func TestBuild_MultipleFKsBetweenSameTables(t *testing.T) {
	// Two distinct FK constraints between the same pair of tables must produce
	// two separate references edges (not be deduplicated).
	ordersTable := &schema.Table{
		Name: "orders",
		Columns: []schema.Column{
			makeColumn("user_id", "int"),
			makeColumn("billing_user_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
			makeFK([]string{"billing_user_id"}, "users", []string{"id"}),
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	referencesEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindReferences)
	if referencesEdges != 2 {
		t.Errorf("two distinct FK constraints must produce 2 references edges, got %d", referencesEdges)
	}
}

// ── Phase 7: Reverse reference edges (referenced_by, depends_on) ─────────────

func TestBuild_ReverseReferenceEdges(t *testing.T) {
	ordersTable := &schema.Table{
		Name:    "orders",
		Columns: []schema.Column{makeColumn("user_id", "int")},
		ForeignKeys: []schema.ForeignKey{makeFK([]string{"user_id"}, "users", []string{"id"})},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	referencedByEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindReferencedBy)
	if referencedByEdges != 1 {
		t.Errorf("wanted 1 referenced_by edge, got %d", referencedByEdges)
	}
	dependsOnEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindDependsOn)
	if dependsOnEdges != 1 {
		t.Errorf("wanted 1 depends_on edge, got %d", dependsOnEdges)
	}

	// referenced_by: parent → child (users → orders)
	refByEdge := findEdge(schemaGraph, "table:users", "table:orders", graph.EdgeKindReferencedBy)
	if refByEdge == nil {
		t.Error("expected referenced_by edge from table:users to table:orders")
	}
	refByMetadata, ok := refByEdge.Metadata.(*graph.FKMetadata)
	if !ok {
		t.Fatalf("referenced_by edge Metadata: expected *FKMetadata, got %T", refByEdge.Metadata)
	}
	if len(refByMetadata.LocalColumns) != 1 || refByMetadata.LocalColumns[0] != "user_id" {
		t.Errorf("referenced_by FKMetadata.LocalColumns: wanted [user_id], got %v", refByMetadata.LocalColumns)
	}

	// depends_on: child → parent (orders → users)
	depEdge := findEdge(schemaGraph, "table:orders", "table:users", graph.EdgeKindDependsOn)
	if depEdge == nil {
		t.Error("expected depends_on edge from table:orders to table:users")
	}
	depMetadata, ok := depEdge.Metadata.(*graph.FKMetadata)
	if !ok {
		t.Fatalf("depends_on edge Metadata: expected *FKMetadata, got %T", depEdge.Metadata)
	}
	if len(depMetadata.LocalColumns) != 1 || depMetadata.LocalColumns[0] != "user_id" {
		t.Errorf("depends_on FKMetadata.LocalColumns: wanted [user_id], got %v", depMetadata.LocalColumns)
	}
}

func TestBuild_ReverseEdgesMultipleFKs(t *testing.T) {
	// Two FKs must produce two referenced_by and two depends_on edges.
	ordersTable := &schema.Table{
		Name: "orders",
		Columns: []schema.Column{
			makeColumn("user_id", "int"),
			makeColumn("addr_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"user_id"}, "users", []string{"id"}),
			makeFK([]string{"addr_id"}, "addresses", []string{"id"}),
		},
	}
	usersTable := makeTable("users", makePKColumn("id", "int"))
	addrTable := makeTable("addresses", makePKColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable, addrTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	if got := countEdgesOfKind(schemaGraph, graph.EdgeKindReferencedBy); got != 2 {
		t.Errorf("wanted 2 referenced_by edges, got %d", got)
	}
	if got := countEdgesOfKind(schemaGraph, graph.EdgeKindDependsOn); got != 2 {
		t.Errorf("wanted 2 depends_on edges, got %d", got)
	}
}

func TestBuild_SelfRefReverseEdges(t *testing.T) {
	// A self-referencing FK must also produce referenced_by and depends_on edges.
	employeesTable := &schema.Table{
		Name: "employees",
		Columns: []schema.Column{
			makePKColumn("id", "int"),
			makeColumn("manager_id", "int"),
		},
		ForeignKeys: []schema.ForeignKey{
			makeFK([]string{"manager_id"}, "employees", []string{"id"}),
		},
	}
	model := makeModel([]*schema.Table{employeesTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	// Self-referencing: referenced_by and depends_on both have from == to.
	refByEdge := findEdge(schemaGraph, "table:employees", "table:employees", graph.EdgeKindReferencedBy)
	if refByEdge == nil {
		t.Error("expected self-referencing referenced_by edge on table:employees")
	}
	depEdge := findEdge(schemaGraph, "table:employees", "table:employees", graph.EdgeKindDependsOn)
	if depEdge == nil {
		t.Error("expected self-referencing depends_on edge on table:employees")
	}
}

// ── Phase 7: Enum usage edges ─────────────────────────────────────────────────

func TestBuild_EnumUsageEdge(t *testing.T) {
	userStatusEnum := schema.EnumType{
		Name:   "user_status",
		Values: []string{"active", "inactive"},
	}
	// The column's Type must exactly match the enum name for the edge to be created.
	usersTable := makeTable("users",
		makePKColumn("id", "int"),
		schema.Column{Name: "status", Type: "user_status", Nullable: false},
	)
	model := makeModel([]*schema.Table{usersTable}, []schema.EnumType{userStatusEnum})

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	usesEnumEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindUsesEnum)
	if usesEnumEdges != 1 {
		t.Errorf("wanted 1 uses_enum edge, got %d", usesEnumEdges)
	}

	enumEdge := findEdge(schemaGraph, "column:users.status", "enum:user_status", graph.EdgeKindUsesEnum)
	if enumEdge == nil {
		t.Error("expected uses_enum edge from column:users.status to enum:user_status")
	}
}

func TestBuild_NonEnumColumnProducesNoEnumEdge(t *testing.T) {
	// A column whose type does not match any enum must not produce a uses_enum edge.
	usersTable := makeTable("users", makeColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	usesEnumEdges := countEdgesOfKind(schemaGraph, graph.EdgeKindUsesEnum)
	if usesEnumEdges != 0 {
		t.Errorf("wanted 0 uses_enum edges for non-enum column, got %d", usesEnumEdges)
	}
}

// ── Total node and edge counts ────────────────────────────────────────────────

func TestBuild_TotalNodeCount(t *testing.T) {
	// 2 tables + 2 enums + 3 columns (2 in users, 1 in orders) = 7 nodes total.
	usersTable := makeTable("users", makePKColumn("id", "int"), makeColumn("name", "varchar"))
	ordersTable := makeTable("orders", makeColumn("user_id", "int"))
	enums := []schema.EnumType{
		{Name: "status_type", Values: []string{"a", "b"}},
		{Name: "priority", Values: []string{"low", "high"}},
	}
	model := makeModel([]*schema.Table{usersTable, ordersTable}, enums)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	wantNodeCount := 2 + 2 + 3 // tables + enums + columns
	if len(schemaGraph.NodeList) != wantNodeCount {
		t.Errorf("wanted %d total nodes, got %d", wantNodeCount, len(schemaGraph.NodeList))
	}
}

func TestBuild_TotalEdgeCount(t *testing.T) {
	// users: 2 columns → 2 contains edges
	// orders: 2 columns → 2 contains edges
	// orders.user_id → users: 1 references edge
	//                   1 referenced_by edge (users → orders)
	//                   1 depends_on edge (orders → users)
	// orders.status → status_type: 1 uses_enum edge
	// Total: 8 edges
	statusEnum := schema.EnumType{Name: "status_type", Values: []string{"open", "closed"}}
	usersTable := makeTable("users", makePKColumn("id", "int"), makeColumn("name", "varchar"))
	ordersTable := &schema.Table{
		Name: "orders",
		Columns: []schema.Column{
			makeColumn("user_id", "int"),
			schema.Column{Name: "status", Type: "status_type"},
		},
		ForeignKeys: []schema.ForeignKey{makeFK([]string{"user_id"}, "users", []string{"id"})},
	}
	model := makeModel([]*schema.Table{usersTable, ordersTable}, []schema.EnumType{statusEnum})

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	// Each FK produces 3 edges: references + referenced_by + depends_on.
	wantEdgeCount := 4 + 3 + 1 // contains + FK-related + uses_enum
	if len(schemaGraph.Edges) != wantEdgeCount {
		t.Errorf("wanted %d total edges, got %d", wantEdgeCount, len(schemaGraph.Edges))
	}
}

// ── Node ID uniqueness ────────────────────────────────────────────────────────

func TestBuild_AllNodeIDsAreUnique(t *testing.T) {
	// Deliberately use column names that could collide across tables
	// if ID generation were naive (e.g. both tables have a column named "id").
	usersTable := makeTable("users", makePKColumn("id", "int"), makeColumn("name", "varchar"))
	ordersTable := makeTable("orders", makePKColumn("id", "int"), makeColumn("total", "numeric"))
	model := makeModel([]*schema.Table{usersTable, ordersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	// The Nodes map uses IDs as keys, so len(Nodes) == len(NodeList) guarantees uniqueness.
	if len(schemaGraph.Nodes) != len(schemaGraph.NodeList) {
		t.Errorf(
			"duplicate node IDs detected: Nodes map has %d entries but NodeList has %d",
			len(schemaGraph.Nodes), len(schemaGraph.NodeList),
		)
	}
}

// ── Determinism ───────────────────────────────────────────────────────────────

func TestBuild_Determinism(t *testing.T) {
	// Building the same schema twice must produce identical graphs.
	buildModel := func() *schema.Model {
		statusEnum := schema.EnumType{Name: "status", Values: []string{"a", "b"}}
		usersTable := &schema.Table{
			Name:       "users",
			Columns:    []schema.Column{makePKColumn("id", "int"), makeColumn("status", "status")},
			PrimaryKey: []string{"id"},
		}
		ordersTable := &schema.Table{
			Name:        "orders",
			Columns:     []schema.Column{makePKColumn("id", "int"), makeColumn("user_id", "int")},
			PrimaryKey:  []string{"id"},
			ForeignKeys: []schema.ForeignKey{makeFK([]string{"user_id"}, "users", []string{"id"})},
		}
		return makeModel([]*schema.Table{usersTable, ordersTable}, []schema.EnumType{statusEnum})
	}

	firstGraph, err := graph.Build(buildModel())
	if err != nil {
		t.Fatalf("first Build() returned unexpected error: %v", err)
	}
	secondGraph, err := graph.Build(buildModel())
	if err != nil {
		t.Fatalf("second Build() returned unexpected error: %v", err)
	}

	if len(firstGraph.NodeList) != len(secondGraph.NodeList) {
		t.Errorf("node counts differ: %d vs %d", len(firstGraph.NodeList), len(secondGraph.NodeList))
	}
	for index, firstNode := range firstGraph.NodeList {
		if index >= len(secondGraph.NodeList) {
			break
		}
		secondNode := secondGraph.NodeList[index]
		if firstNode.ID != secondNode.ID || firstNode.Kind != secondNode.Kind || firstNode.Label != secondNode.Label {
			t.Errorf("node at position %d differs: first=%+v second=%+v", index, firstNode, secondNode)
		}
	}

	if len(firstGraph.Edges) != len(secondGraph.Edges) {
		t.Errorf("edge counts differ: %d vs %d", len(firstGraph.Edges), len(secondGraph.Edges))
	}
	for index, firstEdge := range firstGraph.Edges {
		if index >= len(secondGraph.Edges) {
			break
		}
		secondEdge := secondGraph.Edges[index]
		if firstEdge.From != secondEdge.From || firstEdge.To != secondEdge.To || firstEdge.Kind != secondEdge.Kind {
			t.Errorf("edge at position %d differs: first=%+v second=%+v", index, firstEdge, secondEdge)
		}
	}
}

// ── Node ordering ─────────────────────────────────────────────────────────────

func TestBuild_NodeOrder(t *testing.T) {
	// Tables come before enums, enums before columns, in schema order.
	statusEnum := schema.EnumType{Name: "status", Values: []string{"a"}}
	usersTable := makeTable("users", makeColumn("id", "int"))
	model := makeModel([]*schema.Table{usersTable}, []schema.EnumType{statusEnum})

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	// Expected order: table:users, enum:status, column:users.id
	expectedOrder := []string{"table:users", "enum:status", "column:users.id"}
	if len(schemaGraph.NodeList) != len(expectedOrder) {
		t.Fatalf("wanted %d nodes, got %d", len(expectedOrder), len(schemaGraph.NodeList))
	}
	for index, expectedID := range expectedOrder {
		if schemaGraph.NodeList[index].ID != expectedID {
			t.Errorf("node at position %d: wanted %q, got %q",
				index, expectedID, schemaGraph.NodeList[index].ID)
		}
	}
}
