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

	metadata, hasMetadata := edge.Metadata.(*graph.FKMetadata)
	if !hasMetadata {
		// Fallback for an edge missing metadata, though builder should prevent this.
		return RelationshipKindAssociation
	}

	// 2. Many-to-many cardinality directly maps to ManyToMany semantics.
	if metadata.Cardinality == graph.CardinalityManyToMany {
		return RelationshipKindManyToMany
	}

	isNullable := isForeignKeyNullable(edge, metadata, sourceGraph)

	// 3. Composition vs Association/Aggregation
	// If the foreign key is strictly required (NOT NULL on all source columns)
	// AND the database is instructed to CASCADE deletes, this is strong evidence
	// for existentially dependent Composition.
	isStrictlyRequired := !isNullable
	cascadesDeletes := metadata.OnDelete == schema.FKCascade

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
func isForeignKeyNullable(edge *graph.Edge, metadata *graph.FKMetadata, sourceGraph *graph.Graph) bool {
	localCols := make(map[string]bool, len(metadata.LocalColumns))
	for _, fkCol := range metadata.LocalColumns {
		localCols[fkCol] = true
	}

	for _, edgeFromTable := range sourceGraph.Edges {
		if edgeFromTable.Kind != graph.EdgeKindContains || edgeFromTable.From != edge.From {
			continue
		}

		colNode, exists := sourceGraph.Nodes[edgeFromTable.To]
		if !exists {
			continue
		}

		if !localCols[colNode.Label] {
			continue
		}

		if colData, ok := colNode.Data.(graph.ColumnData); ok && colData.Nullable {
			return true
		}
	}
	return false
}
