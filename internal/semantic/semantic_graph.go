package semantic

import "synthgraph/internal/graph"

// SemanticGraph is the output of the Semantic Layer.
//
// It wraps the original graph.Graph and enriches every node with inferred
// meaning. The original graph is always preserved unchanged via Source —
// any downstream system that needs raw structural data can always access it.
//
// # Determinism
//
// Building the same graph.Graph twice always produces an identical SemanticGraph.
// NodeList and Relationships are in the same fixed order as the source graph's
// NodeList and Edges respectively, ensuring stable rendering, diffing, and caching.
type SemanticGraph struct {
	// Nodes maps node ID → SemanticNode for O(1) lookup by ID.
	// IDs are identical to those in the source graph.Graph.
	Nodes map[string]*SemanticNode

	// NodeList is the ordered list of all semantic nodes in deterministic order,
	// matching the insertion order of the source graph's NodeList.
	NodeList []*SemanticNode

	// Relationships is the list of all semantic relationships, in the same order
	// as the source graph's Edges. Only EdgeKindReferences edges produce
	// relationships; structural edges (contains, uses_enum) are not included.
	Relationships []*SemanticRelationship

	// Source is the original graph.Graph this semantic graph was built from.
	// It is preserved without modification. Downstream AI and analysis layers
	// can traverse the source graph for any information not surfaced here.
	Source *graph.Graph
}

// SemanticNode wraps a graph.Node with inferred semantic properties.
//
// For NodeKindTable nodes, all semantic fields are populated. For NodeKindColumn
// and NodeKindEnum nodes, only the embedded Node is populated — semantic analysis
// at the column and enum level is reserved for a future deeper analysis pass.
type SemanticNode struct {
	// Node is the original graph node. All original fields (ID, Kind, Label, Data)
	// are accessible directly on this embedded struct.
	*graph.Node

	// Roles is the set of structural roles this table plays. A table may hold
	// multiple roles simultaneously — for example, an employees table can be
	// both Entity and Hierarchical. Roles is nil for non-table nodes.
	Roles []TableRole

	// Inferences is the full list of all conclusions the inference engine made
	// about this node, each with its confidence score and supporting evidence.
	// Rules are applied in registration order, and their inferences are appended
	// in that same order. Inferences is nil for non-table nodes.
	Inferences []Inference

	// Temporal describes which time-tracking columns were detected on this table.
	// Nil if no temporal columns were found, or if this is not a table node.
	Temporal *TemporalPattern

	// Audit describes which accountability-tracking columns were detected on this
	// table. Nil if no audit columns were found, or if this is not a table node.
	Audit *AuditPattern

	// IsHierarchical is true if this table has a self-referencing foreign key —
	// i.e., it references itself. False for non-table nodes.
	IsHierarchical bool

	// IsSoftDelete is true if a deleted_at column was detected on this table,
	// indicating that rows are logically deleted rather than physically removed.
	// False for non-table nodes.
	IsSoftDelete bool
}

// SemanticRelationship wraps a graph.Edge with an inferred relationship kind.
//
// Only EdgeKindReferences edges produce SemanticRelationships. Structural edges
// (EdgeKindContains, EdgeKindUsesEnum) represent composition within a table and
// are not semantic relationships between domain entities.
type SemanticRelationship struct {
	// Edge is the original graph edge. From, To, Kind, and Metadata (FKMetadata)
	// are all accessible directly.
	*graph.Edge

	// Kind is the inferred semantic nature of this relationship.
	// See RelationshipKind constants for the full vocabulary.
	Kind RelationshipKind
}

// HasRole returns true if this semantic node has been assigned the given role.
// This is a convenience method to avoid manually scanning the Roles slice.
func (semanticNode *SemanticNode) HasRole(role TableRole) bool {
	for _, assignedRole := range semanticNode.Roles {
		if assignedRole == role {
			return true
		}
	}
	return false
}

// newSemanticGraph creates and returns an empty, fully-initialised SemanticGraph.
func newSemanticGraph(sourceGraph *graph.Graph) *SemanticGraph {
	return &SemanticGraph{
		Nodes:         make(map[string]*SemanticNode, len(sourceGraph.Nodes)),
		NodeList:      make([]*SemanticNode, 0, len(sourceGraph.NodeList)),
		Relationships: make([]*SemanticRelationship, 0),
		Source:        sourceGraph,
	}
}
