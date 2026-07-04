// Demo program: displays the full translation and graph construction pipeline.
//
// Phase 1 — Canonical schema.Model (the intermediate representation)
// Phase 2 — Canonical Graph (the central data structure of SynthGraph)
// Phase 3 — JSON dump of the complete graph
// Phase 4 — Mermaid ER diagram visualization
//
// Since we build the AST directly here (bypassing the SQL parser),
// this demo works without CGO.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"synthgraph/internal/graph"
	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/render/mermaid"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

func main() {
	// ── Input: Build a comprehensive e-commerce schema ─────────────────────

	enumOrderStatus := postgresql.CreateEnumStmt{
		Name:   "order_status",
		Values: []string{"pending", "confirmed", "shipped", "delivered", "cancelled"},
	}

	enumProductCategory := postgresql.CreateEnumStmt{
		Schema: "inventory",
		Name:   "product_category",
		Values: []string{"electronics", "clothing", "food", "books", "other"},
	}

	tableUsers := postgresql.CreateTableStmt{
		Name: "users",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "email", Type: postgresql.ColumnType{BaseType: "varchar", Length: 255}, NotNull: true, IsUnique: true},
			{Name: "full_name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 200}, NotNull: true},
			{Name: "is_active", Type: postgresql.ColumnType{BaseType: "boolean"}, Default: "true"},
			{Name: "metadata", Type: postgresql.ColumnType{BaseType: "jsonb"}},
			{Name: "created_at", Type: postgresql.ColumnType{BaseType: "timestamp"}, Default: "now()"},
		},
	}

	tableProducts := postgresql.CreateTableStmt{
		Name: "products",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 500}, NotNull: true},
			{Name: "description", Type: postgresql.ColumnType{BaseType: "text"}},
			{Name: "category", Type: postgresql.ColumnType{BaseType: "inventory.product_category"}, NotNull: true},
			{Name: "price", Type: postgresql.ColumnType{BaseType: "decimal", Length: 10, Precision: 2}, NotNull: true},
			{Name: "stock", Type: postgresql.ColumnType{BaseType: "int"}, Default: "0", NotNull: true},
			{Name: "sku", Type: postgresql.ColumnType{BaseType: "varchar", Length: 50}, NotNull: true, IsUnique: true},
			{Name: "image_urls", Type: postgresql.ColumnType{BaseType: "text", IsArray: true}},
		},
		TableConstraints: []postgresql.TableConstraint{
			{Type: postgresql.ConstraintCheck, Name: "positive_price", CheckExpr: "price > 0"},
			{Type: postgresql.ConstraintCheck, Name: "non_negative_stock", CheckExpr: "stock >= 0"},
		},
	}

	tableOrders := postgresql.CreateTableStmt{
		Name: "orders",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{
				Name:       "user_id",
				Type:       postgresql.ColumnType{BaseType: "bigint"},
				NotNull:    true,
				References: &postgresql.InlineFKRef{RefTable: "users", RefColumns: []string{"id"}, OnDelete: postgresql.FKNoAction},
			},
			{Name: "status", Type: postgresql.ColumnType{BaseType: "order_status"}, Default: "'pending'", NotNull: true},
			{Name: "total", Type: postgresql.ColumnType{BaseType: "decimal", Length: 12, Precision: 2}, NotNull: true},
			{Name: "shipping_address", Type: postgresql.ColumnType{BaseType: "text"}, NotNull: true},
			{Name: "created_at", Type: postgresql.ColumnType{BaseType: "timestamp"}, Default: "now()"},
			{Name: "updated_at", Type: postgresql.ColumnType{BaseType: "timestamp"}},
		},
		TableConstraints: []postgresql.TableConstraint{
			{Type: postgresql.ConstraintCheck, CheckExpr: "total >= 0"},
		},
	}

	tableOrderItems := postgresql.CreateTableStmt{
		Name: "order_items",
		Columns: []postgresql.ColumnDef{
			{Name: "order_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "product_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "quantity", Type: postgresql.ColumnType{BaseType: "int"}, Default: "1", NotNull: true},
			{Name: "unit_price", Type: postgresql.ColumnType{BaseType: "decimal", Length: 10, Precision: 2}, NotNull: true},
		},
		TableConstraints: []postgresql.TableConstraint{
			{Type: postgresql.ConstraintPrimaryKey, Columns: []string{"order_id", "product_id"}},
			{Type: postgresql.ConstraintForeignKey, Columns: []string{"order_id"}, RefTable: "orders", RefColumns: []string{"id"}, OnDelete: postgresql.FKCascade},
			{Type: postgresql.ConstraintForeignKey, Columns: []string{"product_id"}, RefTable: "products", RefColumns: []string{"id"}},
			{Type: postgresql.ConstraintCheck, CheckExpr: "quantity > 0"},
			{Type: postgresql.ConstraintCheck, CheckExpr: "unit_price >= 0"},
		},
	}

	tableReviews := postgresql.CreateTableStmt{
		Name: "reviews",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "order_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true, References: &postgresql.InlineFKRef{RefTable: "orders", RefColumns: []string{"id"}, OnDelete: postgresql.FKCascade}},
			{Name: "product_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true, References: &postgresql.InlineFKRef{RefTable: "products", RefColumns: []string{"id"}, OnDelete: postgresql.FKRestrict}},
			{Name: "user_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true, References: &postgresql.InlineFKRef{RefTable: "users", RefColumns: []string{"id"}, OnDelete: postgresql.FKSetNull}},
			{Name: "rating", Type: postgresql.ColumnType{BaseType: "int"}, NotNull: true},
			{Name: "comment", Type: postgresql.ColumnType{BaseType: "text"}},
			{Name: "created_at", Type: postgresql.ColumnType{BaseType: "timestamp"}, Default: "now()"},
		},
		TableConstraints: []postgresql.TableConstraint{
			{Type: postgresql.ConstraintCheck, Name: "valid_rating", CheckExpr: "rating >= 1 AND rating <= 5"},
			{Type: postgresql.ConstraintUnique, Columns: []string{"order_id", "product_id"}},
		},
	}

	tableCategories := postgresql.CreateTableStmt{
		Schema: "inventory",
		Name:   "categories",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 100}, NotNull: true, IsUnique: true},
			{Name: "slug", Type: postgresql.ColumnType{BaseType: "varchar", Length: 100}, NotNull: true, IsUnique: true},
			{Name: "description", Type: postgresql.ColumnType{BaseType: "text"}},
		},
	}

	tableProductCategories := postgresql.CreateTableStmt{
		Name: "product_categories",
		Columns: []postgresql.ColumnDef{
			{Name: "product_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "category_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
		},
		TableConstraints: []postgresql.TableConstraint{
			{Type: postgresql.ConstraintPrimaryKey, Columns: []string{"product_id", "category_id"}},
			{Type: postgresql.ConstraintForeignKey, Columns: []string{"product_id"}, RefTable: "products", RefColumns: []string{"id"}, OnDelete: postgresql.FKCascade},
			{Type: postgresql.ConstraintForeignKey, Columns: []string{"category_id"}, RefTable: "inventory.categories", RefColumns: []string{"id"}, OnDelete: postgresql.FKCascade},
		},
	}

	tableEmployees := postgresql.CreateTableStmt{
		Name: "employees",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 200}, NotNull: true},
			{Name: "email", Type: postgresql.ColumnType{BaseType: "varchar", Length: 255}, NotNull: true, IsUnique: true},
			{Name: "salary", Type: postgresql.ColumnType{BaseType: "decimal", Length: 12, Precision: 2}},
			{Name: "manager_id", Type: postgresql.ColumnType{BaseType: "bigint"}, References: &postgresql.InlineFKRef{RefTable: "employees", RefColumns: []string{"id"}, OnDelete: postgresql.FKSetNull}},
		},
		TableConstraints: []postgresql.TableConstraint{
			{Type: postgresql.ConstraintCheck, Name: "positive_salary", CheckExpr: "salary > 0 OR salary IS NULL"},
		},
	}

	statements := []postgresql.Stmt{
		enumOrderStatus,
		enumProductCategory,
		tableUsers,
		tableProducts,
		tableCategories,
		tableOrders,
		tableOrderItems,
		tableReviews,
		tableProductCategories,
		tableEmployees,
	}

	// ── Phase 1: Translate to schema.Model ────────────────────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 1 — CANONICAL SCHEMA MODEL")
	fmt.Println(strings.Repeat("═", 72))

	schemaModel, err := postgresql.Translate(statements)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError during translation: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(summarizeSchema(schemaModel))

	// ── Phase 2: Build the Canonical Graph ────────────────────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 2 — CANONICAL GRAPH")
	fmt.Println(strings.Repeat("═", 72))

	schemaGraph, err := graph.Build(schemaModel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError during graph construction: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(summarizeGraph(schemaGraph))
	fmt.Println()

	// ── Phase 4: Build the Semantic Graph ──────────────────────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 4 — SEMANTIC GRAPH (INFERRED ROLES & RELATIONSHIPS)")
	fmt.Println(strings.Repeat("═", 72))

	semanticGraph, err := semantic.Build(schemaGraph)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError during semantic graph construction: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(summarizeSemanticGraph(semanticGraph))
	fmt.Println()

	// ── Phase 3: Full JSON dump ───────────────────────────────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 3 — FULL JSON DUMP")
	fmt.Println(strings.Repeat("═", 72))
	fmt.Println()

	jsonOutput := dumpGraphAsJSON(schemaGraph)
	fmt.Println(jsonOutput)

	// ── Phase 4: Mermaid ER diagram ───────────────────────────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 4 — MERMAID ER DIAGRAM")
	fmt.Println(strings.Repeat("═", 72))
	fmt.Println()

	mermaidOutput, err := mermaid.Render(schemaGraph)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError during Mermaid rendering: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(mermaidOutput)
}

