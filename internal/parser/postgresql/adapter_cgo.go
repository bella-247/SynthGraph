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
	statements := preprocessSQL(text)
	var result []Stmt

	for _, statementText := range statements {
		tree, err := pg_query.Parse(statementText)
		if err != nil {
			return nil, fmt.Errorf("parse error: %w", err)
		}

		for _, rawStatement := range tree.Stmts {
			converted, err := convertNode(rawStatement.Stmt)
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
	switch typedNode := node.Node.(type) {
	case *pg_query.Node_CreateStmt:
		return convertCreateTable(typedNode.CreateStmt)
	case *pg_query.Node_CreateEnumStmt:
		return convertCreateEnum(typedNode.CreateEnumStmt)
	default:
		// Explicit error — never silently drop statements.
		// Add a handler above when supporting this node type.
		return nil, fmt.Errorf("unsupported statement type: %T", node.Node)
	}
}

func convertCreateEnum(statement *pg_query.CreateEnumStmt) (CreateEnumStmt, error) {
	// TypeName is a list of name parts, e.g. ['public', 'mood'] for CREATE TYPE public.mood AS ENUM
	nameParts := make([]string, 0, len(statement.TypeName))
	for _, nameNode := range statement.TypeName {
		if stringNode, ok := nameNode.Node.(*pg_query.Node_String_); ok {
			nameParts = append(nameParts, stringNode.String_.Sval)
		}
	}

	var schema, name string
	switch len(nameParts) {
	case 0:
		// No name parts — will be caught by validation
	case 1:
		name = nameParts[0]
	default:
		schema = nameParts[0]
		name = nameParts[1]
	}

	enumValues := make([]string, len(statement.Vals))
	for index, valueNode := range statement.Vals {
		if stringNode, ok := valueNode.Node.(*pg_query.Node_String_); ok {
			enumValues[index] = stringNode.String_.Sval
		}
	}
	return CreateEnumStmt{Schema: schema, Name: name, Values: enumValues}, nil
}

func convertCreateTable(statement *pg_query.CreateStmt) (CreateTableStmt, error) {
	table := CreateTableStmt{
		Name:        statement.Relation.Relname,
		IfNotExists: statement.IfNotExists,
	}
	if statement.Relation.Schemaname != "" {
		table.Schema = statement.Relation.Schemaname
	}

	for _, element := range statement.TableElts {
		switch typedElement := element.Node.(type) {
		case *pg_query.Node_ColumnDef:
			column, err := convertColumnDef(typedElement.ColumnDef)
			if err != nil {
				return table, err
			}
			table.Columns = append(table.Columns, column)

		case *pg_query.Node_Constraint:
			constraint := convertConstraint(typedElement.Constraint)
			if constraint != nil {
				table.TableConstraints = append(table.TableConstraints, *constraint)
			}
		}
	}

	return table, nil
}

func convertColumnDef(definition *pg_query.ColumnDef) (ColumnDef, error) {
	column := ColumnDef{
		Name: definition.Colname,
	}

	if definition.TypeName != nil {
		column.Type = convertTypeName(definition.TypeName)
	}

	for _, constraintNode := range definition.Constraints {
		if err := extractColumnConstraint(&column, constraintNode); err != nil {
			return column, fmt.Errorf("column %q: %w", definition.Colname, err)
		}
	}

	return column, nil
}

// extractColumnConstraint applies a single column-level constraint to the ColumnDef.
func extractColumnConstraint(column *ColumnDef, constraintNode *pg_query.Node) error {
	constraint, ok := constraintNode.Node.(*pg_query.Node_Constraint)
	if !ok {
		return nil
	}
	switch constraint.Constraint.Contype {
	case pg_query.ConstrType_CONSTR_NOTNULL:
		column.NotNull = true
	case pg_query.ConstrType_CONSTR_NULL:
		column.NotNull = false
	case pg_query.ConstrType_CONSTR_PRIMARY:
		column.IsPrimaryKey = true
	case pg_query.ConstrType_CONSTR_UNIQUE:
		column.IsUnique = true
	case pg_query.ConstrType_CONSTR_DEFAULT:
		column.Default = nodeToDefaultString(constraint.Constraint.RawExpr)

	case pg_query.ConstrType_CONSTR_FOREIGN:
		column.References = extractInlineFKRef(constraint.Constraint)
	}
	return nil
}

// nodeToDefaultString extracts the default expression as a string.
// Fast path for simple constants — falls back to pg_query.Deparse for complex expressions.
func nodeToDefaultString(node *pg_query.Node) string {
	if node == nil {
		return ""
	}

	// Fast path for simple constants (most common default values)
	switch typedNode := node.Node.(type) {
	case *pg_query.Node_AConst:
		switch constantValue := typedNode.AConst.Val.(type) {
		case *pg_query.A_Const_Ival:
			return fmt.Sprintf("%d", constantValue.Ival.Ival)
		case *pg_query.A_Const_Sval:
			return fmt.Sprintf("'%s'", constantValue.Sval.Sval)
		case *pg_query.A_Const_Boolval:
			if constantValue.Boolval.Boolval {
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

func convertConstraint(constraint *pg_query.Constraint) *TableConstraint {
	tableConstraint := &TableConstraint{Name: constraint.Conname}

	switch constraint.Contype {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		tableConstraint.Type = ConstraintPrimaryKey
		tableConstraint.Columns = extractNames(constraint.Keys)
		return tableConstraint

	case pg_query.ConstrType_CONSTR_UNIQUE:
		tableConstraint.Type = ConstraintUnique
		tableConstraint.Columns = extractNames(constraint.Keys)
		return tableConstraint

	case pg_query.ConstrType_CONSTR_FOREIGN:
		tableConstraint.Type = ConstraintForeignKey
		if len(constraint.FkAttrs) > 0 {
			tableConstraint.Columns = extractNames(constraint.FkAttrs)
		} else if len(constraint.Keys) > 0 {
			tableConstraint.Columns = extractNames(constraint.Keys)
		}
		if constraint.Pktable != nil {
			if constraint.Pktable.Schemaname != "" {
				tableConstraint.RefTable = constraint.Pktable.Schemaname + "." + constraint.Pktable.Relname
			} else {
				tableConstraint.RefTable = constraint.Pktable.Relname
			}
		}
		tableConstraint.RefColumns = extractNames(constraint.PkAttrs)
		tableConstraint.OnDelete = mapForeignKeyAction(constraint.FkDelAction)
		tableConstraint.OnUpdate = mapForeignKeyAction(constraint.FkUpdAction)
		return tableConstraint

	case pg_query.ConstrType_CONSTR_CHECK:
		tableConstraint.Type = ConstraintCheck
		tableConstraint.CheckExpr = nodeToDefaultString(constraint.RawExpr)
		return tableConstraint
	}

	return nil
}

// mapForeignKeyAction converts a pg_query FK action character to an FKAction.
//
//	'a' = NO ACTION, 'r' = RESTRICT, 'c' = CASCADE, 'n' = SET NULL, 'd' = SET DEFAULT
func mapForeignKeyAction(actionCode string) FKAction {
	if actionCode == "" {
		return FKNoAction
	}
	switch actionCode {
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

// extractInlineFKRef converts a column-level FOREIGN constraint from pg_query
// into our InlineFKRef representation, which the translator later normalizes
// to the same schema.ForeignKey as a table-level FOREIGN KEY constraint.
func extractInlineFKRef(constraint *pg_query.Constraint) *InlineFKRef {
	if constraint.Pktable == nil {
		return nil
	}

	ref := &InlineFKRef{}
	if constraint.Pktable.Schemaname != "" {
		ref.RefTable = constraint.Pktable.Schemaname + "." + constraint.Pktable.Relname
	} else {
		ref.RefTable = constraint.Pktable.Relname
	}
	ref.RefColumns = extractNames(constraint.PkAttrs)
	ref.OnDelete = mapForeignKeyAction(constraint.FkDelAction)
	ref.OnUpdate = mapForeignKeyAction(constraint.FkUpdAction)

	return ref
}

func convertTypeName(typeName *pg_query.TypeName) ColumnType {
	columnType := ColumnType{}

	nameParts := make([]string, 0, len(typeName.Names))
	for _, nameNode := range typeName.Names {
		if stringNode, ok := nameNode.Node.(*pg_query.Node_String_); ok {
			nameParts = append(nameParts, stringNode.String_.Sval)
		}
	}

	if len(nameParts) > 0 {
		// Skip leading "pg_catalog" and join remaining parts into a full type name.
		// This handles multi-word types like "double precision" that pg_query may
		// represent as ["pg_catalog", "double"] or ["pg_catalog", "double", "precision"].
		startIndex := 0
		if strings.ToLower(nameParts[0]) == "pg_catalog" {
			startIndex = 1
		}
		columnType.BaseType = strings.ToLower(strings.Join(nameParts[startIndex:], " "))
	} else {
		columnType.BaseType = "unknown"
	}

	if len(typeName.Typmods) > 0 {
		if modifier, ok := typeName.Typmods[0].Node.(*pg_query.Node_AConst); ok {
			if integerValue, ok := modifier.AConst.Val.(*pg_query.A_Const_Ival); ok {
				columnType.Length = int(integerValue.Ival.Ival)
			}
		}
		if len(typeName.Typmods) > 1 {
			if modifier, ok := typeName.Typmods[1].Node.(*pg_query.Node_AConst); ok {
				if integerValue, ok := modifier.AConst.Val.(*pg_query.A_Const_Ival); ok {
					columnType.Precision = int(integerValue.Ival.Ival)
				}
			}
		}
	}

	columnType.IsSerial = IsSerialType(columnType.BaseType)
	columnType.IsArray = len(typeName.ArrayBounds) > 0

	return columnType
}

func extractNames(nodes []*pg_query.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if stringNode, ok := node.Node.(*pg_query.Node_String_); ok {
			names = append(names, stringNode.String_.Sval)
		}
	}
	return names
}
