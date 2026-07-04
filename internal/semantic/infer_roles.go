package semantic

import (
	"strconv"

	"synthgraph/internal/graph"
)

// JunctionRule detects junction (many-to-many bridge) tables.
//
// A junction table is one whose composite primary key consists entirely of
// foreign key columns. This is the canonical structural signal for a bridge
// table in relational modelling.
//
// Examples: product_categories (product_id, category_id), user_roles (user_id, role_id).
//
// Confidence is 0.95 — this is an extremely strong structural signal with no
// ambiguity. The small gap from 1.0 accounts for the theoretical possibility
// that a schema uses a composite PK of FK columns for reasons other than
// junction semantics.
type JunctionRule struct{}

func (junctionRule *JunctionRule) Name() string { return "junction_rule" }

func (junctionRule *JunctionRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	tableData, isTableNode := tableNode.Data.(graph.TableData)
	if !isTableNode || len(tableData.PrimaryKey) < 2 {
		// Junction tables always have a composite PK (at least 2 columns).
		return nil
	}

	foreignKeyColumnIndex := context.ForeignKeyColumnIndex[tableNode.ID]

	evidenceLines := make([]string, 0, len(tableData.PrimaryKey)+1)
	allPrimaryKeyColumnsAreForeignKeys := true

	for _, primaryKeyColumn := range tableData.PrimaryKey {
		if !foreignKeyColumnIndex[primaryKeyColumn] {
			allPrimaryKeyColumnsAreForeignKeys = false
			break
		}
		evidenceLines = append(evidenceLines, "primary key column \""+primaryKeyColumn+"\" is also a foreign key column")
	}

	if !allPrimaryKeyColumnsAreForeignKeys {
		return nil
	}

	evidenceLines = append(evidenceLines,
		"composite primary key has "+countString(len(tableData.PrimaryKey))+" columns, all of which are foreign keys",
	)

	return []Inference{{
		Kind:       string(TableRoleJunction),
		Confidence: 0.95,
		Evidence:   evidenceLines,
	}}
}

// HierarchyRule detects self-referencing (hierarchical) tables.
//
// A table is hierarchical if it has a foreign key that points back to itself.
// This pattern forms a tree or directed acyclic graph of rows within the same
// table (e.g., an employee reports to a manager, who is also an employee).
//
// Confidence is 0.95 — a self-referencing FK is an unambiguous structural signal.
type HierarchyRule struct{}

func (hierarchyRule *HierarchyRule) Name() string { return "hierarchy_rule" }

func (hierarchyRule *HierarchyRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	if tableNode.Kind != graph.NodeKindTable {
		return nil
	}

	selfReferencingFKCount := context.SelfRefCount[tableNode.ID]
	if selfReferencingFKCount == 0 {
		return nil
	}

	return []Inference{{
		Kind:       string(TableRoleHierarchical),
		Confidence: 0.95,
		Evidence: []string{
			"table has " + countString(selfReferencingFKCount) + " self-referencing foreign key(s), forming a hierarchical structure",
		},
	}}
}

// LookupRule detects lookup / reference tables — small, stable tables of
// known values that other tables reference frequently.
//
// Structural signals (each adds to confidence):
//   - No outgoing foreign keys (the table doesn't depend on anything): +0.40
//   - Frequently referenced by other tables (many incoming references): +0.30
//   - Few columns (≤ 6): +0.20
//   - No temporal columns (lookup tables are stable, not time-tracked): +0.10
//
// A table must accumulate at least 0.50 confidence to be classified as a lookup.
// This threshold prevents misclassifying tables that only partially match the pattern.
type LookupRule struct{}

func (lookupRule *LookupRule) Name() string { return "lookup_rule" }

func (lookupRule *LookupRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	_, isTableNode := tableNode.Data.(graph.TableData)
	if !isTableNode {
		return nil
	}

	outgoingFKCount := context.OutgoingForeignKeyCount[tableNode.ID]
	incomingFKCount := context.IncomingForeignKeyCount[tableNode.ID]
	columnCount := context.ColumnCount[tableNode.ID]
	temporalPattern := context.TemporalPattern[tableNode.ID]
	hasTemporalColumns := temporalPattern.HasCreatedAt || temporalPattern.HasUpdatedAt

	confidence := 0.0
	evidenceLines := make([]string, 0, 4)

	if outgoingFKCount == 0 {
		confidence += 0.40
		evidenceLines = append(evidenceLines, "has no outgoing foreign keys (does not depend on other tables)")
	}
	if incomingFKCount >= 2 {
		confidence += 0.30
		evidenceLines = append(evidenceLines,
			"referenced by "+countString(incomingFKCount)+" other tables (frequently used reference)")
	}
	if columnCount <= 6 {
		confidence += 0.20
		evidenceLines = append(evidenceLines, "has only "+countString(columnCount)+" column(s) — small, focused table")
	}
	if !hasTemporalColumns {
		confidence += 0.10
		evidenceLines = append(evidenceLines, "has no temporal tracking columns (stable, infrequently modified)")
	}

	const minimumLookupConfidence = 0.50
	if confidence < minimumLookupConfidence {
		return nil
	}

	return []Inference{{
		Kind:       string(TableRoleLookup),
		Confidence: confidence,
		Evidence:   evidenceLines,
	}}
}

