package semantic

import "synthgraph/internal/graph"

// InferenceContext carries the source graph and precomputed indexes that rules
// use to avoid repeated O(N) edge scans. It is built once per SemanticGraph
// construction and shared across all rule applications.
//
// Every precomputed index is keyed by table node ID. Indexes are populated
// lazily during newInferenceContext initialisation.
type InferenceContext struct {
	// Graph is the original graph.Graph that the SemanticGraph was built from.
	Graph *graph.Graph

	// OutgoingForeignKeyCount is the number of foreign key constraints
	// originating from a table node (how many other tables this table depends on).
	// Counted from EdgeKindReferences edges only (not from the duplicate
	// EdgeKindReferencedBy or EdgeKindDependsOn edges).
	OutgoingForeignKeyCount map[string]int

	// IncomingForeignKeyCount is the number of foreign key constraints
	// pointing to a table node (how many other tables reference this table).
	// Counted from EdgeKindReferences edges only.
	IncomingForeignKeyCount map[string]int

	// ColumnCount is the number of EdgeKindContains edges from a table node
	// to its column nodes.
	ColumnCount map[string]int

	// TemporalPattern maps table node ID to its detected time-tracking pattern.
	TemporalPattern map[string]TemporalPattern

	// AuditPattern maps table node ID to its detected accountability pattern.
	AuditPattern map[string]AuditPattern

	// ForeignKeyColumnIndex maps table node ID to the set of column names that
	// participate as source columns in at least one foreign key on that table.
	ForeignKeyColumnIndex map[string]map[string]bool

	// SelfRefCount maps table node ID to the number of self-referencing foreign
	// keys on that table. Used by HierarchyRule.
	SelfRefCount map[string]int
}

// newInferenceContext builds all precomputed indexes from the source graph
// in a single O(E) pass where E is the number of edges.
//
// IMPORTANT: Each FK constraint produces three graph edges
// (EdgeKindReferences, EdgeKindReferencedBy, EdgeKindDependsOn).
// We count each FK constraint exactly once to avoid double-counting.
// EdgeKindReferencedBy and EdgeKindDependsOn are skipped for metric
// counting (the counting happens on the corresponding EdgeKindReferences
// edge), but FK column metadata is still populated on all edge kinds
// to build the ForeignKeyColumnIndex regardless of edge processing order.
func newInferenceContext(sourceGraph *graph.Graph) *InferenceContext {
	context := &InferenceContext{
		Graph:                   sourceGraph,
		OutgoingForeignKeyCount: make(map[string]int),
		IncomingForeignKeyCount: make(map[string]int),
		ColumnCount:             make(map[string]int),
		TemporalPattern:         make(map[string]TemporalPattern),
		AuditPattern:            make(map[string]AuditPattern),
		ForeignKeyColumnIndex:   make(map[string]map[string]bool),
		SelfRefCount:            make(map[string]int),
	}

	// Single pass over all edges to populate every index.
	for _, edge := range sourceGraph.Edges {
		switch edge.Kind {
		case graph.EdgeKindReferences:
			context.OutgoingForeignKeyCount[edge.From]++
			context.IncomingForeignKeyCount[edge.To]++
			populateForeignKeyColumnIndex(context, edge)

			// Count self-referencing foreign keys for hierarchy detection.
			if edge.From == edge.To {
				context.SelfRefCount[edge.From]++
			}

		case graph.EdgeKindReferencedBy:
			// EdgeKindReferencedBy is the reverse of EdgeKindReferences and
			// carries the same FKMetadata. Counting was already done on the
			// References edge to avoid double-counting each FK constraint.
			// We still process FK column metadata here because the build
			// order could process ReferencedBy edges before References edges.
			populateForeignKeyColumnIndex(context, edge)

		case graph.EdgeKindDependsOn:
			// EdgeKindDependsOn also carries the same FKMetadata.
			// Process column metadata for the same reasons as above.
			populateForeignKeyColumnIndex(context, edge)

		case graph.EdgeKindContains:
			context.ColumnCount[edge.From]++
			populateTemporalPattern(context, edge, sourceGraph)
			populateAuditPattern(context, edge, sourceGraph)
		}
	}

	return context
}

