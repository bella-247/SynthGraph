package semantic

import (
	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// inferRelationshipKind translates the structural cardinality of a foreign key
// edge into a semantic RelationshipKind.
func inferRelationshipKind(edge *graph.Edge, sourceGraph *graph.Graph) RelationshipKind {
	// 1. Check for self-referencing hierarchy first.
	if edge.From == edge.To {
		return RelationshipKindHierarchy
	}

	foreignKeyMetadata, hasMetadata := edge.Metadata.(*graph.FKMetadata)
	if !hasMetadata {
		// Fallback for an edge missing metadata, though builder should prevent this.
		return RelationshipKindAssociation
	}

	// 2. Many-to-many cardinality directly maps to ManyToMany semantics.
	if foreignKeyMetadata.Cardinality == graph.CardinalityManyToMany {
		return RelationshipKindManyToMany
	}

	isNullable := isForeignKeyNullable(edge, foreignKeyMetadata, sourceGraph)

	// 3. Composition vs Association/Aggregation
	// If the foreign key is strictly required (NOT NULL on all source columns)
	// AND the database is instructed to CASCADE deletes, this is strong evidence
	// for existentially dependent Composition.
	isStrictlyRequired := !isNullable
	cascadesDeletes := foreignKeyMetadata.OnDelete == schema.FKCascade

	if isStrictlyRequired && cascadesDeletes {
		return RelationshipKindComposition
	}

	// 4. Association vs Aggregation
	// If the FK is nullable, it's an optional aggregation.
	if isNullable {
		return RelationshipKindAggregation
	}

	// If it's NOT NULL but doesn't cascade, it's a mandatory association.
	return RelationshipKindAssociation
}

// isForeignKeyNullable checks if any of the columns participating in the
// foreign key constraint are nullable.
func isForeignKeyNullable(edge *graph.Edge, foreignKeyMetadata *graph.FKMetadata, sourceGraph *graph.Graph) bool {
	localColumns := make(map[string]bool, len(foreignKeyMetadata.LocalColumns))
	for _, foreignKeyColumn := range foreignKeyMetadata.LocalColumns {
		localColumns[foreignKeyColumn] = true
	}

	for _, edgeFromTable := range sourceGraph.Edges {
		if edgeFromTable.Kind != graph.EdgeKindContains || edgeFromTable.From != edge.From {
			continue
		}

		columnNode, exists := sourceGraph.Nodes[edgeFromTable.To]
		if !exists {
			continue
		}

		if !localColumns[columnNode.Label] {
			continue
		}

		if columnData, ok := columnNode.Data.(graph.ColumnData); ok && columnData.Nullable {
			return true
		}
	}
	return false
}