// TransactionalRule detects transactional / event tables — tables that capture
// that something happened at a point in time.
//
// Structural signals (each adds to confidence):
//   - Has a created_at temporal column: +0.40
//   - Has at least one outgoing foreign key to another entity: +0.30
//   - Has an updated_at column: +0.15
//   - Is not a junction table: +0.15
//
// A table must accumulate at least 0.55 confidence to be classified as
// transactional. Junction tables are excluded because they are already classified
// with a stronger, more specific signal.
type TransactionalRule struct{}

func (transactionalRule *TransactionalRule) Name() string { return "transactional_rule" }

func (transactionalRule *TransactionalRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	tableData, isTableNode := tableNode.Data.(graph.TableData)
	if !isTableNode {
		return nil
	}

	temporalPattern := context.TemporalPattern[tableNode.ID]
	outgoingFKCount := context.OutgoingForeignKeyCount[tableNode.ID]
	isJunction := hasJunctionSignal(tableData, context.ForeignKeyColumnIndex[tableNode.ID])

	confidence := 0.0
	evidenceLines := make([]string, 0, 4)

	if temporalPattern.HasCreatedAt {
		confidence += 0.40
		evidenceLines = append(evidenceLines, "has a \"created_at\" column indicating event timestamping")
	}
	if outgoingFKCount >= 1 {
		confidence += 0.30
		evidenceLines = append(evidenceLines,
			"has "+countString(outgoingFKCount)+" outgoing foreign key(s) referencing domain entities")
	}
	if temporalPattern.HasUpdatedAt {
		confidence += 0.15
		evidenceLines = append(evidenceLines, "has an \"updated_at\" column indicating lifecycle tracking")
	}
	if !isJunction {
		confidence += 0.15
		evidenceLines = append(evidenceLines, "is not a junction table — has an independent identity")
	}

	const minimumTransactionalConfidence = 0.55
	if confidence < minimumTransactionalConfidence {
		return nil
	}

	return []Inference{{
		Kind:       string(TableRoleTransactional),
		Confidence: confidence,
		Evidence:   evidenceLines,
	}}
}

// EntityRule assigns the baseline Entity role to every table node.
//
// Entity is the default classification — if no more specific rule fires with
// sufficient confidence, the table is still a domain entity (a thing the system
// stores). Confidence is always 0.50 for entity, which means more specific roles
// (junction: 0.95, hierarchical: 0.95) always have higher confidence in the
// Inferences list, giving downstream consumers a clear ranking.
type EntityRule struct{}

func (entityRule *EntityRule) Name() string { return "entity_rule" }

func (entityRule *EntityRule) Apply(tableNode *SemanticNode, context *InferenceContext) []Inference {
	if tableNode.Kind != graph.NodeKindTable {
		return nil
	}
	return []Inference{{
		Kind:       string(TableRoleEntity),
		Confidence: 0.50,
		Evidence:   []string{"every table is a domain entity by default"},
	}}
}

// ── Shared helpers ─────────────────────────────────────────────────────────────

// hasJunctionSignal returns true if the table's composite primary key consists
// entirely of foreign key columns. This is a proper structural check — it
// verifies that every column in the PK also participates as a source column in
// at least one foreign key constraint, rather than just checking PK length.
func hasJunctionSignal(tableData graph.TableData, foreignKeyColumnIndex map[string]bool) bool {
	if len(tableData.PrimaryKey) < 2 {
		return false
	}
	for _, primaryKeyColumn := range tableData.PrimaryKey {
		if !foreignKeyColumnIndex[primaryKeyColumn] {
			return false
		}
	}
	return true
}

// countString converts an integer to its string representation.
func countString(value int) string {
	return strconv.Itoa(value)
}
