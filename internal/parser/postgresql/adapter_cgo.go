//go:build cgo

package postgresql

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v5"
)

func init() {
	parseSQL = parseWithPgQuery
}

func parseWithPgQuery(text string) ([]Stmt, error) {
	stmts := preprocessSQL(text)
	var result []Stmt

	for _, stmtText := range stmts {
		tree, err := pg_query.Parse(stmtText)
		if err != nil {
			return nil, fmt.Errorf("parse error: %w", err)
		}

		for _, rawStmt := range tree.Stmts {
			converted, err := convertNode(rawStmt.Stmt)
			if err != nil {
				return nil, err
			}
			if converted != nil {
				result = append(result, converted)
			}
		}
	}

	return result, nil
}

func convertNode(node *pg_query.Node) (Stmt, error) {
	switch n := node.Node.(type) {
	case *pg_query.Node_CreateStmt:
		return convertCreateTable(n.CreateStmt)
	case *pg_query.Node_CreateEnumStmt:
		return convertCreateEnum(n.CreateEnumStmt)
	default:
		// Explicit error — never silently drop statements.
		// Add a handler above when supporting this node type.
		return nil, fmt.Errorf("unsupported statement type: %T", node.Node)
	}
}

func convertCreateEnum(stmt *pg_query.CreateEnumStmt) (CreateEnumStmt, error) {
	// TypeName is a list of name parts, e.g. ['public', 'mood'] for CREATE TYPE public.mood AS ENUM
	parts := make([]string, 0, len(stmt.TypeName))
	for _, n := range stmt.TypeName {
		if s, ok := n.Node.(*pg_query.Node_String_); ok {
			parts = append(parts, s.String_.Sval)
		}
	}

	var schema, name string
	switch len(parts) {
	case 0:
		// No name parts — will be caught by validation
	case 1:
		name = parts[0]
	default:
		schema = parts[0]
		name = parts[1]
	}

	vals := make([]string, len(stmt.Vals))
	for i, v := range stmt.Vals {
		if s, ok := v.Node.(*pg_query.Node_String_); ok {
			vals[i] = s.String_.Sval
		}
	}
	return CreateEnumStmt{Schema: schema, Name: name, Values: vals}, nil
}

func convertCreateTable(stmt *pg_query.CreateStmt) (CreateTableStmt, error) {
	ct := CreateTableStmt{
		Name:        stmt.Relation.Relname,
		IfNotExists: stmt.IfNotExists,
	}
	if stmt.Relation.Schemaname != "" {
		ct.Schema = stmt.Relation.Schemaname
	}

	for _, elt := range stmt.TableElts {
		switch e := elt.Node.(type) {
		case *pg_query.Node_ColumnDef:
			col, err := convertColumnDef(e.ColumnDef)
			if err != nil {
				return ct, err
			}
			ct.Columns = append(ct.Columns, col)

		case *pg_query.Node_Constraint:
			tc := convertConstraint(e.Constraint)
			if tc != nil {
				ct.TableConstraints = append(ct.TableConstraints, *tc)
			}
		}
	}

	return ct, nil
}

func convertColumnDef(def *pg_query.ColumnDef) (ColumnDef, error) {
	col := ColumnDef{
		Name: def.Colname,
	}

	if def.TypeName != nil {
		col.Type = convertTypeName(def.TypeName)
	}

	for _, c := range def.Constraints {
		if err := extractColumnConstraint(&col, c); err != nil {
			return col, fmt.Errorf("column %q: %w", def.Colname, err)
		}
	}

	return col, nil
}

// extractColumnConstraint applies a single column-level constraint to the ColumnDef.
func extractColumnConstraint(col *ColumnDef, c *pg_query.Node) error {
	constraint, ok := c.Node.(*pg_query.Node_Constraint)
	if !ok {
		return nil
	}
	switch constraint.Constraint.Contype {
	case pg_query.ConstrType_CONSTR_NOTNULL:
		col.NotNull = true
	case pg_query.ConstrType_CONSTR_NULL:
		col.NotNull = false
	case pg_query.ConstrType_CONSTR_PRIMARY:
		col.IsPrimaryKey = true
	case pg_query.ConstrType_CONSTR_UNIQUE:
		col.IsUnique = true
	case pg_query.ConstrType_CONSTR_DEFAULT:
		col.Default = nodeToDefaultString(constraint.Constraint.RawExpr)
	}
	return nil
}

