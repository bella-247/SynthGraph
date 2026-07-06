package exporter

import (
	"strings"
	"testing"

	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// buildDataset generates a Dataset from a schema model for testing.
func buildDataset(t *testing.T, model *schema.Model) *generator.Dataset {
	t.Helper()
	schemaGraph, err := graph.Build(model)
	if err != nil {
		t.Fatalf("graph.Build: %v", err)
	}
	semantic.ResolveColumns(model)
	semanticGraph, err := semantic.Build(schemaGraph)
	if err != nil {
		t.Fatalf("semantic.Build: %v", err)
	}
	genPlan, err := planner.BuildPlan(schemaGraph, model, 5)
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

func TestExportSQL_SingleTable(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
					{Name: "active", Type: "boolean"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(t, model)
	var buf strings.Builder
	config := &ExportConfig{}
	err := ExportSQL(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportSQL failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INSERT INTO") {
		t.Errorf("expected INSERT INTO, got: %s", output)
	}
	if !strings.Contains(output, "TRUE") && !strings.Contains(output, "FALSE") {
		t.Errorf("expected boolean value, got: %s", output)
	}
}

func TestExportSQL_MultipleTables(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "email", Type: "varchar"},
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

	dataset := buildDataset(t, model)
	var buf strings.Builder
	config := &ExportConfig{}
	err := ExportSQL(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportSQL failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INSERT INTO users") {
		t.Errorf("expected users table, got: %s", output)
	}
	if !strings.Contains(output, "INSERT INTO orders") {
		t.Errorf("expected orders table, got: %s", output)
	}
}

func TestExportSQL_WithSchema(t *testing.T) {
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
		},
	}

	dataset := buildDataset(t, model)
	var buf strings.Builder
	config := &ExportConfig{SchemaName: "public"}
	err := ExportSQL(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportSQL failed: %v", err)
	}

	output := buf.String()
	expected := "INSERT INTO public.users"
	if !strings.Contains(output, expected) {
		t.Errorf("expected %q, got: %s", expected, output)
	}
}

func TestExportSQL_NULLValues(t *testing.T) {
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

	dataset := buildDataset(t, model)
	// Set all nickname values to NULL.
	for _, row := range dataset.Tables[0].Rows {
		row["nickname"] = nil
	}

	var buf strings.Builder
	config := &ExportConfig{}
	err := ExportSQL(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportSQL failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NULL") {
		t.Errorf("expected NULL values, got: %s", output)
	}
}

func TestExportCSV_SingleTable(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar"},
					{Name: "active", Type: "boolean"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	dataset := buildDataset(t, model)
	var buf strings.Builder
	config := &ExportConfig{IncludeHeader: true}
	err := ExportCSV(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"id","name","active"`) {
		t.Errorf("expected CSV header, got: %s", output)
	}
	// Should have at least a header + data rows.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines, got %d", len(lines))
	}
}

func TestExportCSV_NoHeader(t *testing.T) {
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

	dataset := buildDataset(t, model)
	var buf strings.Builder
	config := &ExportConfig{IncludeHeader: false}
	err := ExportCSV(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	output := buf.String()
	if strings.HasPrefix(output, `"id","label"`) {
		t.Errorf("expected no header, but header found: %s", output)
	}
}

func TestExportCSV_NULLValues(t *testing.T) {
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

	dataset := buildDataset(t, model)
	for _, row := range dataset.Tables[0].Rows {
		row["nickname"] = nil
	}

	var buf strings.Builder
	config := &ExportConfig{IncludeHeader: true}
	err := ExportCSV(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	output := buf.String()
	// The second field in data rows should be empty for NULL values.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines[1:] {
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			t.Fatalf("line %d: expected 2 fields, got %d: %q", i+1, len(parts), line)
		}
		if parts[1] != "" {
			t.Errorf("line %d: expected empty for NULL, got %q", i+1, parts[1])
		}
	}
}

func TestExport_EmptyDataset(t *testing.T) {
	model := &schema.Model{}
	dataset := &generator.Dataset{Tables: []*generator.GeneratedTable{}}

	var sqlBuf strings.Builder
	err := ExportSQL(&sqlBuf, dataset, model, &ExportConfig{})
	if err != nil {
		t.Fatalf("ExportSQL with empty dataset: %v", err)
	}
	if sqlBuf.Len() != 0 {
		t.Errorf("expected empty output, got: %s", sqlBuf.String())
	}

	var csvBuf strings.Builder
	err = ExportCSV(&csvBuf, dataset, model, &ExportConfig{})
	if err != nil {
		t.Fatalf("ExportCSV with empty dataset: %v", err)
	}
	if csvBuf.Len() != 0 {
		t.Errorf("expected empty output, got: %s", csvBuf.String())
	}
}

func TestExport_CycleResolution(t *testing.T) {
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

	dataset := buildDataset(t, model)
	var buf strings.Builder
	config := &ExportConfig{}
	err := ExportSQL(&buf, dataset, model, config)
	if err != nil {
		t.Fatalf("ExportSQL with cycle failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INSERT INTO a") {
		t.Errorf("expected table a, got: %s", output)
	}
	if !strings.Contains(output, "INSERT INTO b") {
		t.Errorf("expected table b, got: %s", output)
	}
}
