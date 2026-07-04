package generator

import (
	"math/rand/v2"
	"strings"
	"testing"

	"synthgraph/internal/schema"
)

// ── Type-specific generation tests ─────────────────────────────────────────

func TestGenerate_BoolColumn(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "flags",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "active", Type: "boolean"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	ctx, plan := setupGeneratorTest(t, model)
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, row := range dataset.Tables[0].Rows {
		_, ok := row["active"].(bool)
		if !ok {
			t.Errorf("expected bool, got %T", row["active"])
		}
	}
}

func TestGenerate_DecimalColumn(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "prices",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "amount", Type: "decimal", Length: 10, Precision: 2},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	ctx, plan := setupGeneratorTest(t, model)
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, row := range dataset.Tables[0].Rows {
		val, ok := row["amount"].(string)
		if !ok {
			t.Errorf("expected string, got %T", row["amount"])
			continue
		}
		parts := strings.Split(val, ".")
		if len(parts) != 2 {
			t.Errorf("expected decimal format 'X.YY', got %q", val)
			continue
		}
		if len(parts[1]) != 2 {
			t.Errorf("expected 2 decimal places, got %q", val)
		}
	}
}

func TestGenerate_JSONColumn(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "config",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "data", Type: "jsonb"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}

	ctx, plan := setupGeneratorTest(t, model)
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, row := range dataset.Tables[0].Rows {
		json, ok := row["data"].(string)
		if !ok {
			t.Errorf("expected string, got %T", row["data"])
			continue
		}
		if !strings.HasPrefix(json, "{") || !strings.HasSuffix(json, "}") {
			t.Errorf("expected JSON object, got %q", json)
		}
	}
}

// ── Type generators ────────────────────────────────────────────────────────

func TestTypeGeneratorFor_KnownTypes(t *testing.T) {
	model := &schema.Model{}
	enumValues := buildEnumValues(model)

	tests := []struct {
		typeName string
	}{
		{"int"}, {"varchar"}, {"text"}, {"uuid"}, {"timestamp"},
		{"boolean"}, {"decimal"}, {"jsonb"}, {"inet"}, {"macaddr"},
	}

	for _, test := range tests {
		t.Run(test.typeName, func(t *testing.T) {
			generator := typeGeneratorFor(test.typeName, model, enumValues)
			rng := rand.New(rand.NewPCG(1, 2))
			col := &schema.Column{Name: test.typeName, Type: test.typeName}
			val, err := generator.Generate(col, 0, rng)
			if err != nil {
				t.Errorf("Generate failed: %v", err)
			}
			if val == nil {
				t.Errorf("generated nil value for %s", test.typeName)
			}
		})
	}
}

func TestTypeGeneratorFor_Enum(t *testing.T) {
	model := &schema.Model{
		Enums: []schema.EnumType{
			{Name: "order_status", Values: []string{"pending", "confirmed", "shipped", "delivered", "cancelled"}},
		},
	}
	enumValues := buildEnumValues(model)

	generator := typeGeneratorFor("order_status", model, enumValues)
	rng := rand.New(rand.NewPCG(1, 2))
	col := &schema.Column{Name: "status", Type: "order_status"}

	for i := 0; i < 20; i++ {
		val, err := generator.Generate(col, i, rng)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		strVal, ok := val.(string)
		if !ok {
			t.Fatalf("expected string, got %T", val)
		}
		found := false
		for _, v := range model.Enums[0].Values {
			if strVal == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("value %q not in enum %v", strVal, model.Enums[0].Values)
		}
	}
}

func TestUnknownTypeGenerator(t *testing.T) {
	model := &schema.Model{}
	enumValues := buildEnumValues(model)

	generator := typeGeneratorFor("point", model, enumValues)
	rng := rand.New(rand.NewPCG(1, 2))
	col := &schema.Column{Name: "p", Type: "point", Length: 10}
	val, err := generator.Generate(col, 0, rng)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	str, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if !strings.HasPrefix(str, "<point:") {
		t.Errorf("expected <point:...> prefix, got %q", str)
	}
}
