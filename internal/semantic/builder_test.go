package semantic

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// setupGraph is a helper that wraps the graph layer's builder so we don't
// have to manually construct graph.Graph and all its edges for testing.
// It allows testing semantic inference directly against the real output
// of the graph layer.
func setupGraph(t *testing.T, model *schema.Model) *graph.Graph {
	g, err := graph.Build(model)
	if err != nil {
		t.Fatalf("Failed to build graph: %v", err)
	}
	return g
}

func TestBuild_JunctionRole(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "roles",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "user_roles",
				Columns: []schema.Column{
					{Name: "user_id", Type: "uuid", IsPrimaryKey: true},
					{Name: "role_id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"user_id", "role_id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
					{Columns: []string{"role_id"}, RefTable: "roles", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	userRolesNode := sg.Nodes["table:user_roles"]
	if userRolesNode == nil {
		t.Fatalf("table:user_roles not found")
	}

	if !userRolesNode.HasRole(TableRoleJunction) {
		t.Errorf("expected user_roles to have junction role, got roles: %v", userRolesNode.Roles)
	}
	if !userRolesNode.HasRole(TableRoleEntity) {
		t.Errorf("expected user_roles to have entity role (default), got roles: %v", userRolesNode.Roles)
	}
}

func TestBuild_HierarchyRole(t *testing.T) {
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
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	employeesNode := sg.Nodes["table:employees"]
	if !employeesNode.HasRole(TableRoleHierarchical) {
		t.Errorf("expected employees to have hierarchical role")
	}
	if !employeesNode.IsHierarchical {
		t.Errorf("expected employees.IsHierarchical to be true")
	}
}

func TestBuild_LookupRole(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "statuses",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "status_id", Type: "int"},
					{Name: "prev_status_id", Type: "int"},
					{Name: "next_status_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"status_id"}, RefTable: "statuses", RefColumns: []string{"id"}},
					{Columns: []string{"prev_status_id"}, RefTable: "statuses", RefColumns: []string{"id"}}, // simulate high usage
					{Columns: []string{"next_status_id"}, RefTable: "statuses", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	statusesNode := sg.Nodes["table:statuses"]
	if !statusesNode.HasRole(TableRoleLookup) {
		t.Errorf("expected statuses to have lookup role")
	}
}

func TestBuild_TemporalAndAudit(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "articles",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "created_at", Type: "timestamp"},
					{Name: "deleted_at", Type: "timestamp"},
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

	articlesNode := sg.Nodes["table:articles"]
	if articlesNode.Temporal == nil {
		t.Fatalf("expected temporal pattern to be populated")
	}
	if !articlesNode.Temporal.HasCreatedAt {
		t.Errorf("expected HasCreatedAt=true")
	}
	if !articlesNode.Temporal.HasDeletedAt {
		t.Errorf("expected HasDeletedAt=true")
	}
	if articlesNode.Temporal.HasUpdatedAt {
		t.Errorf("expected HasUpdatedAt=false")
	}
	if !articlesNode.IsSoftDelete {
		t.Errorf("expected IsSoftDelete=true")
	}

	if articlesNode.Audit == nil {
		t.Fatalf("expected audit pattern to be populated")
	}
	if !articlesNode.Audit.HasUpdatedBy {
		t.Errorf("expected HasUpdatedBy=true")
	}
}

func TestBuild_Relationships(t *testing.T) {
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
				Name: "profiles",
				Columns: []schema.Column{
					{Name: "user_id", Type: "int", IsPrimaryKey: true, Nullable: false}, // 1:1, required
				},
				PrimaryKey: []string{"user_id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}, OnDelete: schema.FKCascade}, // composition
				},
			},
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int", Nullable: false}, // 1:N, required, no cascade
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}, // association
				},
			},
			{
				Name: "posts",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int", Nullable: true}, // 1:N, optional
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}, // aggregation
				},
			},
		},
	}

	g := setupGraph(t, model)
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	relMap := make(map[string]RelationshipKind)
	for _, rel := range sg.Relationships {
		key := rel.From + "->" + rel.To
		relMap[key] = rel.Kind
	}

	if kind, ok := relMap["table:profiles->table:users"]; !ok || kind != RelationshipKindComposition {
		t.Errorf("expected profiles->users to be composition, got %v", kind)
	}
	if kind, ok := relMap["table:orders->table:users"]; !ok || kind != RelationshipKindAssociation {
		t.Errorf("expected orders->users to be association, got %v", kind)
	}
	if kind, ok := relMap["table:posts->table:users"]; !ok || kind != RelationshipKindAggregation {
		t.Errorf("expected posts->users to be aggregation, got %v", kind)
	}
}

func TestBuild_ManyToManyRelationship(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "authors",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "books",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "authors_books",
				Columns: []schema.Column{
					{Name: "author_id", Type: "int", IsPrimaryKey: true},
					{Name: "book_id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"author_id", "book_id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"author_id"}, RefTable: "authors", RefColumns: []string{"id"}},
					{Columns: []string{"book_id"}, RefTable: "books", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// The junction table authors_books creates FK edges with many-to-many
	// cardinality (both sides of the junction are FKs).
	for _, rel := range sg.Relationships {
		if rel.Kind == RelationshipKindManyToMany {
			return // Found at least one many-to-many relationship.
		}
	}
	t.Errorf("expected at least one many_to_many relationship, found none")
}

func TestBuild_HierarchyRelationship(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "categories",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "parent_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"parent_id"}, RefTable: "categories", RefColumns: []string{"id"}},
				},
			},
		},
	}

	g := setupGraph(t, model)
	sg, err := Build(g)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	foundHierarchy := false
	for _, rel := range sg.Relationships {
		if rel.Kind == RelationshipKindHierarchy {
			foundHierarchy = true
			break
		}
	}
	if !foundHierarchy {
		t.Errorf("expected at least one hierarchy relationship for self-referencing FK")
	}
}

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
