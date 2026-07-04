package semantic

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// ── Integration and unit tests ────────────────────────────────────────────

func TestBuild_FullyPopulatedNode(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "audit_log",
				Columns: []schema.Column{
					{Name: "id", Type: "bigint", IsPrimaryKey: true},
					{Name: "created_at", Type: "timestamp"},
					{Name: "updated_at", Type: "timestamp"},
					{Name: "deleted_at", Type: "timestamp"},
					{Name: "created_by", Type: "int"},
					{Name: "updated_by", Type: "int"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	g := setupGraph(t, model)
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	node := sg.Nodes["table:audit_log"]
	if node == nil {
		t.Fatal("audit_log node not found")
	}

	// Temporal pattern
	if node.Temporal == nil {
		t.Fatal("expected temporal pattern")
	}
	if !node.Temporal.HasCreatedAt {
		t.Error("expected HasCreatedAt")
	}
	if !node.Temporal.HasUpdatedAt {
		t.Error("expected HasUpdatedAt")
	}
	if !node.Temporal.HasDeletedAt {
		t.Error("expected HasDeletedAt")
	}
	if !node.IsSoftDelete {
		t.Error("expected IsSoftDelete")
	}

	// Audit pattern
	if node.Audit == nil {
		t.Fatal("expected audit pattern")
	}
	if !node.Audit.HasCreatedBy {
		t.Error("expected HasCreatedBy")
	}
	if !node.Audit.HasUpdatedBy {
		t.Error("expected HasUpdatedBy")
	}

	// Should have Entity role at minimum.
	if !node.HasRole(TableRoleEntity) {
		t.Errorf("expected entity role, got %v", node.Roles)
	}

	// All inferences should be present in node.Inferences.
	expectedKinds := map[string]bool{
		"entity":      false,
		"temporal":    false,
		"soft_delete": false,
		"audit":       false,
	}
	for _, inf := range node.Inferences {
		if _, ok := expectedKinds[inf.Kind]; ok {
			expectedKinds[inf.Kind] = true
		}
	}
	for kind, found := range expectedKinds {
		if !found {
			t.Errorf("missing inference kind %q on fully-populated node", kind)
		}
	}
}

// TestNewInferenceContext verifies that the precomputed indexes are populated
// correctly from the source graph.
func TestNewInferenceContext(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "posts",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int", Nullable: false},
					{Name: "created_at", Type: "timestamp"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "comments",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "post_id", Type: "int", Nullable: false},
					{Name: "user_id", Type: "int", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"post_id"}, RefTable: "posts", RefColumns: []string{"id"}},
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	context := newInferenceContext(g)

	// OutgoingForeignKeyCount
	if context.OutgoingForeignKeyCount["table:users"] != 0 {
		t.Errorf("users outgoing FKs: want 0, got %d", context.OutgoingForeignKeyCount["table:users"])
	}
	if context.OutgoingForeignKeyCount["table:posts"] != 1 {
		t.Errorf("posts outgoing FKs: want 1, got %d", context.OutgoingForeignKeyCount["table:posts"])
	}
	if context.OutgoingForeignKeyCount["table:comments"] != 2 {
		t.Errorf("comments outgoing FKs: want 2, got %d", context.OutgoingForeignKeyCount["table:comments"])
	}

	// IncomingForeignKeyCount
	if context.IncomingForeignKeyCount["table:users"] != 2 {
		t.Errorf("users incoming FKs: want 2, got %d", context.IncomingForeignKeyCount["table:users"])
	}
	if context.IncomingForeignKeyCount["table:posts"] != 1 {
		t.Errorf("posts incoming FKs: want 1, got %d", context.IncomingForeignKeyCount["table:posts"])
	}

	// ColumnCount
	if context.ColumnCount["table:users"] != 1 {
		t.Errorf("users column count: want 1, got %d", context.ColumnCount["table:users"])
	}
	if context.ColumnCount["table:posts"] != 3 {
		t.Errorf("posts column count: want 3, got %d", context.ColumnCount["table:posts"])
	}

	// ForeignKeyColumnIndex
	postFKColumns := context.ForeignKeyColumnIndex["table:posts"]
	if postFKColumns == nil || !postFKColumns["user_id"] {
		t.Error("posts should have user_id in FK column index")
	}

	commentsFKColumns := context.ForeignKeyColumnIndex["table:comments"]
	if commentsFKColumns == nil {
		t.Fatal("comments should have FK column index")
	}
	if !commentsFKColumns["post_id"] {
		t.Error("comments should have post_id in FK column index")
	}
	if !commentsFKColumns["user_id"] {
		t.Error("comments should have user_id in FK column index")
	}

	// TemporalPattern
	postTemporal := context.TemporalPattern["table:posts"]
	if !postTemporal.HasCreatedAt {
		t.Error("posts should have HasCreatedAt")
	}

	// SelfRefCount
	if context.SelfRefCount["table:users"] != 0 {
		t.Error("users should have 0 self-refs")
	}
}

func TestHasJunctionSignal(t *testing.T) {
	tests := []struct {
		name              string
		primaryKey        []string
		foreignKeyColumns map[string]bool
		expected          bool
	}{
		{
			name:              "composite PK all FKs",
			primaryKey:        []string{"product_id", "category_id"},
			foreignKeyColumns: map[string]bool{"product_id": true, "category_id": true},
			expected:          true,
		},
		{
			name:              "composite PK not all FKs",
			primaryKey:        []string{"id", "user_id"},
			foreignKeyColumns: map[string]bool{"user_id": true},
			expected:          false,
		},
		{
			name:              "single column PK (even if FK)",
			primaryKey:        []string{"user_id"},
			foreignKeyColumns: map[string]bool{"user_id": true},
			expected:          false, // need at least 2 cols
		},
		{
			name:              "empty PK",
			primaryKey:        []string{},
			foreignKeyColumns: map[string]bool{},
			expected:          false,
		},
		{
			name:              "nil FK index",
			primaryKey:        []string{"a_id", "b_id"},
			foreignKeyColumns: nil,
			expected:          false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tableData := graph.TableData{PrimaryKey: test.primaryKey}
			got := hasJunctionSignal(tableData, test.foreignKeyColumns)
			if got != test.expected {
				t.Errorf("hasJunctionSignal(%v, %v) = %v, want %v",
					test.primaryKey, test.foreignKeyColumns, got, test.expected)
			}
		})
	}
}
