// Demo program: constructs DDL AST directly (no SQL parser needed)
// and shows the translated schema.Schema output as JSON.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"synthgraph/internal/parser/postgresql"
)

func main() {
	// Build a simple e-commerce schema using our DDL AST helpers
	stmts := []postgresql.Stmt{
		// 1. Enum type
		postgresql.CreateEnumStmt{
			Name:   "order_status",
			Values: []string{"pending", "confirmed", "shipped", "delivered", "cancelled"},
		},

		// 2. Users table
		postgresql.CreateTableStmt{
			Name: "users",
			Columns: []postgresql.ColumnDef{
				{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
				{Name: "email", Type: postgresql.ColumnType{BaseType: "varchar", Length: 255}, NotNull: true, IsUnique: true},
				{Name: "is_active", Type: postgresql.ColumnType{BaseType: "boolean"}, Default: "true"},
			},
		},

		// 3. Products table
		postgresql.CreateTableStmt{
			Name: "products",
			Columns: []postgresql.ColumnDef{
				{Name: "id", Type: postgresql.ColumnType{BaseType: "bigserial"}, IsPrimaryKey: true},
				{Name: "name", Type: postgresql.ColumnType{BaseType: "varchar", Length: 500}, NotNull: true},
				{Name: "price", Type: postgresql.ColumnType{BaseType: "decimal", Length: 10, Precision: 2}, NotNull: true},
				{Name: "stock", Type: postgresql.ColumnType{BaseType: "int"}, Default: "0", NotNull: true},
			},
		},

		// 4. Orders table (references users)
		postgresql.CreateTableStmt{
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
				},
			},
		},

		// 5. Order items (composite PK, references orders + products)
		postgresql.CreateTableStmt{
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
		},
	}

	// Translate to canonical schema
	schema, err := postgresql.Translate(stmts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print as pretty JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		fmt.Fprintf(os.Stderr, "JSON error: %v\n", err)
		os.Exit(1)
	}
}
