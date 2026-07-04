// Demo program: displays the full translation pipeline.
//
// Phase 1 — Raw DDL AST (the parsed statements before translation)
// Phase 2 — Translated schema.Model (the canonical intermediate representation)
//
// Since we build the AST directly here (bypassing the SQL parser),
// this demo works without CGO.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/schema"
)

func main() {
	// ── Input: Build a comprehensive e-commerce schema ─────────────────────
	//
	// This schema tests the full capability of the translator:
	//   - Enums (order_status, product_category)
	//   - Tables with various column types (varchar, decimal, text, int, jsonb, uuid)
	//   - Inline PRIMARY KEY, table-level composite PK
	//   - Inline REFERENCES (column-level FK)
	//   - Table-level FOREIGN KEY with ON DELETE / ON UPDATE actions
	//   - UNIQUE constraints (inline and table-level)
	//   - CHECK constraints with expressions
	//   - NOT NULL, DEFAULT values
	//   - Self-referencing FK (employees)
	//   - Composite PK (order_items)
	//   - Schema-qualified tables

	// ── Enums ────────────────────────────────────────────────────────────

	enumOrderStatus := postgresql.CreateEnumStmt{
		Name:   "order_status",
		Values: []string{"pending", "confirmed", "shipped", "delivered", "cancelled"},
	}

	enumProductCategory := postgresql.CreateEnumStmt{
		Schema: "inventory",
		Name:   "product_category",
		Values: []string{"electronics", "clothing", "food", "books", "other"},
	}

	// ── Tables ───────────────────────────────────────────────────────────

	// Users: core identity table
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

	// Products: inventory items with CHECK constraints
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
			{
				Type:      postgresql.ConstraintCheck,
				Name:      "positive_price",
				CheckExpr: "price > 0",
			},
			{
				Type:      postgresql.ConstraintCheck,
				Name:      "non_negative_stock",
				CheckExpr: "stock >= 0",
			},
		},
	}

	// Orders: references users via inline FK
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
			{
				Type:      postgresql.ConstraintCheck,
				CheckExpr: "total >= 0",
			},
		},
	}

	// Order items: composite PK, references orders and products
	tableOrderItems := postgresql.CreateTableStmt{
		Name: "order_items",
		Columns: []postgresql.ColumnDef{
			{Name: "order_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "product_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "quantity", Type: postgresql.ColumnType{BaseType: "int"}, Default: "1", NotNull: true},
			{Name: "unit_price", Type: postgresql.ColumnType{BaseType: "decimal", Length: 10, Precision: 2}, NotNull: true},
		},
		TableConstraints: []postgresql.TableConstraint{
			{
				Type:    postgresql.ConstraintPrimaryKey,
				Columns: []string{"order_id", "product_id"},
			},
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"order_id"},
				RefTable:   "orders",
				RefColumns: []string{"id"},
				OnDelete:   postgresql.FKCascade,
			},
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"product_id"},
				RefTable:   "products",
				RefColumns: []string{"id"},
			},
			{
				Type:      postgresql.ConstraintCheck,
				CheckExpr: "quantity > 0",
			},
			{
				Type:      postgresql.ConstraintCheck,
				CheckExpr: "unit_price >= 0",
			},
		},
	}

	// Reviews: extends orders with per-product ratings
	tableReviews := postgresql.CreateTableStmt{
		Name: "reviews",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{
				Name:       "order_id",
				Type:       postgresql.ColumnType{BaseType: "bigint"},
				NotNull:    true,
				References: &postgresql.InlineFKRef{RefTable: "orders", RefColumns: []string{"id"}, OnDelete: postgresql.FKCascade},
			},
			{
				Name:       "product_id",
				Type:       postgresql.ColumnType{BaseType: "bigint"},
				NotNull:    true,
				References: &postgresql.InlineFKRef{RefTable: "products", RefColumns: []string{"id"}, OnDelete: postgresql.FKRestrict},
			},
			{
				Name:       "user_id",
				Type:       postgresql.ColumnType{BaseType: "bigint"},
				NotNull:    true,
				References: &postgresql.InlineFKRef{RefTable: "users", RefColumns: []string{"id"}, OnDelete: postgresql.FKSetNull},
			},
			{Name: "rating", Type: postgresql.ColumnType{BaseType: "int"}, NotNull: true},
			{Name: "comment", Type: postgresql.ColumnType{BaseType: "text"}},
			{Name: "created_at", Type: postgresql.ColumnType{BaseType: "timestamp"}, Default: "now()"},
		},
		TableConstraints: []postgresql.TableConstraint{
			{
				Type:      postgresql.ConstraintCheck,
				Name:      "valid_rating",
				CheckExpr: "rating >= 1 AND rating <= 5",
			},
			{
				Type:    postgresql.ConstraintUnique,
				Columns: []string{"order_id", "product_id"},
			},
		},
	}

	// Categories: schema-qualified, self-contained reference table
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

	// Product-Categories: many-to-many join with composite PK and FKs
	tableProductCategories := postgresql.CreateTableStmt{
		Name: "product_categories",
		Columns: []postgresql.ColumnDef{
			{Name: "product_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "category_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
		},
		TableConstraints: []postgresql.TableConstraint{
			{
				Type:    postgresql.ConstraintPrimaryKey,
				Columns: []string{"product_id", "category_id"},
			},
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"product_id"},
				RefTable:   "products",
				RefColumns: []string{"id"},
				OnDelete:   postgresql.FKCascade,
			},
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"category_id"},
				RefTable:   "inventory.categories",
				RefColumns: []string{"id"},
				OnDelete:   postgresql.FKCascade,
			},
		},
	}

	// Employees: self-referencing FK for manager hierarchy
	tableEmployees := postgresql.CreateTableStmt{
		Name: "employees",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 200}, NotNull: true},
			{Name: "email", Type: postgresql.ColumnType{BaseType: "varchar", Length: 255}, NotNull: true, IsUnique: true},
			{Name: "salary", Type: postgresql.ColumnType{BaseType: "decimal", Length: 12, Precision: 2}},
			{
				Name:       "manager_id",
				Type:       postgresql.ColumnType{BaseType: "bigint"},
				References: &postgresql.InlineFKRef{RefTable: "employees", RefColumns: []string{"id"}, OnDelete: postgresql.FKSetNull},
			},
		},
		TableConstraints: []postgresql.TableConstraint{
			{
				Type:      postgresql.ConstraintCheck,
				Name:      "positive_salary",
				CheckExpr: "salary > 0 OR salary IS NULL",
			},
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

	// ── Phase 1: Display the raw DDL AST ──────────────────────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 1 — RAW DDL AST")
	fmt.Println(strings.Repeat("═", 72))

	for index, statement := range statements {
		fmt.Printf("\n── Statement %d ──────────────────────────────────────\n\n", index+1)
		fmt.Println(formatAST(statement))
	}

	// ── Phase 2: Translate and display the canonical model ────────────────

	fmt.Println(strings.Repeat("═", 72))
	fmt.Println("PHASE 2 — TRANSLATED SCHEMA MODEL")
	fmt.Println(strings.Repeat("═", 72))

	model, err := postgresql.Translate(statements)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}

	summary := summarize(model)
	fmt.Println(summary)
	fmt.Println()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(model); err != nil {
		fmt.Fprintf(os.Stderr, "\nJSON error: %v\n", err)
		os.Exit(1)
	}
}

