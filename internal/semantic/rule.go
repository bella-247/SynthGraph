package semantic

import "synthgraph/internal/graph"

// Rule is the single interface that every inference rule must implement.
//
// A Rule examines a SemanticNode in the context of the full source graph and
// returns any Inferences it can make. Returning an empty slice is valid and
// simply means the rule found no applicable evidence for this node.
//
// # Design contract
//
//   - Rules must be pure functions. They must not mutate the node or the graph.
//   - Rules must be deterministic. The same node + graph must always produce
//     the same inferences in the same order.
//   - Rules must not panic. If evidence is absent, return an empty slice.
//   - Rules examine only NodeKindTable nodes. If passed a column or enum node,
//     they should return an empty slice.
//
// # Adding new rules
//
// Implement this interface and register the rule in the defaultRules() function
// in builder.go. No other files need to be modified. This makes domain-specific
// rule sets (healthcare, finance, DDD) trivially composable.
type Rule interface {
	// Name returns the short, machine-readable identifier for this rule.
	// Used for logging and debugging. Example: "junction_rule".
	Name() string

	// Apply examines the given semantic node in the context of the full source
	// graph and returns all inferences this rule can make. The sourceGraph
	// is the original graph.Graph, available for any structural traversal needed.
	Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference
}
