package validator

import (
	"strings"
	"testing"

	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildTableMap(model *schema.Model) {
	model.TableMap = make(map[string]*schema.Table, len(model.Tables))
	for _, t := range model.Tables {
		model.TableMap[t.Name] = t
	}
}

func makeSimpleModel() *schema.Model {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", Nullable: false, IsPrimaryKey: true},
					{Name: "name", Type: "varchar", Nullable: false, Length: 50},
					{Name: "email", Type: "varchar", Nullable: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	buildTableMap(model)
	return model
}

func makeSimpleDataset(model *schema.Model) *generator.Dataset {
	t := model.Tables[0]
	return &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: t.Name,
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": "Alice", "email": "alice@example.com"},
					{"id": int64(2), "name": "Bob", "email": "bob@example.com"},
				},
			},
		},
	}
}

// ── NOT NULL tests ──────────────────────────────────────────────────────────

func TestValidateNotNull_OK(t *testing.T) {
	model := makeSimpleModel()
	dataset := makeSimpleDataset(model)

	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateNotNull_NilValue(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": nil, "email": "alice@example.com"},
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected NOT_NULL error, got none")
	}
	found := false
	for _, e := range errs {
		if e.Rule == "NOT_NULL" && e.Column == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NOT_NULL error for column 'name', got: %v", errs)
	}
}

func TestValidateNotNull_NullableColumn_OK(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": "Alice", "email": nil},
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors for nullable nil column, got: %v", errs)
	}
}

func TestValidateNotNull_MissingColumn(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1)}, // "name" is missing entirely
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected NOT_NULL error for missing column, got none")
	}
}

// ── PK uniqueness tests ────────────────────────────────────────────────────

func TestValidatePKUnique_Duplicate(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": "Alice"},
					{"id": int64(1), "name": "Bob"}, // duplicate PK
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected PK_UNIQUE error, got none")
	}
	found := false
	for _, e := range errs {
		if e.Rule == "PK_UNIQUE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PK_UNIQUE error, got: %v", errs)
	}
}

func TestValidatePKUnique_CompositeKey(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "order_items",
				Columns: []schema.Column{
					{Name: "order_id", Type: "int", Nullable: false},
					{Name: "product_id", Type: "int", Nullable: false},
					{Name: "qty", Type: "int"},
				},
				PrimaryKey: []string{"order_id", "product_id"},
			},
		},
	}
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "order_items",
				Rows: []generator.GeneratedRow{
					{"order_id": int64(1), "product_id": int64(1), "qty": 2},
					{"order_id": int64(1), "product_id": int64(1), "qty": 5}, // duplicate composite PK
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected PK_UNIQUE error for composite key, got none")
	}
}

// ── UNIQUE constraint tests ────────────────────────────────────────────────

func TestValidateUniqueConstraint_Duplicate(t *testing.T) {
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
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "email": "same@example.com"},
					{"id": int64(2), "email": "same@example.com"}, // duplicate unique value
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected UNIQUE error, got none")
	}
	found := false
	for _, e := range errs {
		if e.Rule == "UNIQUE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNIQUE error, got: %v", errs)
	}
}

func TestValidateUniqueConstraint_Composite(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "follows",
				Columns: []schema.Column{
					{Name: "follower_id", Type: "int"},
					{Name: "followee_id", Type: "int"},
				},
				Unique: [][]string{{"follower_id", "followee_id"}},
			},
		},
	}
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "follows",
				Rows: []generator.GeneratedRow{
					{"follower_id": int64(1), "followee_id": int64(2)},
					{"follower_id": int64(1), "followee_id": int64(2)}, // duplicate composite unique
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected UNIQUE error for composite, got none")
	}
}

// ── Enum tests ─────────────────────────────────────────────────────────────

func TestValidateEnum_Invalid(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "status", Type: "order_status"},
				},
				PrimaryKey: []string{"id"},
			},
		},
		Enums: []schema.EnumType{
			{Name: "order_status", Values: []string{"pending", "shipped", "delivered"}},
		},
	}
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "orders",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "status": "cancelled"}, // invalid enum value
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected ENUM error, got none")
	}
	found := false
	for _, e := range errs {
		if e.Rule == "ENUM" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ENUM error, got: %v", errs)
	}
}

