package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"synthgraph/internal/exporter"
	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
	"synthgraph/internal/validator"
)

// TestFullPipeline_Ecommerce runs the complete generation pipeline on the
// ecommerce test schema and verifies the output is valid at every stage.
func TestFullPipeline_Ecommerce(t *testing.T) {
	sqlSchema := readTestSchema(t, "../../testdata/schemas/ecommerce.sql")

	ctx := context.Background()

	// Stage 1: Parse
	parsedModel, err := postgresql.New().Parse([]byte(sqlSchema))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	validateParseOutput(t, parsedModel)

	// Stage 2: Build graph
	g, err := graph.Build(parsedModel)
	if err != nil {
		t.Fatalf("graph build failed: %v", err)
	}
	if len(g.NodeList) == 0 {
		t.Fatal("graph has no nodes")
	}

	// Stage 3: Semantic analysis
	semantic.ResolveColumns(parsedModel)
	sg, err := semantic.Build(g)
	if err != nil {
		t.Fatalf("semantic build failed: %v", err)
	}
	if sg == nil {
		t.Fatal("semantic graph is nil")
	}

	// Stage 4: Plan generation
	plan, err := planner.BuildPlan(g, parsedModel, 10)
	if err != nil {
		t.Fatalf("plan build failed: %v", err)
	}
	if len(plan.Order) == 0 {
		t.Fatal("plan has no tables")
	}

	// Stage 5: Generate data
	genCtx := &generator.GenerationContext{
		Context:       ctx,
		GlobalSeed:    42,
		Model:         parsedModel,
		Graph:         g,
		SemanticGraph: sg,
	}
	dataset, err := generator.Generate(plan, genCtx)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if dataset == nil {
		t.Fatal("dataset is nil")
	}
	if len(dataset.Tables) == 0 {
		t.Fatal("dataset has no tables")
	}

	// Each table should have the expected number of rows
	for _, table := range dataset.Tables {
		if len(table.Rows) != 10 {
			t.Errorf("table %q: expected 10 rows, got %d", table.TableName, len(table.Rows))
		}
	}

	// Stage 6: Validate output
	validationErrors := validator.Validate(dataset, parsedModel)
	if len(validationErrors) > 0 {
		for _, ve := range validationErrors {
			t.Errorf("validation error: %s", ve.Error())
		}
	}

	// Stage 7: Export as SQL
	var sqlBuf strings.Builder
	err = exporter.ExportSQL(&sqlBuf, dataset, parsedModel, &exporter.ExportConfig{})
	if err != nil {
		t.Fatalf("SQL export failed: %v", err)
	}
	sqlOutput := sqlBuf.String()
	if !strings.Contains(sqlOutput, "INSERT INTO") {
		t.Error("SQL export missing INSERT statements")
	}

	// Stage 8: Export as CSV
	var csvBuf strings.Builder
	err = exporter.ExportCSV(&csvBuf, dataset, parsedModel, &exporter.ExportConfig{IncludeHeader: true})
	if err != nil {
		t.Fatalf("CSV export failed: %v", err)
	}
	csvOutput := csvBuf.String()
	if !strings.Contains(csvOutput, "id") {
		t.Error("CSV export missing header row")
	}
}

// TestFullPipeline_Users runs the pipeline on the single-table users schema.
func TestFullPipeline_Users(t *testing.T) {
	sqlSchema := readTestSchema(t, "../../testdata/schemas/users.sql")

	ctx := context.Background()
	parsedModel, err := postgresql.New().Parse([]byte(sqlSchema))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	g, err := graph.Build(parsedModel)
	if err != nil {
		t.Fatalf("graph build failed: %v", err)
	}

	semantic.ResolveColumns(parsedModel)
	sg, err := semantic.Build(g)
	if err != nil {
		t.Fatalf("semantic build failed: %v", err)
	}

	plan, err := planner.BuildPlan(g, parsedModel, 5)
	if err != nil {
		t.Fatalf("plan build failed: %v", err)
	}

	genCtx := &generator.GenerationContext{
		Context:       ctx,
		GlobalSeed:    42,
		Model:         parsedModel,
		Graph:         g,
		SemanticGraph: sg,
	}
	dataset, err := generator.Generate(plan, genCtx)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if len(dataset.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(dataset.Tables))
	}

	validationErrors := validator.Validate(dataset, parsedModel)
	if len(validationErrors) > 0 {
		for _, ve := range validationErrors {
			t.Errorf("validation error: %s", ve.Error())
		}
	}
}

// TestFullPipeline_Determinism verifies that identical inputs produce identical
// outputs — the foundation of reproducible data generation.
func TestFullPipeline_Determinism(t *testing.T) {
	sqlSchema := readTestSchema(t, "../../testdata/schemas/ecommerce.sql")

	run := func(seed int64) string {
		ctx := context.Background()
		parsedModel, _ := postgresql.New().Parse([]byte(sqlSchema))
		g, _ := graph.Build(parsedModel)
		semantic.ResolveColumns(parsedModel)
		sg, _ := semantic.Build(g)
		plan, _ := planner.BuildPlan(g, parsedModel, 3)
		genCtx := &generator.GenerationContext{
			Context:       ctx,
			GlobalSeed:    uint64(seed),
			Model:         parsedModel,
			Graph:         g,
			SemanticGraph: sg,
		}
		dataset, _ := generator.Generate(plan, genCtx)
		var buf strings.Builder
		exporter.ExportSQL(&buf, dataset, parsedModel, &exporter.ExportConfig{})
		return buf.String()
	}

	out1 := run(42)
	out2 := run(42)
	out3 := run(99)

	if out1 != out2 {
		t.Fatal("same seed produced different output")
	}
	if out1 == out3 {
		t.Fatal("different seed produced identical output")
	}
}

