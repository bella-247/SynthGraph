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

// ── Helpers ───────────────────────────────────────────────────────────────

func setupGeneratorTest(t *testing.T, model *schema.Model) (*GenerationContext, *planner.GenerationPlan) {
	t.Helper()

	g, err := graph.Build(model)
	if err != nil {
		t.Fatalf("graph.Build failed: %v", err)
	}

	sg, err := semantic.Build(g)
	if err != nil {
		t.Fatalf("semantic.Build failed: %v", err)
	}

	plan, err := planner.BuildPlan(g, model, 10)
	if err != nil {
		t.Fatalf("planner.BuildPlan failed: %v", err)
	}

	ctx := &GenerationContext{
		GlobalSeed:    42,
		Model:         model,
		Graph:         g,
		SemanticGraph: sg,
	}

	return ctx, plan
}

func TestGenerate_SingleTable(t *testing.T) {
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

	ctx, plan := setupGeneratorTest(t, model)
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(dataset.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(dataset.Tables))
	}

	table := dataset.Tables[0]
	if table.TableName != "users" {
		t.Errorf("expected 'users', got %q", table.TableName)
	}
	if len(table.Rows) != 10 {
		t.Errorf("expected 10 rows, got %d", len(table.Rows))
	}

	// Each row should have id, name, email.
	for i, row := range table.Rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row %d missing 'id'", i)
		}
		if _, ok := row["name"]; !ok {
			t.Errorf("row %d missing 'name'", i)
		}
		if _, ok := row["email"]; !ok {
			t.Errorf("row %d missing 'email'", i)
		}
	}

	// IDs should be unique (PK constraint).
	seenIDs := make(map[int64]bool)
	for _, row := range table.Rows {
		id := row["id"].(int64)
		if seenIDs[id] {
			t.Errorf("duplicate id %d", id)
		}
		seenIDs[id] = true
	}
}

func TestGenerate_FKResolution(t *testing.T) {
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

	ctx, plan := setupGeneratorTest(t, model)
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(dataset.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(dataset.Tables))
	}

	// Find orders table.
	ordersTable := dataset.Tables[1]
	if ordersTable.TableName != "orders" {
		t.Errorf("expected orders second, got %q", ordersTable.TableName)
	}

	// Collect user IDs.
	userIDs := make(map[int64]bool)
	usersTable := dataset.Tables[0]
	for _, row := range usersTable.Rows {
		userIDs[row["id"].(int64)] = true
	}

	// All order.user_id should reference a valid user ID.
	for i, row := range ordersTable.Rows {
		uid := row["user_id"].(int64)
		if !userIDs[uid] {
			t.Errorf("order %d references non-existent user_id %d", i, uid)
		}
	}
}

func TestGenerate_UniqueConstraint(t *testing.T) {
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

	// Generate more rows to stress-test uniqueness.
	g, err := graph.Build(model)
	if err != nil {
		t.Fatalf("graph.Build failed: %v", err)
	}
	sg, err := semantic.Build(g)
	if err != nil {
		t.Fatalf("semantic.Build failed: %v", err)
	}
	plan, err := planner.BuildPlan(g, model, 50)
	if err != nil {
		t.Fatalf("planner.BuildPlan failed: %v", err)
	}

	ctx := &GenerationContext{GlobalSeed: 42, Model: model, Graph: g, SemanticGraph: sg}
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	table := dataset.Tables[0]
	if len(table.Rows) != 50 {
		t.Fatalf("expected 50 rows, got %d", len(table.Rows))
	}

	// All emails should be unique.
	seen := make(map[string]bool)
	for _, row := range table.Rows {
		email := row["email"].(string)
		if seen[email] {
			t.Errorf("duplicate email %q", email)
		}
		seen[email] = true
	}
}

func TestGenerate_Determinism(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "a",
				Columns: []schema.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "val", Type: "varchar"},
				},
				PrimaryKey: []string{"id"},
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

	// Generate twice with same seed.
	g1, _ := graph.Build(model)
	sg1, _ := semantic.Build(g1)
	plan1, _ := planner.BuildPlan(g1, model, 5)
	ctx1 := &GenerationContext{GlobalSeed: 12345, Model: model, Graph: g1, SemanticGraph: sg1}
	dataset1, err := Generate(plan1, ctx1)
	if err != nil {
		t.Fatalf("first Generate failed: %v", err)
	}

	g2, _ := graph.Build(model)
	sg2, _ := semantic.Build(g2)
	plan2, _ := planner.BuildPlan(g2, model, 5)
	ctx2 := &GenerationContext{GlobalSeed: 12345, Model: model, Graph: g2, SemanticGraph: sg2}
	dataset2, err := Generate(plan2, ctx2)
	if err != nil {
		t.Fatalf("second Generate failed: %v", err)
	}

	// Same seed should produce identical results.
	for i := range dataset1.Tables {
		t1 := dataset1.Tables[i]
		t2 := dataset2.Tables[i]
		if t1.TableName != t2.TableName {
			t.Errorf("table %d: name mismatch: %q vs %q", i, t1.TableName, t2.TableName)
		}
		if len(t1.Rows) != len(t2.Rows) {
			t.Errorf("table %s: row count mismatch: %d vs %d", t1.TableName, len(t1.Rows), len(t2.Rows))
		}
		for j := range t1.Rows {
			for col := range t1.Rows[j] {
				v1 := t1.Rows[j][col]
				v2 := t2.Rows[j][col]
				if v1 != v2 {
					t.Errorf("%s row %d col %s: %v vs %v", t1.TableName, j, col, v1, v2)
				}
			}
		}
	}
}

