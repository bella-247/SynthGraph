package generator

import (
	"math/rand/v2"
	"strings"
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// ── RNG determinism tests ─────────────────────────────────────────────────

func TestNewTableRNG_Determinism(t *testing.T) {
	rng1 := newTableRNG(42, "users")
	rng2 := newTableRNG(42, "users")

	for i := 0; i < 100; i++ {
		v1 := rng1.Int64()
		v2 := rng2.Int64()
		if v1 != v2 {
			t.Fatalf("mismatch at iteration %d: %d vs %d", i, v1, v2)
		}
	}
}

func TestNewTableRNG_DifferentTables(t *testing.T) {
	rng1 := newTableRNG(42, "users")
	rng2 := newTableRNG(42, "orders")

	// Verify seeds differ by comparing first value.
	if rng1.Int64() == rng2.Int64() {
		t.Error("expected different seeds for different tables")
	}
}

func TestNewTableRNG_DifferentSeeds(t *testing.T) {
	rng1 := newTableRNG(1, "users")
	rng2 := newTableRNG(2, "users")

	if rng1.Int64() == rng2.Int64() {
		t.Error("expected different seeds for different global seeds")
	}
}

// ── Seed-based generation tests ───────────────────────────────────────────

func TestGenerate_DifferentSeeds(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "data",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "val", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	g1, _ := graph.Build(model)
	semantic.ResolveColumns(model)
	sg1, _ := semantic.Build(g1)
	plan1, _ := planner.BuildPlan(g1, model, 10)
	ctx1 := &GenerationContext{GlobalSeed: 1, Model: model, Graph: g1, SemanticGraph: sg1}
	d1, _ := Generate(plan1, ctx1)

	g2, _ := graph.Build(model)
	sg2, _ := semantic.Build(g2)
	plan2, _ := planner.BuildPlan(g2, model, 10)
	ctx2 := &GenerationContext{GlobalSeed: 99999, Model: model, Graph: g2, SemanticGraph: sg2}
	d2, _ := Generate(plan2, ctx2)

	// Different seeds should produce different data.
	same := true
	for i := range d1.Tables[0].Rows {
		v1 := d1.Tables[0].Rows[i]["val"].(string)
		v2 := d2.Tables[0].Rows[i]["val"].(string)
		if v1 != v2 {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different seeds to produce different data")
	}
}

func TestGenerate_ZeroRows(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "empty",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	g, _ := graph.Build(model)
	semantic.ResolveColumns(model)
	sg, _ := semantic.Build(g)
	plan, _ := planner.BuildPlan(g, model, 0)
	ctx := &GenerationContext{GlobalSeed: 42, Model: model, Graph: g, SemanticGraph: sg}
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(dataset.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(dataset.Tables))
	}
	if len(dataset.Tables[0].Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(dataset.Tables[0].Rows))
	}
}

// ── UUID generation ───────────────────────────────────────────────────────

func TestGenerateUUID(t *testing.T) {
	rng := rand.New(rand.NewPCG(0, 0))
	uuid := generateUUID(rng)

	if len(uuid) != 36 {
		t.Errorf("expected 36 chars, got %d: %q", len(uuid), uuid)
	}

	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 segments, got %d", len(parts))
	}

	// Version nibble should be 4.
	if len(parts[2]) > 0 && parts[2][0] != '4' {
		t.Errorf("expected version 4, got %c", parts[2][0])
	}
}
