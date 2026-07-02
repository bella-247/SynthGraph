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

// ── Phase 2: Table nodes ──────────────────────────────────────────────────────

func TestBuild_TableNodes(t *testing.T) {
	tests := []struct {
		name           string
		tableNames     []string
		wantTableCount int
	}{
		{
			name:           "single table",
			tableNames:     []string{"users"},
			wantTableCount: 1,
		},
		{
			name:           "multiple tables",
			tableNames:     []string{"users", "orders", "products"},
			wantTableCount: 3,
		},
		{
			name:           "schema-qualified table name",
			tableNames:     []string{"public.users"},
			wantTableCount: 1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			tables := make([]*schema.Table, len(testCase.tableNames))
			for index, name := range testCase.tableNames {
				tables[index] = makeTable(name)
			}
			model := makeModel(tables, nil)

			schemaGraph, err := graph.Build(model)
			if err != nil {
				t.Fatalf("Build() returned unexpected error: %v", err)
			}

			tableNodeCount := 0
			for _, node := range schemaGraph.NodeList {
				if node.Kind == graph.NodeKindTable {
					tableNodeCount++
				}
			}
			if tableNodeCount != testCase.wantTableCount {
				t.Errorf("wanted %d table nodes, got %d", testCase.wantTableCount, tableNodeCount)
			}

			for _, tableName := range testCase.tableNames {
				expectedNodeID := "table:" + tableName
				if !schemaGraph.HasNode(expectedNodeID) {
					t.Errorf("expected node %q to exist in graph", expectedNodeID)
				}
			}
		})
	}
}

func TestBuild_TableNodeKindAndLabel(t *testing.T) {
	model := makeModel([]*schema.Table{makeTable("users")}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	tableNode := schemaGraph.Nodes["table:users"]
	if tableNode == nil {
		t.Fatal("expected node table:users to exist")
	}
	if tableNode.Kind != graph.NodeKindTable {
		t.Errorf("wanted Kind=NodeKindTable, got %q", tableNode.Kind)
	}
	if tableNode.Label != "users" {
		t.Errorf("wanted Label=%q, got %q", "users", tableNode.Label)
	}
}

func TestBuild_TableDataPreservation(t *testing.T) {
	usersTable := &schema.Table{
		Name:       "users",
		PrimaryKey: []string{"id"},
		Unique:     [][]string{{"email"}},
		Checks: []schema.CheckConstraint{
			{Name: "chk_age", Expression: "age > 0"},
		},
	}
	model := makeModel([]*schema.Table{usersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	tableNode := schemaGraph.Nodes["table:users"]
	if tableNode == nil {
		t.Fatal("expected node table:users to exist")
	}

	tableData, ok := tableNode.Data.(graph.TableData)
	if !ok {
		t.Fatalf("expected Data to be TableData, got %T", tableNode.Data)
	}
	if tableData.Name != "users" {
		t.Errorf("TableData.Name: wanted %q, got %q", "users", tableData.Name)
	}
	if len(tableData.PrimaryKey) != 1 || tableData.PrimaryKey[0] != "id" {
		t.Errorf("TableData.PrimaryKey: wanted [id], got %v", tableData.PrimaryKey)
	}
	if len(tableData.Unique) != 1 || tableData.Unique[0][0] != "email" {
		t.Errorf("TableData.Unique: wanted [[email]], got %v", tableData.Unique)
	}
	if len(tableData.Checks) != 1 || tableData.Checks[0].Expression != "age > 0" {
		t.Errorf("TableData.Checks: wanted one check with expression 'age > 0', got %v", tableData.Checks)
	}
}

// ── Phase 3: Enum nodes ───────────────────────────────────────────────────────

func TestBuild_EnumNodes(t *testing.T) {
	enums := []schema.EnumType{
		{Name: "user_status", Values: []string{"active", "inactive", "banned"}},
		{Name: "order_state", Values: []string{"pending", "shipped", "delivered"}},
	}
	model := makeModel(nil, enums)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	if !schemaGraph.HasNode("enum:user_status") {
		t.Error("expected node enum:user_status to exist")
	}
	if !schemaGraph.HasNode("enum:order_state") {
		t.Error("expected node enum:order_state to exist")
	}

	enumNode := schemaGraph.Nodes["enum:user_status"]
	if enumNode.Kind != graph.NodeKindEnum {
		t.Errorf("wanted Kind=NodeKindEnum, got %q", enumNode.Kind)
	}
	enumData, ok := enumNode.Data.(graph.EnumData)
	if !ok {
		t.Fatalf("expected Data to be EnumData, got %T", enumNode.Data)
	}
	if len(enumData.Values) != 3 || enumData.Values[0] != "active" {
		t.Errorf("EnumData.Values: wanted [active inactive banned], got %v", enumData.Values)
	}
}

// ── Phase 4: Column nodes ─────────────────────────────────────────────────────

func TestBuild_ColumnNodes(t *testing.T) {
	usersTable := makeTable("users",
		makePKColumn("id", "int"),
		makeColumn("email", "varchar"),
		makeColumn("age", "int"),
	)
	model := makeModel([]*schema.Table{usersTable}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	columnIDs := []string{"column:users.id", "column:users.email", "column:users.age"}
	for _, columnID := range columnIDs {
		if !schemaGraph.HasNode(columnID) {
			t.Errorf("expected column node %q to exist", columnID)
		}
		node := schemaGraph.Nodes[columnID]
		if node.Kind != graph.NodeKindColumn {
			t.Errorf("node %q: wanted Kind=NodeKindColumn, got %q", columnID, node.Kind)
		}
	}
}

func TestBuild_ColumnDataPreservation(t *testing.T) {
	defaultValue := "now()"
	column := schema.Column{
		Name:         "created_at",
		Type:         "timestamp",
		Length:       0,
		Precision:    0,
		Nullable:     true,
		Default:      &defaultValue,
		IsPrimaryKey: false,
	}
	model := makeModel([]*schema.Table{makeTable("events", column)}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	columnNode := schemaGraph.Nodes["column:events.created_at"]
	if columnNode == nil {
		t.Fatal("expected node column:events.created_at to exist")
	}
	columnData, ok := columnNode.Data.(graph.ColumnData)
	if !ok {
		t.Fatalf("expected Data to be ColumnData, got %T", columnNode.Data)
	}
	if columnData.Type != "timestamp" {
		t.Errorf("ColumnData.Type: wanted %q, got %q", "timestamp", columnData.Type)
	}
	if !columnData.Nullable {
		t.Error("ColumnData.Nullable: wanted true")
	}
	if columnData.Default == nil || *columnData.Default != "now()" {
		t.Errorf("ColumnData.Default: wanted %q, got %v", "now()", columnData.Default)
	}
	if columnData.IsPrimaryKey {
		t.Error("ColumnData.IsPrimaryKey: wanted false")
	}
}

func TestBuild_PKColumnDataPreservation(t *testing.T) {
	model := makeModel([]*schema.Table{makeTable("users", makePKColumn("id", "int"))}, nil)

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	columnNode := schemaGraph.Nodes["column:users.id"]
	if columnNode == nil {
		t.Fatal("expected node column:users.id to exist")
	}
	columnData := columnNode.Data.(graph.ColumnData)
	if !columnData.IsPrimaryKey {
		t.Error("ColumnData.IsPrimaryKey: wanted true for PK column")
	}
}

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
