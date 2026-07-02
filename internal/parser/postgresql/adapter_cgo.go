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
	default:
		return nil, nil
	}
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
		switch constraint := c.Node.(type) {
		case *pg_query.Node_Constraint:
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
		}
	}

	return col, nil
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

// fkActionString converts a pg_query FK action character to a human-readable string.
//   'a' = NO ACTION, 'r' = RESTRICT, 'c' = CASCADE, 'n' = SET NULL, 'd' = SET DEFAULT
func fkActionString(c string) string {
	if c == "" {
		return "NO ACTION"
	}
	switch c {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return "NO ACTION"
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

func nodeToDefaultString(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
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
	case *pg_query.Node_FuncCall:
		parts := make([]string, 0, len(n.FuncCall.Funcname))
		for _, p := range n.FuncCall.Funcname {
			if s, ok := p.Node.(*pg_query.Node_String_); ok {
				parts = append(parts, s.String_.Sval)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, ".") + "()"
		}
	case *pg_query.Node_TypeCast:
		result, err := pg_query.Deparse(&pg_query.ParseResult{
			Stmts: []*pg_query.RawStmt{{Stmt: node}},
		})
		if err != nil {
			return ""
		}
		return result
	}
	result, err := pg_query.Deparse(&pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{{Stmt: node}},
	})
	if err != nil {
		return ""
	}
	return result
}
