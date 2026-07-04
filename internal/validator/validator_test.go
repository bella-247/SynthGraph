package validator

import (
	"testing"

	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// setupTest builds a complete pipeline for a schema model and returns the
// generated dataset plus the model and graph needed by the validator.
func setupTest(t *testing.T, model *schema.Model) *generator.Dataset {
	t.Helper()

	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("graph.Build: %v", err)
	}

	semanticGraph, err := semantic.Build(schemaGraph)
	if err != nil {
		t.Fatalf("semantic.Build: %v", err)
	}

	genPlan, err := planner.BuildPlan(schemaGraph, model, 10)
	if err != nil {
		t.Fatalf("planner.BuildPlan: %v", err)
	}

	ctx := &generator.GenerationContext{
		GlobalSeed:    42,
		Model:         model,
		Graph:         schemaGraph,
		SemanticGraph: semanticGraph,
	}

	dataset, err := generator.Generate(genPlan, ctx)
	if err != nil {
		t.Fatalf("generator.Generate: %v", err)
	}

	return dataset
}

func buildDataset(model *schema.Model) *generator.Dataset {
	schemaGraph, _ := graph.Build(model)
	semanticGraph, _ := semantic.Build(schemaGraph)
	genPlan, _ := planner.BuildPlan(schemaGraph, model, 10)
	ctx := &generator.GenerationContext{
		GlobalSeed:    42,
		Model:         model,
		Graph:         schemaGraph,
		SemanticGraph: semanticGraph,
	}
	dataset, _ := generator.Generate(genPlan, ctx)
	return dataset
}

func TestValidate_EmptySchema(t *testing.T) {
	model := &schema.Model{}
	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	result := Validate(dataset, model, schemaGraph)
	if !result.Valid {
		t.Errorf("expected valid, got %d errors", len(result.Errors))
	}
}

func TestValidate_SingleTable(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
					{Name: "email", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)
	result := Validate(dataset, model, schemaGraph)

	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidate_NotNullViolation(t *testing.T) {
	// Build a dataset, then manually inject a nil into a NOT NULL column.
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "label", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Corrupt row 0's label.
	dataset.Tables[0].Rows[0]["label"] = nil

	result := Validate(dataset, model, schemaGraph)
	if result.Valid {
		t.Fatal("expected invalid due to NOT NULL violation")
	}

	found := false
	for _, err := range result.Errors {
		if err.Constraint == "NOT NULL" && err.Column == "label" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NOT NULL violation for label, got errors: %v", result.Errors)
	}
}

func TestValidate_NullableColumn(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "nickname", Type: "varchar", Nullable: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Set nickname to nil — should be allowed since it's nullable.
	dataset.Tables[0].Rows[0]["nickname"] = nil

	result := Validate(dataset, model, schemaGraph)
	if !result.Valid {
		t.Errorf("expected valid (nullable column), got: %v", result.Errors)
	}
}

func TestValidate_PrimaryKeyViolation(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "val", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Corrupt: duplicate PK value.
	dataset.Tables[0].Rows[1]["id"] = dataset.Tables[0].Rows[0]["id"]

	result := Validate(dataset, model, schemaGraph)
	if result.Valid {
		t.Fatal("expected invalid due to PK violation")
	}

	found := false
	for _, err := range result.Errors {
		if err.Constraint == "PRIMARY KEY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PRIMARY KEY violation, got errors: %v", result.Errors)
	}
}

func TestValidate_PrimaryKeyNilViolation(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "val", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Set PK to nil.
	dataset.Tables[0].Rows[0]["id"] = nil

	result := Validate(dataset, model, schemaGraph)
	if result.Valid {
		t.Fatal("expected invalid due to nil PK")
	}

	found := false
	for _, err := range result.Errors {
		if err.Constraint == "PRIMARY KEY" && err.Column == "id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PRIMARY KEY nil violation for id, got: %v", result.Errors)
	}
}

func TestValidate_UniqueConstraint(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "email", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
				Unique:     [][]string{{"email"}},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Corrupt: duplicate email.
	dataset.Tables[0].Rows[1]["email"] = dataset.Tables[0].Rows[0]["email"]

	result := Validate(dataset, model, schemaGraph)
	if result.Valid {
		t.Fatal("expected invalid due to unique constraint violation")
	}

	found := false
	for _, err := range result.Errors {
		if err.Constraint == "UNIQUE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNIQUE violation, got errors: %v", result.Errors)
	}
}

func TestValidate_ForeignKey(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
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
					{Name: "user_id", Type: "int"},
					{Name: "total", Type: "decimal"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	result := Validate(dataset, model, schemaGraph)
	if !result.Valid {
		t.Errorf("expected valid FK references, got errors: %v", result.Errors)
	}
}

func TestValidate_ForeignKeyViolation(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
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
					{Name: "user_id", Type: "int"},
					{Name: "total", Type: "decimal"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Corrupt: set user_id to a value that doesn't exist.
	dataset.Tables[1].Rows[0]["user_id"] = int64(999999)

	result := Validate(dataset, model, schemaGraph)
	if result.Valid {
		t.Fatal("expected invalid due to FK violation")
	}

	found := false
	for _, err := range result.Errors {
		if err.Constraint == "FOREIGN KEY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected FOREIGN KEY violation, got: %v", result.Errors)
	}
}

func TestValidate_CycleResolution(t *testing.T) {
	// a ←→ b with nullable FK on b → a.
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "b_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"b_id"}, RefTable: "b", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "b",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "a_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
				},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	result := Validate(dataset, model, schemaGraph)
	if !result.Valid {
		t.Errorf("expected valid cycle-resolved dataset, got: %v", result.Errors)
	}
}

func TestValidate_MultipleViolations(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "email", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
				Unique:     [][]string{{"email"}},
			},
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int"},
					{Name: "total", Type: "decimal"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}

	dataset := buildDataset(model)
	schemaGraph, _ := graph.Build(model)

	// Multiple violations.
	dataset.Tables[0].Rows[1]["email"] = dataset.Tables[0].Rows[0]["email"] // duplicate email
	dataset.Tables[1].Rows[0]["user_id"] = int64(999999)                     // bad FK

	result := Validate(dataset, model, schemaGraph)
	if result.Valid {
		t.Fatal("expected invalid due to multiple violations")
	}

	uniqueFound := false
	fkFound := false
	for _, err := range result.Errors {
		switch err.Constraint {
		case "UNIQUE":
			uniqueFound = true
		case "FOREIGN KEY":
			fkFound = true
		}
	}
	if !uniqueFound {
		t.Error("expected UNIQUE violation")
	}
	if !fkFound {
		t.Error("expected FOREIGN KEY violation")
	}
}