// formatAST returns a human-readable representation of a DDL statement.
func formatAST(statement postgresql.Stmt) string {
	switch stmt := statement.(type) {
	case postgresql.CreateEnumStmt:
		return fmt.Sprintf(
			"CREATE TYPE %s AS ENUM (%s);",
			qualifiedName(stmt.Schema, stmt.Name),
			quoteJoin(stmt.Values),
		)

	case postgresql.CreateTableStmt:
		var lines []string
		lines = append(lines, fmt.Sprintf("CREATE TABLE %s (", qualifiedName(stmt.Schema, stmt.Name)))

		for _, column := range stmt.Columns {
			lines = append(lines, fmt.Sprintf("  %s", formatColumnDef(column)))
		}
		for _, constraint := range stmt.TableConstraints {
			lines = append(lines, fmt.Sprintf("  %s", formatConstraint(constraint)))
		}
		lines = append(lines, ");")

		return strings.Join(lines, "\n")
	}

	return fmt.Sprintf("%T", statement)
}

// summarize produces a compact overview of the translated schema.
func summarize(model *schema.Model) string {
	var builder strings.Builder
	builder.WriteString(strings.Repeat("─", 72))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("  Tables:  %d\n", len(model.Tables)))
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

	builder.WriteString(strings.Repeat("─", 72))
	return builder.String()
}