// nodeToDefaultString extracts the default expression as a string.
// Delegates all non-trivial cases to pg_query.Deparse to avoid duplicating
// PostgreSQL's expression-to-SQL logic.
func nodeToDefaultString(node *pg_query.Node) string {
	if node == nil {
		return ""
	}

	// Fast path for simple constants (most common case)
	switch n := node.Node.(type) {
	case *pg_query.Node_AConst:
		switch v := n.AConst.Val.(type) {
		case *pg_query.A_Const_Ival:
			return fmt.Sprintf("%d", v.Ival.Ival)
		case *pg_query.A_Const_Sval:
			return fmt.Sprintf("'%s'", v.Sval.Sval)
		case *pg_query.A_Const_Boolval:
			if v.Boolval.Boolval {
				return "true"
			}
			return "false"
		}
	}

	// Fallback: let PostgreSQL's own deparser handle complex expressions
	// (function calls, type casts, operators, etc.)
	result, err := pg_query.Deparse(&pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{{Stmt: node}},
	})
	if err != nil {
		return ""
	}
	return result
}

func convertConstraint(c *pg_query.Constraint) *TableConstraint {
	tc := &TableConstraint{Name: c.Conname}

	switch c.Contype {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		tc.Type = ConstraintPrimaryKey
		tc.Columns = extractNames(c.Keys)
		return tc

	case pg_query.ConstrType_CONSTR_UNIQUE:
		tc.Type = ConstraintUnique
		tc.Columns = extractNames(c.Keys)
		return tc

	case pg_query.ConstrType_CONSTR_FOREIGN:
		tc.Type = ConstraintForeignKey
		if len(c.FkAttrs) > 0 {
			tc.Columns = extractNames(c.FkAttrs)
		} else if len(c.Keys) > 0 {
			tc.Columns = extractNames(c.Keys)
		}
		if c.Pktable != nil {
			if c.Pktable.Schemaname != "" {
				tc.RefTable = c.Pktable.Schemaname + "." + c.Pktable.Relname
			} else {
				tc.RefTable = c.Pktable.Relname
			}
		}
		tc.RefColumns = extractNames(c.PkAttrs)
		tc.OnDelete = fkActionString(c.FkDelAction)
		tc.OnUpdate = fkActionString(c.FkUpdAction)
		return tc

	case pg_query.ConstrType_CONSTR_CHECK:
		tc.Type = ConstraintCheck
		return tc
	}

	return nil
}

// fkActionString converts a pg_query FK action character to FKAction.
//   'a' = NO ACTION, 'r' = RESTRICT, 'c' = CASCADE, 'n' = SET NULL, 'd' = SET DEFAULT
func fkActionString(c string) FKAction {
	if c == "" {
		return FKNoAction
	}
	switch c {
	case "a":
		return FKNoAction
	case "r":
		return FKRestrict
	case "c":
		return FKCascade
	case "n":
		return FKSetNull
	case "d":
		return FKSetDefault
	default:
		return FKNoAction
	}
}

func convertTypeName(typeName *pg_query.TypeName) ColumnType {
	ct := ColumnType{}

	parts := make([]string, 0, len(typeName.Names))
	for _, n := range typeName.Names {
		if s, ok := n.Node.(*pg_query.Node_String_); ok {
			parts = append(parts, s.String_.Sval)
		}
	}

	if len(parts) > 0 {
		ct.BaseType = strings.ToLower(parts[len(parts)-1])
	} else {
		ct.BaseType = "unknown"
	}

	if len(typeName.Typmods) > 0 {
		if mod, ok := typeName.Typmods[0].Node.(*pg_query.Node_AConst); ok {
			if i, ok := mod.AConst.Val.(*pg_query.A_Const_Ival); ok {
				ct.Length = int(i.Ival.Ival)
			}
		}
		if len(typeName.Typmods) > 1 {
			if mod, ok := typeName.Typmods[1].Node.(*pg_query.Node_AConst); ok {
				if i, ok := mod.AConst.Val.(*pg_query.A_Const_Ival); ok {
					ct.Precision = int(i.Ival.Ival)
				}
			}
		}
	}

	ct.IsSerial = IsSerialType(ct.BaseType)
	ct.IsArray = len(typeName.ArrayBounds) > 0

	return ct
}

func extractNames(nodes []*pg_query.Node) []string {
	result := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if s, ok := n.Node.(*pg_query.Node_String_); ok {
			result = append(result, s.String_.Sval)
		}
	}
	return result
}

