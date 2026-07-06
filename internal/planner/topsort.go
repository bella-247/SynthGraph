package planner

import "synthgraph/internal/graph"

// topologicalSort performs Kahn's algorithm on the table nodes using
// EdgeKindReferences edges as the dependency relationship.
//
// An edge FROM table A TO table B means "A depends on B" (A references B via FK).
// Tables with zero outgoing references edges have no FK dependencies and
// can be generated first.
//
// Returns:
//   - ordered: table node IDs in valid generation order
//   - unresolved: table node IDs that never reached count 0 (cycles + blocked)
func topologicalSort(schemaGraph *graph.Graph, tableNodes map[string]*graph.Node) ([]string, []string) {
	var ordered, unresolved []string
	outCount := make(map[string]int, len(tableNodes))
	for nodeID := range tableNodes {
		outCount[nodeID] = 0
	}
	for _, edge := range schemaGraph.Edges {
		if edge.Kind == graph.EdgeKindReferences {
			if _, isTable := tableNodes[edge.From]; isTable {
				outCount[edge.From]++
			}
		}
	}

	queue := make([]string, 0, len(tableNodes))
	for nodeID, count := range outCount {
		if count == 0 {
			queue = append(queue, nodeID)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ordered = append(ordered, current)

		for _, edge := range schemaGraph.Edges {
			if edge.Kind != graph.EdgeKindReferences {
				continue
			}
			if edge.To != current {
				continue
			}
			dependentID := edge.From
			if _, isTable := tableNodes[dependentID]; !isTable {
				continue
			}
			outCount[dependentID]--
			if outCount[dependentID] == 0 {
				queue = append(queue, dependentID)
			}
		}
	}

	// Collect any node still with count > 0.
	for _, edge := range schemaGraph.Edges {
		if edge.Kind == graph.EdgeKindReferences {
			if _, isTable := tableNodes[edge.From]; isTable {
				if outCount[edge.From] > 0 {
					unresolved = append(unresolved, edge.From)
				}
			}
		}
	}

	unresolved = dedupeStrings(unresolved)
	return ordered, unresolved
}
