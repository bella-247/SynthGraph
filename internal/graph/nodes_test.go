package graph_test

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

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
