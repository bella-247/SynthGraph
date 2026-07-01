package postgresql

import (
	"fmt"
	"strings"

	"synthgraph/internal/schema"
)

// Translate converts our PostgreSQL DDL AST into the canonical schema.Schema.
//
// This is the heart of the translator:
//   - One function per DDL construct
//   - Column-level PK/UNIQUE → merged into table-level constraints
//   - SERIAL → INT/BIGINT + IsSerial flag
//   - Defaults stored as raw strings
//   - Names are unquoted and normalized
func Translate(stmts []Stmt) (*schema.Schema, error) {
	s := &schema.Schema{}

	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case CreateTableStmt:
			t, err := translateCreateTable(stmt)
			if err != nil {
				return nil, fmt.Errorf("table %s: %w", stmt.Name, err)
			}
			s.Tables = append(s.Tables, *t)

		case CreateEnumStmt:
			s.Enums = append(s.Enums, schema.EnumType{
				Name:   stmt.Name,
				Values: stmt.Values,
			})
		}
	}

	return s, nil
}

func translateCreateTable(stmt CreateTableStmt) (*schema.Table, error) {
	t := &schema.Table{}
	if stmt.Schema != "" {
		t.Name = stmt.Schema + "." + stmt.Name
	} else {
		t.Name = stmt.Name
	}

	// Phase 1: extract columns
	for _, col := range stmt.Columns {
		c := translateColumn(col)
		t.Columns = append(t.Columns, c)
	}

	// Phase 2: collect column-level constraints into table-level constraints
	var pkCols []string
	var uniqueCols [][]string

	for _, col := range stmt.Columns {
		if col.IsPrimaryKey {
			pkCols = append(pkCols, col.Name)
		}
		if col.IsUnique {
			uniqueCols = append(uniqueCols, []string{col.Name})
		}
	}

	// Phase 3: resolve table-level constraints
	for _, tc := range stmt.TableConstraints {
		switch tc.Type {
		case ConstraintPrimaryKey:
			pkCols = append(pkCols, tc.Columns...)

		case ConstraintForeignKey:
			t.ForeignKeys = append(t.ForeignKeys, schema.ForeignKey{
				Columns:    tc.Columns,
				RefTable:   tc.RefTable,
				RefColumns: tc.RefColumns,
				OnDelete:   strings.ToUpper(tc.OnDelete),
				OnUpdate:   strings.ToUpper(tc.OnUpdate),
			})

		case ConstraintUnique:
			uniqueCols = append(uniqueCols, tc.Columns)

		case ConstraintCheck:
			// V1: parsed, stored for later use by the validator
		}
	}

	// Phase 4: deduplicate and set
	t.PrimaryKey = dedupe(pkCols)

	// Mark PK columns in the Column struct
	pkSet := make(map[string]bool, len(t.PrimaryKey))
	for _, p := range t.PrimaryKey {
		pkSet[p] = true
	}
	for i := range t.Columns {
		if pkSet[t.Columns[i].Name] {
			t.Columns[i].IsPrimaryKey = true
		}
	}

	// Deduplicate uniques already covered by PK
	t.Unique = dedupeUniques(uniqueCols, t.PrimaryKey)

	return t, nil
}

func translateColumn(col ColumnDef) schema.Column {
	c := schema.Column{
		Name: col.Name,
	}

	baseType := strings.ToLower(col.Type.BaseType)
	abstractType := NormalizeType(baseType)

	// Handle SERIAL types: SERIAL4 → INT, SERIAL8 → BIGINT
	if col.Type.IsSerial || IsSerialType(baseType) {
		switch baseType {
		case "bigserial", "serial8":
			abstractType = TypeBigInt
		case "smallserial", "serial2":
			abstractType = TypeSmallInt
		default:
			abstractType = TypeInt
		}
	}

	// Unknown type → treat as enum reference
	if abstractType == TypeUnknown {
		abstractType = TypeEnum
	}

	c.Type = string(abstractType)
	if abstractType == TypeEnum {
		c.Type = col.Type.BaseType
	}

	// Nullability: default is nullable unless NOT NULL is set
	c.Nullable = !col.NotNull

	// Default: store as raw string
	if col.Default != "" {
		d := col.Default
		c.Default = &d
	}

	return c
}

// dedupe removes duplicate strings preserving order.
func dedupe(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// dedupeUniques removes unique constraints already covered by the PK.
func dedupeUniques(uniques [][]string, pk []string) [][]string {
	pkSet := make(map[string]bool, len(pk))
	for _, v := range pk {
		pkSet[v] = true
	}

	isPkCovered := func(cols []string) bool {
		for _, c := range cols {
			if !pkSet[c] {
				return false
			}
		}
		return len(cols) > 0
	}

	seen := make(map[string]bool)
	result := make([][]string, 0)

	for _, u := range uniques {
		if isPkCovered(u) {
			continue
		}
		key := strings.Join(u, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, u)
	}

	return result
}
