package semantic

import (
	"synthgraph/internal/graph"
)

// TemporalRule detects temporal and soft-delete column patterns.
//
// These are unambiguous structural signals based on ubiquitous cross-language
// naming conventions (created_at, updated_at, deleted_at).
type TemporalRule struct{}

func (temporalRule *TemporalRule) Name() string { return "temporal_rule" }

func (temporalRule *TemporalRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	if tableNode.Kind != graph.NodeKindTable {
		return nil
	}

	pattern := context.TemporalPattern[tableNode.ID]
	hasCreatedAt := pattern.HasCreatedAt
	hasUpdatedAt := pattern.HasUpdatedAt
	hasDeletedAt := pattern.HasDeletedAt

	inferences := make([]Inference, 0, 2)
	evidence := make([]string, 0, 3)

	if hasCreatedAt {
		evidence = append(evidence, "has created_at")
	}
	if hasUpdatedAt {
		evidence = append(evidence, "has updated_at")
	}
	if hasDeletedAt {
		evidence = append(evidence, "has deleted_at")
	}

	if len(evidence) > 0 {
		inferences = append(inferences, Inference{
			Kind:       "temporal",
			Confidence: 1.0, // unambiguous matching
			Evidence:   evidence,
		})
	}

	if hasDeletedAt {
		inferences = append(inferences, Inference{
			Kind:       "soft_delete",
			Confidence: 1.0,
			Evidence:   []string{"has deleted_at column indicating logical deletion"},
		})
	}

	return inferences
}
