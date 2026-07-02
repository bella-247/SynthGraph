package graph

import "synthgraph/internal/schema"

// Build constructs a canonical Graph from the given schema.Model.
//
// The builder runs nine phases in a fixed order, each with a single
// responsibility:
//
//  1. Create an empty graph.
//  2. Add one NodeKindTable node per table (in schema order).
//  3. Add one NodeKindEnum node per enum (in schema order).
//  4. Add one NodeKindColumn node per column (table order, then column order).
//  5. Add EdgeKindContains edges: table → each of its columns.
//  6. Add EdgeKindReferences edges: child table → parent table, one per FK constraint.
//  7. Add EdgeKindReferencedBy edges: parent table → child table (reverse of references).
//  8. Add EdgeKindDependsOn edges: child table → parent table (impact analysis).
//  9. Add EdgeKindUsesEnum edges: column → enum, for columns whose type resolves to a known enum.
//
// Each FK edge includes inferred cardinality (one_to_one, one_to_many, many_to_many)
// derived from the child table's primary key.
//
// After all phases the graph is validated for internal consistency.
// If validation fails, Build returns a descriptive *ValidationError.
//
// Build runs in O(T + C + E) time where T = tables, C = columns, E = FK edges.
// It never performs repeated full-table scans.
func Build(model *schema.Model) (*Graph, error) {
	schemaGraph := newGraph()

	addTableNodes(schemaGraph, model)
	addEnumNodes(schemaGraph, model)
	addColumnNodes(schemaGraph, model)
	addContainsEdges(schemaGraph, model)
	addForeignKeyEdges(schemaGraph, model)
	addReverseReferenceEdges(schemaGraph, model)
	addEnumUsageEdges(schemaGraph, model)

	if err := validate(schemaGraph); err != nil {
		return nil, err
	}

	return schemaGraph, nil
}

// addTableNodes creates one NodeKindTable node for every table in the schema.
// Nodes are added in the same order as model.Tables, ensuring determinism.
func addTableNodes(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		node := &Node{
			ID:    tableNodeID(table.Name),
			Kind:  NodeKindTable,
			Label: table.Name,
			Data: TableData{
				Name:       table.Name,
				PrimaryKey: table.PrimaryKey,
				Unique:     table.Unique,
				Checks:     table.Checks,
			},
		}
		schemaGraph.addNode(node)
	}
}

// addEnumNodes creates one NodeKindEnum node for every enum type in the schema.
// Nodes are added in the same order as model.Enums, ensuring determinism.
func addEnumNodes(schemaGraph *Graph, model *schema.Model) {
	for _, enumType := range model.Enums {
		node := &Node{
			ID:    enumNodeID(enumType.Name),
			Kind:  NodeKindEnum,
			Label: enumType.Name,
			Data: EnumData{
				Values: enumType.Values,
			},
		}
		schemaGraph.addNode(node)
	}
}

// addColumnNodes creates one NodeKindColumn node for every column in every table.
// Nodes are added in table order, and within each table in column declaration order,
// ensuring determinism.
func addColumnNodes(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		for _, column := range table.Columns {
			node := &Node{
				ID:    columnNodeID(table.Name, column.Name),
				Kind:  NodeKindColumn,
				Label: column.Name,
				Data: ColumnData{
					Type:         column.Type,
					Length:       column.Length,
					Precision:    column.Precision,
					Nullable:     column.Nullable,
					Default:      column.Default,
					IsPrimaryKey: column.IsPrimaryKey,
				},
			}
			schemaGraph.addNode(node)
		}
	}
}

// addContainsEdges creates one EdgeKindContains edge from each table node
// to each of its column nodes. These edges model the "table owns column"
// relationship.
func addContainsEdges(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		fromTableID := tableNodeID(table.Name)
		for _, column := range table.Columns {
			toColumnID := columnNodeID(table.Name, column.Name)
			edge := &Edge{
				From: fromTableID,
				To:   toColumnID,
				Kind: EdgeKindContains,
			}
			schemaGraph.addEdge(edge)
		}
	}
}

// addForeignKeyEdges creates one EdgeKindReferences edge per FK constraint
// and infers the relationship's cardinality.
//
// Each edge connects the referencing (child) table node to the referenced
// (parent) table node. The column-level mapping is stored in FKMetadata on
// the edge, following the ER diagram convention that FK relationships connect
// tables — not individual columns.
//
// Composite foreign keys (multiple columns) produce exactly one edge with
// multiple columns listed in the FKMetadata, never multiple edges.
//
// Self-referencing foreign keys (a table referencing itself) are represented
// as an edge where From and To are the same node ID.
func addForeignKeyEdges(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		fromTableID := tableNodeID(table.Name)
		for _, foreignKey := range table.ForeignKeys {
			toTableID := tableNodeID(foreignKey.RefTable)
			cardinality := inferCardinality(foreignKey, table)
			edge := &Edge{
				From: fromTableID,
				To:   toTableID,
				Kind: EdgeKindReferences,
				Metadata: &FKMetadata{
					LocalColumns:   foreignKey.Columns,
					ForeignColumns: foreignKey.RefColumns,
					OnDelete:       foreignKey.OnDelete,
					OnUpdate:       foreignKey.OnUpdate,
					Cardinality:    cardinality,
				},
			}
			schemaGraph.addEdge(edge)
		}
	}
}

