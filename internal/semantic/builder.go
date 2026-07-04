package semantic

import "synthgraph/internal/graph"

// Build consumes a graph.Graph and produces a SemanticGraph by applying all
// registered inference rules to every node.
//
// The source graph is never modified. Building the same graph twice always
// produces an identical, deterministic SemanticGraph.
func Build(sourceGraph *graph.Graph) (*SemanticGraph, error) {
	semanticGraph := newSemanticGraph(sourceGraph)
	rules := defaultRules()

	// 0. Precompute all edge-based indexes so rules never scan the graph directly.
	context := newInferenceContext(sourceGraph)

	// 1. Initialise all semantic nodes (copying from source graph).
	// We iterate over the stable NodeList to ensure determinism.
	for _, sourceNode := range sourceGraph.NodeList {
		semanticNode := &SemanticNode{
			Node:       sourceNode,
			Roles:      make([]TableRole, 0),
			Inferences: make([]Inference, 0),
		}
		semanticGraph.Nodes[semanticNode.ID] = semanticNode
		semanticGraph.NodeList = append(semanticGraph.NodeList, semanticNode)
	}

	// 2. Apply all inference rules to every node.
	for _, semanticNode := range semanticGraph.NodeList {
		// Only run inference on table nodes.
		if semanticNode.Kind != graph.NodeKindTable {
			continue
		}

		// Apply each rule in registration order.
		for _, rule := range rules {
			inferences := rule.Apply(semanticNode, context)
			for _, inference := range inferences {
				semanticNode.Inferences = append(semanticNode.Inferences, inference)
				applyInferenceToNode(semanticNode, inference)
			}
		}
	}

	// 3. Infer relationships from edges.
	for _, sourceEdge := range sourceGraph.Edges {
		// We only assign semantic meaning to structural foreign key edges.
		if sourceEdge.Kind != graph.EdgeKindReferences {
			continue
		}

		relationship := &SemanticRelationship{
			Edge: sourceEdge,
			Kind: inferRelationshipKind(sourceEdge, context),
		}
		semanticGraph.Relationships = append(semanticGraph.Relationships, relationship)
	}

	return semanticGraph, nil
}

// defaultRules returns the standard set of inference rules used by SynthGraph.
// They are returned in a specific order to ensure deterministic inference
// processing, although their logic should ideally be order-independent.
func defaultRules() []Rule {
	return []Rule{
		&JunctionRule{},
		&HierarchyRule{},
		&LookupRule{},
		&TransactionalRule{},
		&EntityRule{},
		&TemporalRule{},
		&AuditRule{},
	}
}

// applyInferenceToNode mutates the semantic node to reflect the given inference.
// This bridges the generic Inference system to the strongly-typed struct fields
// on SemanticNode for easier downstream consumption.
func applyInferenceToNode(semanticNode *SemanticNode, inference Inference) {
	switch TableRole(inference.Kind) {
	case TableRoleEntity, TableRoleJunction, TableRoleLookup, TableRoleTransactional, TableRoleHierarchical:
		if !semanticNode.HasRole(TableRole(inference.Kind)) {
			semanticNode.Roles = append(semanticNode.Roles, TableRole(inference.Kind))
		}
	}

	// Special handling for non-role inferences:
	switch inference.Kind {
	case "hierarchical":
		semanticNode.IsHierarchical = true
	case "temporal":
		applyTemporalInference(semanticNode, inference)
	case "soft_delete":
		semanticNode.IsSoftDelete = true
	case "audit":
		applyAuditInference(semanticNode, inference)
	}
}

func applyTemporalInference(semanticNode *SemanticNode, inference Inference) {
	if semanticNode.Temporal == nil {
		semanticNode.Temporal = &TemporalPattern{}
	}
	for _, evidence := range inference.Evidence {
		switch evidence {
		case "has created_at":
			semanticNode.Temporal.HasCreatedAt = true
		case "has updated_at":
			semanticNode.Temporal.HasUpdatedAt = true
		case "has deleted_at":
			semanticNode.Temporal.HasDeletedAt = true
		}
	}
}

func applyAuditInference(semanticNode *SemanticNode, inference Inference) {
	if semanticNode.Audit == nil {
		semanticNode.Audit = &AuditPattern{}
	}
	for _, evidence := range inference.Evidence {
		switch evidence {
		case "has created_by":
			semanticNode.Audit.HasCreatedBy = true
		case "has updated_by":
			semanticNode.Audit.HasUpdatedBy = true
		case "has deleted_by":
			semanticNode.Audit.HasDeletedBy = true
		}
	}
}
