package graph

import "synthgraph/internal/schema"

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

			fkMetadata := &FKMetadata{
				LocalColumns:   foreignKey.Columns,
				ForeignColumns: foreignKey.RefColumns,
				OnDelete:       foreignKey.OnDelete,
				OnUpdate:       foreignKey.OnUpdate,
				Cardinality:    cardinality,
			}

			referencedByEdge := &Edge{
				From:     toTableID,
				To:       fromTableID,
				Kind:     EdgeKindReferencedBy,
				Metadata: fkMetadata,
			}
			schemaGraph.addEdge(referencedByEdge)

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
func buildEnumNameIndex(model *schema.Model) map[string]bool {
	enumNameIndex := make(map[string]bool, len(model.Enums))
	for _, enumType := range model.Enums {
		enumNameIndex[enumType.Name] = true
	}
	return enumNameIndex
}