func TestValidateEnum_Valid(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "status", Type: "order_status"},
				},
				PrimaryKey: []string{"id"},
			},
		},
		Enums: []schema.EnumType{
			{Name: "order_status", Values: []string{"pending", "shipped", "delivered"}},
		},
	}
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "orders",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "status": "shipped"}, // valid
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// ── Length constraint tests ────────────────────────────────────────────────

func TestValidateLength_Exceeded(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": strings.Repeat("A", 51)}, // exceeds varchar(50)
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected LENGTH error, got none")
	}
	found := false
	for _, e := range errs {
		if e.Rule == "LENGTH" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LENGTH error, got: %v", errs)
	}
}

func TestValidateLength_WithinLimit(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": "Alice"}, // within varchar(50)
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// ── FK referential integrity tests ─────────────────────────────────────────

func TestValidateFK_Orphaned(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int"},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": "Alice"},
					{"id": int64(2), "name": "Bob"},
				},
			},
			{
				TableName: "orders",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "user_id": int64(99)}, // 99 is not a valid user PK
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs == nil {
		t.Fatal("expected FK error, got none")
	}
	found := false
	for _, e := range errs {
		if e.Rule == "FK" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected FK error, got: %v", errs)
	}
}

func TestValidateFK_Valid(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "orders",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int", Nullable: true},
				},
				PrimaryKey: []string{"id"},
				ForeignKeys: []schema.ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	buildTableMap(model)
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{
				TableName: "users",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "name": "Alice"},
					{"id": int64(2), "name": "Bob"},
				},
			},
			{
				TableName: "orders",
				Rows: []generator.GeneratedRow{
					{"id": int64(1), "user_id": int64(1)}, // references Alice
					{"id": int64(2), "user_id": nil},      // nullable FK with NULL is OK
				},
			},
		},
	}

	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// ── Edge cases ─────────────────────────────────────────────────────────────

func TestValidate_NilDataset(t *testing.T) {
	errs := Validate(nil, makeSimpleModel())
	if errs == nil {
		t.Fatal("expected error for nil dataset")
	}
}

func TestValidate_NilModel(t *testing.T) {
	errs := Validate(makeSimpleDataset(makeSimpleModel()), nil)
	if errs == nil {
		t.Fatal("expected error for nil model")
	}
}

func TestValidate_EmptyDataset(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{Tables: nil}
	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors for empty dataset, got: %v", errs)
	}
}

func TestValidate_EmptyRows(t *testing.T) {
	model := makeSimpleModel()
	dataset := &generator.Dataset{
		Tables: []*generator.GeneratedTable{
			{TableName: "users", Rows: nil},
		},
	}
	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected no errors for empty rows, got: %v", errs)
	}
}

// ── End-to-end golden test using generator ─────────────────────────────────

func TestValidate_GoldenGeneration(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "employees",
				Columns: []schema.Column{
					{Name: "id", Type: "int", Nullable: false, IsPrimaryKey: true},
					{Name: "name", Type: "varchar", Nullable: false, Length: 100},
					{Name: "email", Type: "varchar", Nullable: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	buildTableMap(model)

	// Generate data using the full pipeline.
	dataset, err := generateFullDataset(t, model, 10)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Validate the generated data — should pass cleanly.
	errs := Validate(dataset, model)
	if errs != nil {
		t.Errorf("expected clean validation for generated data, got %d errors: %v", len(errs), errs)
	}

	// Verify we actually have data to validate.
	if len(dataset.Tables) == 0 || len(dataset.Tables[0].Rows) != 10 {
		t.Errorf("expected 10 rows in generated data, got %d", len(dataset.Tables[0].Rows))
	}
}

// generateFullDataset runs the full generator pipeline for use in validator tests.
func generateFullDataset(t *testing.T, model *schema.Model, rows int) (*generator.Dataset, error) {
	t.Helper()

	g, err := graph.Build(model)
	if err != nil {
		return nil, err
	}

	sg, err := semantic.Build(g)
	if err != nil {
		return nil, err
	}

	plan, err := planner.BuildPlan(g, model, rows)
	if err != nil {
		return nil, err
	}

	ctx := &generator.GenerationContext{
		GlobalSeed:    42,
		Model:         model,
		Graph:         g,
		SemanticGraph: sg,
	}

	return generator.Generate(plan, ctx)
}