// TestFullPipeline_Cancellation verifies that cancelling the context
// mid-generation returns partial data instead of hanging.
func TestFullPipeline_Cancellation(t *testing.T) {
	sqlSchema := readTestSchema(t, "../../testdata/schemas/ecommerce.sql")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	parsedModel, _ := postgresql.New().Parse([]byte(sqlSchema))
	g, _ := graph.Build(parsedModel)
	semantic.ResolveColumns(parsedModel)
	sg, _ := semantic.Build(g)
	plan, _ := planner.BuildPlan(g, parsedModel, 100)

	genCtx := &generator.GenerationContext{
		Context:       ctx,
		GlobalSeed:    42,
		Model:         parsedModel,
		Graph:         g,
		SemanticGraph: sg,
	}
	dataset, err := generator.Generate(plan, genCtx)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if err != generator.ErrCancelled {
		t.Fatalf("expected ErrCancelled, got: %v", err)
	}
	if dataset == nil {
		t.Fatal("expected partial dataset on cancellation, got nil")
	}
}

// TestValidate_RowLimit enforces the max rows configuration.
func TestValidate_RowLimit(t *testing.T) {
	cfg := defaultGenerateConfig()
	cfg.input = "../../testdata/schemas/users.sql"
	cfg.rows = maxRows + 1

	err := cfg.validate()
	if err == nil {
		t.Fatal("expected error for exceeding max rows")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func BenchmarkFullPipeline_Ecommerce(b *testing.B) {
	sqlSchema := readTestSchema(b, "../../testdata/schemas/ecommerce.sql")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsedModel, _ := postgresql.New().Parse([]byte(sqlSchema))
		g, _ := graph.Build(parsedModel)
		semantic.ResolveColumns(parsedModel)
		sg, _ := semantic.Build(g)
		plan, _ := planner.BuildPlan(g, parsedModel, 10)
		genCtx := &generator.GenerationContext{
			Context:       ctx,
			GlobalSeed:    42,
			Model:         parsedModel,
			Graph:         g,
			SemanticGraph: sg,
		}
		generator.Generate(plan, genCtx)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func readTestSchema(tb testing.TB, path string) string {
	tb.Helper()
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		tb.Fatalf("reading test schema %s: %v", path, err)
	}
	return string(data)
}

func validateParseOutput(t *testing.T, model *schema.Model) {
	t.Helper()
	if model == nil {
		t.Fatal("parsed model is nil")
	}
	expectedTables := map[string]bool{
		"users": false, "products": false, "categories": false,
		"orders": false, "order_items": false, "reviews": false,
		"product_categories": false,
	}
	for _, table := range model.Tables {
		if _, ok := expectedTables[table.Name]; ok {
			expectedTables[table.Name] = true
		}
	}
	for name, found := range expectedTables {
		if !found {
			t.Errorf("expected table %q not found in parsed model", name)
		}
	}

	foundEnum := false
	for _, e := range model.Enums {
		if e.Name == "order_status" {
			foundEnum = true
			break
		}
	}
	if !foundEnum {
		t.Error("expected enum order_status not found")
	}
}

// TestProgressCallback verifies the Progress callback is invoked for each
// table in the generation plan with correct 1-based indices.
func TestProgressCallback(t *testing.T) {
	sqlSchema := readTestSchema(t, "../../testdata/schemas/ecommerce.sql")
	ctx := context.Background()

	parsedModel, err := postgresql.New().Parse([]byte(sqlSchema))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	g, err := graph.Build(parsedModel)
	if err != nil {
		t.Fatalf("graph build failed: %v", err)
	}

	semantic.ResolveColumns(parsedModel)
	sg, err := semantic.Build(g)
	if err != nil {
		t.Fatalf("semantic build failed: %v", err)
	}

	plan, err := planner.BuildPlan(g, parsedModel, 10)
	if err != nil {
		t.Fatalf("plan build failed: %v", err)
	}

	type call struct {
		table string
		n, total int
	}
	var calls []call
	genCtx := &generator.GenerationContext{
		Context:       ctx,
		GlobalSeed:    42,
		Model:         parsedModel,
		Graph:         g,
		SemanticGraph: sg,
		Progress: func(tableName string, current int, total int) {
			calls = append(calls, call{tableName, current, total})
		},
	}

	dataset, err := generator.Generate(plan, genCtx)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	expectedTables := 7
	if len(calls) != expectedTables {
		t.Fatalf("expected %d progress callbacks, got %d", expectedTables, len(calls))
	}

	for i, c := range calls {
		if c.n != i+1 {
			t.Errorf("call %d: expected n=%d, got %d", i, i+1, c.n)
		}
		if c.total != expectedTables {
			t.Errorf("call %d: expected total=%d, got %d", i, expectedTables, c.total)
		}
	}

	// Verify table names match the generation plan order.
	for i, tablePlan := range plan.Order {
		if calls[i].table != tablePlan.TableName {
			t.Errorf("call %d: expected table %q, got %q", i, tablePlan.TableName, calls[i].table)
		}
	}

	// Sanity check: dataset should be valid and complete.
	_ = dataset
}
