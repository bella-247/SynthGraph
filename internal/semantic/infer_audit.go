package semantic

import (
	"strings"

	"synthgraph/internal/graph"
)

// AuditRule detects accountability-tracking column patterns.
//
// These are unambiguous structural signals based on ubiquitous cross-language
// naming conventions (created_by, updated_by, deleted_by).
type AuditRule struct{}

func (rule *AuditRule) Name() string { return "audit_rule" }

func (rule *AuditRule) Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference {
	if tableNode.Kind != graph.NodeKindTable {
		return nil
	}

	pattern := detectAuditPattern(tableNode.ID, sourceGraph)

	evidence := make([]string, 0, 3)

	if pattern.HasCreatedBy {
		evidence = append(evidence, "has created_by")
	}
	if pattern.HasUpdatedBy {
		evidence = append(evidence, "has updated_by")
	}
	if pattern.HasDeletedBy {
		evidence = append(evidence, "has deleted_by")
	}

	if len(evidence) == 0 {
		return nil
	}

	return []Inference{{
		Kind:       "audit",
		Confidence: 1.0, // unambiguous matching
		Evidence:   evidence,
	}}
}

// detectAuditPattern scans the column labels of a table to detect
// accountability-tracking columns. Column names are compared case-insensitively.
func detectAuditPattern(tableNodeID string, sourceGraph *graph.Graph) AuditPattern {
	pattern := AuditPattern{}
	for _, edge := range sourceGraph.Edges {
		if edge.Kind != graph.EdgeKindContains || edge.From != tableNodeID {
			continue
		}

		columnNode, exists := sourceGraph.Nodes[edge.To]
		if !exists {
			continue
		}

		lowerColumnName := strings.ToLower(columnNode.Label)
		switch lowerColumnName {
		case "created_by":
			pattern.HasCreatedBy = true
		case "updated_by":
			pattern.HasUpdatedBy = true
		case "deleted_by":
			pattern.HasDeletedBy = true
		}
	}
	return pattern
}
