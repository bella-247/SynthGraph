package semantic

import (
	"synthgraph/internal/graph"
)

// AuditRule detects accountability-tracking column patterns.
//
// These are unambiguous structural signals based on ubiquitous cross-language
// naming conventions (created_by, updated_by, deleted_by).
type AuditRule struct{}

func (auditRule *AuditRule) Name() string { return "audit_rule" }

func (auditRule *AuditRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	if tableNode.Kind != graph.NodeKindTable {
		return nil
	}

	pattern := context.AuditPattern[tableNode.ID]

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
