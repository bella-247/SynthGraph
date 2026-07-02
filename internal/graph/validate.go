package graph

import (
	"fmt"
	"strings"
)

// ValidationError describes a structural violation found during graph validation.
// It implements the error interface and always contains a human-readable message
// that names the exact problem, making it easy to diagnose bugs in the builder.
type ValidationError struct {
	// Message describes the exact violation found during validation.
	Message string
}

// Error implements the error interface.
func (validationError *ValidationError) Error() string {
	return "graph validation error: " + validationError.Message
}

// validate checks the graph for internal consistency and returns the first
// violation found, or nil if the graph is valid.
//
// Checks performed (in order):
//  1. Node list and node map are in sync (no duplicate node IDs silently overwrote entries).
//  2. Every edge's From and To refer to nodes that exist in the graph.
//  3. No two edges are structurally identical.
func validate(schemaGraph *Graph) error {
	if err := checkDuplicateNodes(schemaGraph); err != nil {
		return err
	}
	if err := checkBrokenEdgeReferences(schemaGraph); err != nil {
		return err
	}
	if err := checkDuplicateEdges(schemaGraph); err != nil {
		return err
	}
	return nil
}

// checkDuplicateNodes verifies that the ordered node list and the node lookup map
// contain the same number of entries.
//
// If the graph was accidentally built with two nodes sharing the same ID, the map
// would silently overwrite the older entry while the list would retain both,
// causing this check to fail. This guards against bugs in node ID generation.
func checkDuplicateNodes(schemaGraph *Graph) error {
	if len(schemaGraph.Nodes) != len(schemaGraph.NodeList) {
		return &ValidationError{
			Message: fmt.Sprintf(
				"node list has %d entries but node map has %d — duplicate node IDs detected",
				len(schemaGraph.NodeList), len(schemaGraph.Nodes),
			),
		}
	}
	return nil
}

// checkBrokenEdgeReferences verifies that every edge's From and To node IDs
// each refer to a node that exists in the graph.
//
// A broken reference indicates a bug in the builder — for example, an FK edge
// was added before the referenced table node was created.
func checkBrokenEdgeReferences(schemaGraph *Graph) error {
	for _, edge := range schemaGraph.Edges {
		if !schemaGraph.HasNode(edge.From) {
			return &ValidationError{
				Message: fmt.Sprintf(
					"edge (kind: %q) references unknown source node %q",
					edge.Kind, edge.From,
				),
			}
		}
		if !schemaGraph.HasNode(edge.To) {
			return &ValidationError{
				Message: fmt.Sprintf(
					"edge (kind: %q) references unknown destination node %q",
					edge.Kind, edge.To,
				),
			}
		}
	}
	return nil
}

// checkDuplicateEdges verifies that no two edges in the graph are structurally identical.
//
// Two edges are considered duplicates when they share the same From, To, Kind,
// and — for FK-related edges — the same set of local columns. This intentionally
// allows multiple distinct FK constraints between the same two tables as long as
// they map different sets of columns.
func checkDuplicateEdges(schemaGraph *Graph) error {
	seenEdgeKeys := make(map[string]bool, len(schemaGraph.Edges))
	for _, edge := range schemaGraph.Edges {
		edgeKey := buildEdgeKey(edge)
		if seenEdgeKeys[edgeKey] {
			return &ValidationError{
				Message: fmt.Sprintf(
					"duplicate edge from %q to %q (kind: %q)",
					edge.From, edge.To, edge.Kind,
				),
			}
		}
		seenEdgeKeys[edgeKey] = true
	}
	return nil
}

// buildEdgeKey produces a unique string key for an edge used to detect duplicates.
//
// For FK-related edges (references, referenced_by, depends_on), the local columns
// are appended to the key so that two different FK constraints between the same
// pair of tables (mapping different columns) are correctly treated as distinct,
// non-duplicate edges.
func buildEdgeKey(edge *Edge) string {
	localColumnsKey := ""
	if edge.Kind == EdgeKindReferences || edge.Kind == EdgeKindReferencedBy || edge.Kind == EdgeKindDependsOn {
		if fkMetadata, ok := edge.Metadata.(*FKMetadata); ok {
			localColumnsKey = strings.Join(fkMetadata.LocalColumns, ",")
		}
	}
	return fmt.Sprintf("%s|%s|%s|%s", edge.From, edge.To, string(edge.Kind), localColumnsKey)
}
