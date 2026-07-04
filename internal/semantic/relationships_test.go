package semantic

import (
	"testing"

	"synthgraph/internal/schema"
)

// ── Temporal, audit, and relationship tests ───────────────────────────────

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
