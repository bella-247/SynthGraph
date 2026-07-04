package graph

import "synthgraph/internal/schema"

// inferCardinality determines the FK cardinality from the child table's
// primary key and the FK column set.
//
//   - one_to_one:   FK columns are exactly the child table's primary key.
//   - many_to_many: the child table's PK is composite and each PK column
//     is also an FK column across all FKs on the table. This detects
//     junction/associative tables.
//   - one_to_many:  everything else (the default).
func inferCardinality(foreignKey schema.ForeignKey, childTable *schema.Table) Cardinality {
	fkColumnSet := make(map[string]bool, len(foreignKey.Columns))
	for _, column := range foreignKey.Columns {
		fkColumnSet[column] = true
	}

	pkColumnSet := make(map[string]bool, len(childTable.PrimaryKey))
	for _, column := range childTable.PrimaryKey {
		pkColumnSet[column] = true
	}

	// Check for one_to_one: FK columns are exactly the child's primary key.
	if len(foreignKey.Columns) == len(childTable.PrimaryKey) && len(foreignKey.Columns) > 0 {
		allFKInPK := true
		for _, column := range foreignKey.Columns {
			if !pkColumnSet[column] {
				allFKInPK = false
				break
			}
		}
		if allFKInPK {
			return CardinalityOneToOne
		}
	}

	// Check for many_to_many: the table is a junction table where every
	// PK column is also a foreign key column (across all FKs in the table).
	if len(childTable.PrimaryKey) > 1 {
		allFKColumnsOnTable := make(map[string]bool)
		for _, otherFK := range childTable.ForeignKeys {
			for _, col := range otherFK.Columns {
				allFKColumnsOnTable[col] = true
			}
		}

		allPKColumnsAreFKs := true
		for _, pkColumn := range childTable.PrimaryKey {
			if !allFKColumnsOnTable[pkColumn] {
				allPKColumnsAreFKs = false
				break
			}
		}
		if allPKColumnsAreFKs {
			return CardinalityManyToMany
		}
	}

	return CardinalityOneToMany
}