// populateForeignKeyColumnIndex records the FK source columns from a References edge.
func populateForeignKeyColumnIndex(context *InferenceContext, edge *graph.Edge) {
	foreignKeyMetadata, hasMetadata := edge.Metadata.(*graph.FKMetadata)
	if !hasMetadata {
		return
	}
	index := context.ForeignKeyColumnIndex[edge.From]
	if index == nil {
		index = make(map[string]bool)
		context.ForeignKeyColumnIndex[edge.From] = index
	}
	for _, columnName := range foreignKeyMetadata.LocalColumns {
		index[columnName] = true
	}
}

// populateTemporalPattern checks a Contains edge's target column for
// time-tracking column names and updates the context's precomputed pattern.
func populateTemporalPattern(context *InferenceContext, edge *graph.Edge, sourceGraph *graph.Graph) {
	columnNode, exists := sourceGraph.Nodes[edge.To]
	if !exists {
		return
	}
	pattern := context.TemporalPattern[edge.From]
	switch columnNode.Label {
	case "created_at", "CREATED_AT":
		pattern.HasCreatedAt = true
	case "updated_at", "UPDATED_AT":
		pattern.HasUpdatedAt = true
	case "deleted_at", "DELETED_AT":
		pattern.HasDeletedAt = true
	default:
		// Also check case-insensitive variants beyond the exact matches above.
		lower := toLower(columnNode.Label)
		if lower == "created_at" {
			pattern.HasCreatedAt = true
		} else if lower == "updated_at" {
			pattern.HasUpdatedAt = true
		} else if lower == "deleted_at" {
			pattern.HasDeletedAt = true
		}
	}
	context.TemporalPattern[edge.From] = pattern
}

// populateAuditPattern checks a Contains edge's target column for
// accountability-tracking column names and updates the context's precomputed pattern.
func populateAuditPattern(context *InferenceContext, edge *graph.Edge, sourceGraph *graph.Graph) {
	columnNode, exists := sourceGraph.Nodes[edge.To]
	if !exists {
		return
	}
	pattern := context.AuditPattern[edge.From]
	switch columnNode.Label {
	case "created_by", "CREATED_BY":
		pattern.HasCreatedBy = true
	case "updated_by", "UPDATED_BY":
		pattern.HasUpdatedBy = true
	case "deleted_by", "DELETED_BY":
		pattern.HasDeletedBy = true
	default:
		lower := toLower(columnNode.Label)
		if lower == "created_by" {
			pattern.HasCreatedBy = true
		} else if lower == "updated_by" {
			pattern.HasUpdatedBy = true
		} else if lower == "deleted_by" {
			pattern.HasDeletedBy = true
		}
	}
	context.AuditPattern[edge.From] = pattern
}

// toLower returns a lowercased copy of the given string.
// Defined as a small helper to avoid importing strings in this file.
func toLower(value string) string {
	runes := make([]rune, len(value))
	for index, r := range value {
		if r >= 'A' && r <= 'Z' {
			runes[index] = r + 32
		} else {
			runes[index] = r
		}
	}
	return string(runes)
}

// Rule is the single interface that every inference rule must implement.
//
// A Rule examines a SemanticNode in the context of the full source graph and
// precomputed InferenceContext, returning any Inferences it can make. Returning
// an empty slice is valid and simply means the rule found no applicable evidence
// for this node.
//
// # Design contract
//
//   - Rules must be pure functions. They must not mutate the node or the context.
//   - Rules must be deterministic. The same node + context must always produce
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

	// Apply examines the given semantic node in the context of the full
	// InferenceContext and returns all inferences this rule can make.
	// The context carries precomputed indexes so rules never need to
	// scan the graph's edge list directly.
	Apply(tableNode *SemanticNode, context *InferenceContext) []Inference
}