func TestGenerate_UUIDFormat(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", IsPrimaryKey: true},
					{Name: "data", Type: "text"},
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
		uuid, ok := row["id"].(string)
		if !ok {
			t.Errorf("expected uuid to be string, got %T", row["id"])
			continue
		}
		// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		if len(uuid) != 36 {
			t.Errorf("expected 36-char UUID, got %q (len=%d)", uuid, len(uuid))
		}
		parts := strings.Split(uuid, "-")
		if len(parts) != 5 {
			t.Errorf("expected 5 UUID segments, got %d", len(parts))
		}
		// Version 4 indicator.
		if len(parts[2]) > 0 && parts[2][0] != '4' {
			t.Errorf("expected version 4 UUID, got %q", uuid)
		}
	}
}

func TestGenerate_CycleResolution(t *testing.T) {
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

	ctx, plan := setupGeneratorTest(t, model)

	// Verify the plan has deferred FKs.
	if len(plan.DeferredFKs) == 0 {
		t.Fatal("expected deferred FKs for cyclic dependency")
	}

	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(dataset.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(dataset.Tables))
	}

	// After backfill, no FK column should be NULL.
	for _, table := range dataset.Tables {
		for i, row := range table.Rows {
			for col, val := range row {
				if val == nil {
					t.Errorf("table %s row %d col %s is NULL after backfill", table.TableName, i, col)
				}
			}
		}
	}

	// Verify FK integrity: a.b_id must point to a valid b.id.
	bIDs := make(map[int64]bool)
	for _, row := range dataset.Tables[1].Rows {
		bIDs[row["id"].(int64)] = true
	}

	aTable := dataset.Tables[0]
	for _, row := range aTable.Rows {
		if bid, ok := row["b_id"].(int64); ok {
			if !bIDs[bid] {
				t.Errorf("a.b_id %d references non-existent b.id", bid)
			}
		}
	}
}

func TestGenerate_EmptyPlan(t *testing.T) {
	model := &schema.Model{}
	g, _ := graph.Build(model)
	sg, _ := semantic.Build(g)
	plan, _ := planner.BuildPlan(g, model, 10)

	ctx := &GenerationContext{GlobalSeed: 1, Model: model, Graph: g, SemanticGraph: sg}
	dataset, err := Generate(plan, ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(dataset.Tables) != 0 {
		t.Errorf("expected empty dataset, got %d tables", len(dataset.Tables))
	}
}

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

// ── Unique tracker ────────────────────────────────────────────────────────

func TestUniqueTracker(t *testing.T) {
	table := &schema.Table{
		PrimaryKey: []string{"id"},
		Unique:     [][]string{{"email"}},
		Columns: []schema.Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "email"},
		},
	}

	tracker := newUniqueTracker(table)

	if !tracker.isUniqueColumn("id") {
		t.Error("id should be unique (PK)")
	}
	if !tracker.isUniqueColumn("email") {
		t.Error("email should be unique (UNIQUE constraint)")
	}
	if tracker.isUniqueColumn("name") {
		t.Error("name should not be unique")
	}

	if tracker.checkSeen("id", 1) {
		t.Error("should not be seen yet")
	}
	tracker.record("id", 1)
	if !tracker.checkSeen("id", 1) {
		t.Error("should be seen after recording")
	}
}

// ── buildFKColumnMap ──────────────────────────────────────────────────────

func TestBuildFKColumnMap(t *testing.T) {
	model := &schema.Model{
		Tables: []*schema.Table{
			{Name: "users", Columns: []schema.Column{{Name: "id", Type: "int", IsPrimaryKey: true}}, PrimaryKey: []string{"id"}},
			{Name: "orders", Columns: []schema.Column{{Name: "id", Type: "int", IsPrimaryKey: true}, {Name: "user_id", Type: "int"}}, PrimaryKey: []string{"id"}, ForeignKeys: []schema.ForeignKey{{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}}},
		},
	}

	g, _ := graph.Build(model)
	fkMap := buildFKColumnMap(g)

	orderFKs, ok := fkMap["orders"]
	if !ok {
		t.Fatal("expected orders to have FK entries")
	}
	if len(orderFKs) != 1 {
		t.Fatalf("expected 1 FK entry, got %d", len(orderFKs))
	}
	if orderFKs[0].Column != "user_id" {
		t.Errorf("expected column user_id, got %q", orderFKs[0].Column)
	}
	if orderFKs[0].RefTable != "users" {
		t.Errorf("expected ref table users, got %q", orderFKs[0].RefTable)
	}
}

// ── Type generators ───────────────────────────────────────────────────────

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
