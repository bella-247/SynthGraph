package semantic

import (
	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// inferRelationshipKind translates the structural cardinality of a foreign key
// edge into a semantic RelationshipKind.
func inferRelationshipKind(edge *graph.Edge, context *InferenceContext) RelationshipKind {
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

	isNullable := isForeignKeyNullable(edge, foreignKeyMetadata, context.ColumnNullableIndex)

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
// foreign key constraint are nullable using the precomputed nullable index.
func isForeignKeyNullable(edge *graph.Edge, foreignKeyMetadata *graph.FKMetadata, columnNullableIndex map[string]map[string]bool) bool {
	nullableColumns, tableExists := columnNullableIndex[edge.From]
	if !tableExists {
		return false
	}
	for _, foreignKeyColumn := range foreignKeyMetadata.LocalColumns {
		if nullableColumns[foreignKeyColumn] {
			return true
		}
	}
	return false
}
