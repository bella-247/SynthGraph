package graph

// Graph is the canonical graph representation of a database schema.
//
// It is the central data structure of SynthGraph. Every downstream
// feature — Mermaid renderer, Draw.io exporter, AI documentation generator,
// dependency analyser — consumes the Graph rather than the raw schema.Model.
//
// # Determinism
//
// Building the same schema.Model twice always produces identical Graphs:
//   - Nodes appear in NodeList in a fixed order: tables first (in schema order),
//     then enums, then columns (per-table in column order).
//   - Edges appear in Edges in a fixed order: contains edges first, then
//     references edges, then uses_enum edges — each in schema order.
//
// This determinism is essential for reliable testing, diffing, caching, and
// version control of generated output.
//
// # Complexity
//
// Build runs in O(T + C + E) time where T = tables, C = columns, E = FK edges.
// All node lookups are O(1) via the Nodes map.
type Graph struct {
	Nodes    map[string]*Node `json:"nodes"`
	NodeList []*Node          `json:"node_list"`
	Edges    []*Edge          `json:"edges"`
}

// newGraph creates and returns an empty, fully-initialised Graph.
func newGraph() *Graph {
	return &Graph{
		Nodes:    make(map[string]*Node),
		NodeList: make([]*Node, 0),
		Edges:    make([]*Edge, 0),
	}
}

// addNode inserts a node into the graph, registering it in both the O(1)
// lookup map and the deterministically-ordered node list.
func (g *Graph) addNode(node *Node) {
	g.Nodes[node.ID] = node
	g.NodeList = append(g.NodeList, node)
}

// addEdge appends an edge to the graph's edge list.
func (g *Graph) addEdge(edge *Edge) {
	g.Edges = append(g.Edges, edge)
}

// HasNode returns true if a node with the given ID exists in the graph.
func (g *Graph) HasNode(nodeID string) bool {
	_, exists := g.Nodes[nodeID]
	return exists
}

// StringSet converts a string slice to a set map for O(1) lookups.
func StringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