// ── Schema summary ──────────────────────────────────────────────────────────

func summarizeSchema(model *schema.Model) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("\n  Tables:  %d\n", len(model.Tables)))
	builder.WriteString(fmt.Sprintf("  Enums:   %d\n", len(model.Enums)))
	builder.WriteString("\n")

	for _, table := range model.Tables {
		builder.WriteString(fmt.Sprintf("  ▸ %s (%d cols, PK=%v", table.Name, len(table.Columns), table.PrimaryKey))
		if len(table.ForeignKeys) > 0 {
			builder.WriteString(fmt.Sprintf(", FK=%d", len(table.ForeignKeys)))
		}
		if len(table.Unique) > 0 {
			builder.WriteString(fmt.Sprintf(", UQ=%d", len(table.Unique)))
		}
		if len(table.Checks) > 0 {
			builder.WriteString(fmt.Sprintf(", CK=%d", len(table.Checks)))
		}
		builder.WriteString(")\n")

		for _, col := range table.Columns {
			nullable := "NULL"
			if !col.Nullable {
				nullable = "NOT NULL"
			}
			precision := ""
			if col.Length > 0 {
				if col.Precision > 0 {
					precision = fmt.Sprintf("(%d,%d)", col.Length, col.Precision)
				} else {
					precision = fmt.Sprintf("(%d)", col.Length)
				}
			}
			builder.WriteString(fmt.Sprintf("    • %s %s%s [%s]", col.Name, col.Type, precision, nullable))
			if col.Default != nil {
				builder.WriteString(fmt.Sprintf(" DEFAULT %s", *col.Default))
			}
			if col.IsPrimaryKey {
				builder.WriteString(" PK")
			}
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n")
	return builder.String()
}

// ── Graph summary ──────────────────────────────────────────────────────────

func summarizeGraph(schemaGraph *graph.Graph) string {
	var builder strings.Builder

	// Node counts by kind
	tableCount := 0
	columnCount := 0
	enumCount := 0
	for _, node := range schemaGraph.NodeList {
		switch node.Kind {
		case graph.NodeKindTable:
			tableCount++
		case graph.NodeKindColumn:
			columnCount++
		case graph.NodeKindEnum:
			enumCount++
		}
	}

	// Edge counts by kind
	containsCount := 0
	referencesCount := 0
	referencedByCount := 0
	dependsOnCount := 0
	usesEnumCount := 0
	for _, edge := range schemaGraph.Edges {
		switch edge.Kind {
		case graph.EdgeKindContains:
			containsCount++
		case graph.EdgeKindReferences:
			referencesCount++
		case graph.EdgeKindReferencedBy:
			referencedByCount++
		case graph.EdgeKindDependsOn:
			dependsOnCount++
		case graph.EdgeKindUsesEnum:
			usesEnumCount++
		}
	}

	builder.WriteString(fmt.Sprintf("\n  Nodes:  %d total (%d tables, %d columns, %d enums)\n",
		len(schemaGraph.NodeList), tableCount, columnCount, enumCount))
	builder.WriteString(fmt.Sprintf("  Edges:  %d total (%d contains, %d references, %d referenced_by, %d depends_on, %d uses_enum)\n",
		len(schemaGraph.Edges), containsCount, referencesCount, referencedByCount, dependsOnCount, usesEnumCount))
	builder.WriteString("\n")

	// Print each table node and its outgoing edges
	for _, node := range schemaGraph.NodeList {
		if node.Kind != graph.NodeKindTable {
			continue
		}
		tableData := node.Data.(graph.TableData)
		builder.WriteString(fmt.Sprintf("  ▸ %s\n", node.Label))
		for _, col := range tableData.PrimaryKey {
			builder.WriteString(fmt.Sprintf("      PK: %s\n", col))
		}
		for _, uniqueConstraint := range tableData.Unique {
			builder.WriteString(fmt.Sprintf("      UQ: %s\n", strings.Join(uniqueConstraint, ", ")))
		}
		for _, check := range tableData.Checks {
			if check.Name != "" {
				builder.WriteString(fmt.Sprintf("      CK: %s → %s\n", check.Name, check.Expression))
			} else {
				builder.WriteString(fmt.Sprintf("      CK: %s\n", check.Expression))
			}
		}

		// Contains edges: columns
		for _, edge := range schemaGraph.Edges {
			if edge.From == node.ID && edge.Kind == graph.EdgeKindContains {
				columnNode := schemaGraph.Nodes[edge.To]
				columnData := columnNode.Data.(graph.ColumnData)
				typeInfo := columnData.Type
				if columnData.Length > 0 {
					if columnData.Precision > 0 {
						typeInfo = fmt.Sprintf("%s(%d,%d)", typeInfo, columnData.Length, columnData.Precision)
					} else {
						typeInfo = fmt.Sprintf("%s(%d)", typeInfo, columnData.Length)
					}
				}
				nullable := "NULL"
				if !columnData.Nullable {
					nullable = "NOT NULL"
				}
				builder.WriteString(fmt.Sprintf("    • %s %s [%s]", columnNode.Label, typeInfo, nullable))
				if columnData.Default != nil {
					builder.WriteString(fmt.Sprintf(" DEFAULT %s", *columnData.Default))
				}
				if columnData.IsPrimaryKey {
					builder.WriteString(" PK")
				}
				builder.WriteString("\n")
			}
		}

		// Reference edges: FK relationships
		for _, edge := range schemaGraph.Edges {
			if edge.From == node.ID && edge.Kind == graph.EdgeKindReferences {
				fkMetadata := edge.Metadata.(*graph.FKMetadata)
				builder.WriteString(fmt.Sprintf("    ⤷ REF %s", edge.To))
				builder.WriteString(fmt.Sprintf("  (%s → %s) [%s]",
					strings.Join(fkMetadata.LocalColumns, ", "),
					strings.Join(fkMetadata.ForeignColumns, ", "),
					fkMetadata.Cardinality))
				if fkMetadata.OnDelete != "" && fkMetadata.OnDelete != schema.FKNoAction {
					builder.WriteString(fmt.Sprintf(" ON DELETE %s", fkMetadata.OnDelete))
				}
				if fkMetadata.OnUpdate != "" && fkMetadata.OnUpdate != schema.FKNoAction {
					builder.WriteString(fmt.Sprintf(" ON UPDATE %s", fkMetadata.OnUpdate))
				}
				builder.WriteString("\n")
			}
		}

		// Referenced-by edges: reverse FK direction
		for _, edge := range schemaGraph.Edges {
			if edge.From == node.ID && edge.Kind == graph.EdgeKindReferencedBy {
				fkMetadata := edge.Metadata.(*graph.FKMetadata)
				builder.WriteString(fmt.Sprintf("    ← REF_BY %s  (%s → %s) [%s]\n",
					edge.To,
					strings.Join(fkMetadata.LocalColumns, ", "),
					strings.Join(fkMetadata.ForeignColumns, ", "),
					fkMetadata.Cardinality))
			}
		}

		// Depends-on edges: impact analysis
		for _, edge := range schemaGraph.Edges {
			if edge.From == node.ID && edge.Kind == graph.EdgeKindDependsOn {
				fkMetadata := edge.Metadata.(*graph.FKMetadata)
				builder.WriteString(fmt.Sprintf("    ⚠ DEPENDS_ON %s  (%s → %s) [%s]\n",
					edge.To,
					strings.Join(fkMetadata.LocalColumns, ", "),
					strings.Join(fkMetadata.ForeignColumns, ", "),
					fkMetadata.Cardinality))
			}
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// ── JSON dump ───────────────────────────────────────────────────────────────

// graphJSON is a JSON-friendly representation of the graph.
// It avoids issues with the Data interface{} field by rendering nodes and edges
// as structs with explicit type-discriminated payloads.
type graphJSON struct {
	Nodes []nodeJSON `json:"nodes"`
	Edges []edgeJSON `json:"edges"`
}

type nodeJSON struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Data  any    `json:"data,omitempty"`
}

type edgeJSON struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Kind     string `json:"kind"`
	Metadata any    `json:"metadata,omitempty"`
}

func dumpGraphAsJSON(schemaGraph *graph.Graph) string {
	graphOutput := graphJSON{
		Nodes: make([]nodeJSON, 0, len(schemaGraph.NodeList)),
		Edges: make([]edgeJSON, 0, len(schemaGraph.Edges)),
	}

	for _, node := range schemaGraph.NodeList {
		nodeEntry := nodeJSON{
			ID:    node.ID,
			Kind:  string(node.Kind),
			Label: node.Label,
			Data:  node.Data,
		}
		graphOutput.Nodes = append(graphOutput.Nodes, nodeEntry)
	}

	for _, edge := range schemaGraph.Edges {
		edgeEntry := edgeJSON{
			From:     edge.From,
			To:       edge.To,
			Kind:     string(edge.Kind),
			Metadata: edge.Metadata,
		}
		graphOutput.Edges = append(graphOutput.Edges, edgeEntry)
	}

	encoded, err := json.MarshalIndent(graphOutput, "", "  ")
	if err != nil {
		return fmt.Sprintf("JSON encoding error: %v", err)
	}
	return string(encoded)
}

// ── Semantic graph summary ────────────────────────────────────────────────────

func summarizeSemanticGraph(semanticGraph *semantic.SemanticGraph) string {
	var builder strings.Builder

	// Table roles
	builder.WriteString("\n  Table Roles:\n")
	for _, node := range semanticGraph.NodeList {
		if node.Kind != graph.NodeKindTable {
			continue
		}
		roleNames := make([]string, 0, len(node.Roles))
		for _, role := range node.Roles {
			roleNames = append(roleNames, string(role))
		}
		builder.WriteString(fmt.Sprintf("    ▸ %-30s  %s\n", node.Label, strings.Join(roleNames, ", ")))
	}

	// Temporal / audit / soft-delete
	builder.WriteString("\n  Temporal & Audit Patterns:\n")
	for _, node := range semanticGraph.NodeList {
		if node.Kind != graph.NodeKindTable {
			continue
		}
		details := make([]string, 0, 5)
		if node.Temporal != nil {
			if node.Temporal.HasCreatedAt {
				details = append(details, "created_at")
			}
			if node.Temporal.HasUpdatedAt {
				details = append(details, "updated_at")
			}
			if node.Temporal.HasDeletedAt {
				details = append(details, "deleted_at")
			}
		}
		if node.IsSoftDelete {
			details = append(details, "soft_delete")
		}
		if node.Audit != nil {
			if node.Audit.HasCreatedBy {
				details = append(details, "created_by")
			}
			if node.Audit.HasUpdatedBy {
				details = append(details, "updated_by")
			}
			if node.Audit.HasDeletedBy {
				details = append(details, "deleted_by")
			}
		}
		if len(details) > 0 {
			builder.WriteString(fmt.Sprintf("    ▸ %-30s  %s\n", node.Label, strings.Join(details, ", ")))
		}
	}

	// Relationship semantics
	builder.WriteString("\n  Relationship Semantics:\n")
	for _, rel := range semanticGraph.Relationships {
		builder.WriteString(fmt.Sprintf("    %-26s  →  %-26s  [%s]\n",
			rel.From, rel.To, rel.Kind))
	}

	return builder.String()
}
