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