func qualifiedName(schema, name string) string {
	if schema != "" {
		return schema + "." + name
	}
	return name
}

func quoteJoin(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = fmt.Sprintf("'%s'", value)
	}
	return strings.Join(quoted, ", ")
}

func formatColumnDef(column postgresql.ColumnDef) string {
	parts := []string{column.Name, formatColumnType(column.Type)}

	if column.NotNull {
		parts = append(parts, "NOT NULL")
	}
	if column.Default != "" {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", column.Default))
	}
	if column.IsPrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}
	if column.IsUnique {
		parts = append(parts, "UNIQUE")
	}
	if column.References != nil {
		fk := fmt.Sprintf("REFERENCES %s (%s)", column.References.RefTable, strings.Join(column.References.RefColumns, ", "))
		if column.References.OnDelete != "" && column.References.OnDelete != postgresql.FKNoAction {
			fk += fmt.Sprintf(" ON DELETE %s", column.References.OnDelete)
		}
		if column.References.OnUpdate != "" && column.References.OnUpdate != postgresql.FKNoAction {
			fk += fmt.Sprintf(" ON UPDATE %s", column.References.OnUpdate)
		}
		parts = append(parts, fk)
	}
	if column.Comment != "" {
		parts = append(parts, fmt.Sprintf("-- %s", column.Comment))
	}

	return strings.Join(parts, " ")
}

func formatColumnType(columnType postgresql.ColumnType) string {
	typeName := columnType.BaseType
	if columnType.Length > 0 {
		if columnType.Precision > 0 {
			typeName = fmt.Sprintf("%s(%d,%d)", typeName, columnType.Length, columnType.Precision)
		} else {
			typeName = fmt.Sprintf("%s(%d)", typeName, columnType.Length)
		}
	}
	if columnType.IsSerial {
		typeName = fmt.Sprintf("%s (SERIAL)", typeName)
	}
	if columnType.IsArray {
		typeName += "[]"
	}
	return typeName
}

func formatConstraint(constraint postgresql.TableConstraint) string {
	switch constraint.Type {
	case postgresql.ConstraintPrimaryKey:
		name := ""
		if constraint.Name != "" {
			name = fmt.Sprintf("CONSTRAINT %s ", constraint.Name)
		}
		return fmt.Sprintf("%sPRIMARY KEY (%s)", name, strings.Join(constraint.Columns, ", "))

	case postgresql.ConstraintForeignKey:
		name := ""
		if constraint.Name != "" {
			name = fmt.Sprintf("CONSTRAINT %s ", constraint.Name)
		}
		fk := fmt.Sprintf("%sFOREIGN KEY (%s) REFERENCES %s (%s)",
			name,
			strings.Join(constraint.Columns, ", "),
			constraint.RefTable,
			strings.Join(constraint.RefColumns, ", "),
		)
		if constraint.OnDelete != "" && constraint.OnDelete != postgresql.FKNoAction {
			fk += fmt.Sprintf(" ON DELETE %s", constraint.OnDelete)
		}
		if constraint.OnUpdate != "" && constraint.OnUpdate != postgresql.FKNoAction {
			fk += fmt.Sprintf(" ON UPDATE %s", constraint.OnUpdate)
		}
		return fk

	case postgresql.ConstraintUnique:
		name := ""
		if constraint.Name != "" {
			name = fmt.Sprintf("CONSTRAINT %s ", constraint.Name)
		}
		return fmt.Sprintf("%sUNIQUE (%s)", name, strings.Join(constraint.Columns, ", "))

	case postgresql.ConstraintCheck:
		name := ""
		if constraint.Name != "" {
			name = fmt.Sprintf("CONSTRAINT %s ", constraint.Name)
		}
		expr := constraint.CheckExpr
		if expr == "" {
			expr = constraint.Name
		}
		return fmt.Sprintf("%sCHECK (%s)", name, expr)

	default:
		return fmt.Sprintf("CONSTRAINT %s (unknown type)", constraint.Name)
	}
}
