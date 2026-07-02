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
)

func main() {
	// ── Input: Build an e-commerce schema using our DDL AST types ──────────

	enumStatus := postgresql.CreateEnumStmt{
		Name:   "order_status",
		Values: []string{"pending", "confirmed", "shipped", "delivered", "cancelled"},
	}

	tableUsers := postgresql.CreateTableStmt{
		Name: "users",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "email", Type: postgresql.ColumnType{BaseType: "varchar", Length: 255}, NotNull: true, IsUnique: true},
			{Name: "is_active", Type: postgresql.ColumnType{BaseType: "boolean"}, Default: "true"},
		},
	}

	tableProducts := postgresql.CreateTableStmt{
		Name: "products",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 500}, NotNull: true},
			{Name: "price", Type: postgresql.ColumnType{BaseType: "decimal", Length: 10, Precision: 2}, NotNull: true},
			{Name: "stock", Type: postgresql.ColumnType{BaseType: "int"}, Default: "0", NotNull: true},
		},
	}

	tableOrders := postgresql.CreateTableStmt{
		Name: "orders",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "user_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "status", Type: postgresql.ColumnType{BaseType: "order_status"}, Default: "'pending'", NotNull: true},
			{Name: "total", Type: postgresql.ColumnType{BaseType: "decimal", Length: 12, Precision: 2}, NotNull: true},
		},
		TableConstraints: []postgresql.TableConstraint{
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"user_id"},
				RefTable:   "users",
				RefColumns: []string{"id"},
				OnDelete:   postgresql.FKNoAction,
				OnUpdate:   postgresql.FKCascade,
			},
		},
	}

	tableOrderItems := postgresql.CreateTableStmt{
		Name: "order_items",
		Columns: []postgresql.ColumnDef{
			{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
			{Name: "order_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "product_id", Type: postgresql.ColumnType{BaseType: "bigint"}, NotNull: true},
			{Name: "quantity", Type: postgresql.ColumnType{BaseType: "int"}, Default: "1", NotNull: true},
			{Name: "unit_price", Type: postgresql.ColumnType{BaseType: "decimal", Length: 10, Precision: 2}, NotNull: true},
		},
		TableConstraints: []postgresql.TableConstraint{
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"order_id"},
				RefTable:   "orders",
				RefColumns: []string{"id"},
			},
			{
				Type:       postgresql.ConstraintForeignKey,
				Columns:    []string{"product_id"},
				RefTable:   "products",
				RefColumns: []string{"id"},
			},
		},
	}

	statements := []postgresql.Stmt{
		enumStatus,
		tableUsers,
		tableProducts,
		tableOrders,
		tableOrderItems,
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
			"CREATE TYPE %s AS ENUM (%s)",
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
		return fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(constraint.Columns, ", "))

	case postgresql.ConstraintForeignKey:
		fk := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
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
		return fmt.Sprintf("UNIQUE (%s)", strings.Join(constraint.Columns, ", "))

	case postgresql.ConstraintCheck:
		return fmt.Sprintf("CHECK (%s)", constraint.Name)

	default:
		return fmt.Sprintf("CONSTRAINT %s (unknown type)", constraint.Name)
	}
}
