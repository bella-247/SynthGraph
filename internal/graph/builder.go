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
