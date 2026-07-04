package postgresql

import "synthgraph/internal/schema"

// extractInlinePK collects column-level PRIMARY KEY flags into a PK column list.
func extractInlinePK(stmt CreateTableStmt) []string {
	var primaryKeys []string
	for _, column := range stmt.Columns {
		if column.IsPrimaryKey {
			primaryKeys = append(primaryKeys, column.Name)
		}
	}
	return primaryKeys
}

// extractInlineUniques collects column-level UNIQUE flags into unique-constraint lists.
func extractInlineUniques(stmt CreateTableStmt) [][]string {
	var uniques [][]string
	for _, column := range stmt.Columns {
		if column.IsUnique {
			uniques = append(uniques, []string{column.Name})
		}
	}
	return uniques
}

// extractTableConstraints processes table-level constraints and merges them
// into the already-collected column-level constraint lists.
func extractTableConstraints(stmt CreateTableStmt, existingPK []string, existingUniques [][]string) (fks []schema.ForeignKey, pkCols []string, uniques [][]string, checks []schema.CheckConstraint) {
	pkCols = existingPK
	uniques = existingUniques

	for _, tableConstraint := range stmt.TableConstraints {
		switch tableConstraint.Type {
		case ConstraintPrimaryKey:
			pkCols = append(pkCols, tableConstraint.Columns...)

		case ConstraintForeignKey:
			fks = append(fks, schema.ForeignKey{
				Columns:    tableConstraint.Columns,
				RefTable:   tableConstraint.RefTable,
				RefColumns: tableConstraint.RefColumns,
				OnDelete:   schema.FKAction(tableConstraint.OnDelete),
				OnUpdate:   schema.FKAction(tableConstraint.OnUpdate),
			})

		case ConstraintUnique:
			uniques = append(uniques, tableConstraint.Columns)

		case ConstraintCheck:
			checks = append(checks, schema.CheckConstraint{
				Name:       tableConstraint.Name,
				Expression: tableConstraint.CheckExpr,
			})
		}
	}

	return fks, pkCols, uniques, checks
}

// extractInlineFKs collects column-level REFERENCES constraints and returns
// them as schema.ForeignKey entries, which are equivalent to table-level FKs.
func extractInlineFKs(stmt CreateTableStmt) []schema.ForeignKey {
	var foreignKeys []schema.ForeignKey
	for _, column := range stmt.Columns {
		if column.References != nil {
			foreignKeys = append(foreignKeys, schema.ForeignKey{
				Columns:    []string{column.Name},
				RefTable:   column.References.RefTable,
				RefColumns: column.References.RefColumns,
				OnDelete:   schema.FKAction(column.References.OnDelete),
				OnUpdate:   schema.FKAction(column.References.OnUpdate),
			})
		}
	}
	return foreignKeys
}