// inferCardinality determines the FK cardinality from the child table's
// primary key and the FK column set.
//
//   - one_to_one:   FK columns are exactly the child table's primary key.
//   - many_to_many: the child table's PK is composite and each PK column
//     is also an FK column across all FKs on the table. This detects
//     junction/associative tables.
//   - one_to_many:  everything else (the default).
//
// The function builds hash sets of the child's PK and FK columns, reusing
// the PK set for all checks on that table.
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
			// All PK columns are FK columns — this is a one-to-one relationship
			// from the parent's perspective.
			return CardinalityOneToOne
		}
	}

	// Check for many_to_many: the table is a junction table where every
	// PK column is also a foreign key column (across all FKs in the table).
	// We only check this when the table has a composite PK and the FK is
	// part of it — a junction table like product_categories with PK
	// (product_id, category_id) where each column is its own FK.
	if len(childTable.PrimaryKey) > 1 {
		// Build the union of all FK columns on the table.
		allFKColumnsOnTable := make(map[string]bool)
		for _, otherFK := range childTable.ForeignKeys {
			for _, col := range otherFK.Columns {
				allFKColumnsOnTable[col] = true
			}
		}

		// Every PK column must appear in at least one FK on this table.
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

// addReverseReferenceEdges creates EdgeKindReferencedBy and EdgeKindDependsOn
// edges for every foreign key relationship.
//
// EdgeKindReferencedBy connects the referenced (parent) table node to the
// referencing (child) table node — the reverse of EdgeKindReferences.
// It carries the same FKMetadata so renderers and analyzers have complete
// information when traversing in either direction.
//
// EdgeKindDependsOn connects the referencing (child) table node to the
// referenced (parent) table node — the same direction as EdgeKindReferences —
// but is explicitly intended for impact analysis: "if this table changes,
// which other tables are affected?" It also carries FKMetadata.
//
// Self-referencing FKs produce referenced_by and depends_on edges where
// From equals To.
func addReverseReferenceEdges(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		fromTableID := tableNodeID(table.Name)
		for _, foreignKey := range table.ForeignKeys {
			toTableID := tableNodeID(foreignKey.RefTable)
			cardinality := inferCardinality(foreignKey, table)

			// Build metadata once, share across all edges for this FK.
			fkMetadata := &FKMetadata{
				LocalColumns:   foreignKey.Columns,
				ForeignColumns: foreignKey.RefColumns,
				OnDelete:       foreignKey.OnDelete,
				OnUpdate:       foreignKey.OnUpdate,
				Cardinality:    cardinality,
			}

			// referenced_by: parent → child (reverse of references)
			referencedByEdge := &Edge{
				From:     toTableID,
				To:       fromTableID,
				Kind:     EdgeKindReferencedBy,
				Metadata: fkMetadata,
			}
			schemaGraph.addEdge(referencedByEdge)

			// depends_on: child → parent (impact analysis direction)
			dependsOnEdge := &Edge{
				From:     fromTableID,
				To:       toTableID,
				Kind:     EdgeKindDependsOn,
				Metadata: fkMetadata,
			}
			schemaGraph.addEdge(dependsOnEdge)
		}
	}
}

// addEnumUsageEdges creates one EdgeKindUsesEnum edge from each column node
// to the enum node whose name exactly matches the column's type name.
//
// A column "uses" an enum when its Type field is the name of a known enum
// type in the schema. Matching is exact (case-sensitive), which is correct
// because the translator already normalises enum type names during parsing.
//
// To avoid O(C × E) scanning, a hash set of known enum names is built first.
func addEnumUsageEdges(schemaGraph *Graph, model *schema.Model) {
	knownEnumNames := buildEnumNameIndex(model)

	for _, table := range model.Tables {
		for _, column := range table.Columns {
			if !knownEnumNames[column.Type] {
				continue
			}
			fromColumnID := columnNodeID(table.Name, column.Name)
			toEnumID := enumNodeID(column.Type)
			edge := &Edge{
				From: fromColumnID,
				To:   toEnumID,
				Kind: EdgeKindUsesEnum,
			}
			schemaGraph.addEdge(edge)
		}
	}
}

// buildEnumNameIndex returns a set of all known enum type names for O(1) lookup.
// This avoids repeated linear scans of model.Enums during edge construction.
func buildEnumNameIndex(model *schema.Model) map[string]bool {
	enumNameIndex := make(map[string]bool, len(model.Enums))
	for _, enumType := range model.Enums {
		enumNameIndex[enumType.Name] = true
	}
	return enumNameIndex
}
