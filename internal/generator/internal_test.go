package generator

import (
	"testing"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

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

	if tracker.isUniqueColumn("id") {
		t.Error("PK columns do not need uniqueness tracking (RNG guaranteed)")
	}
	if !tracker.isUniqueColumn("email") {
		t.Error("email should be unique (UNIQUE constraint)")
	}
	if tracker.isUniqueColumn("name") {
		t.Error("name should not be unique")
	}

	if tracker.checkSeen("id", 1) {
		t.Error("id should not be tracked")
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
