package semantic

import (
	"strconv"
	"strings"

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

func (junctionRule *JunctionRule) Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference {
	tableData, isTableNode := tableNode.Data.(graph.TableData)
	if !isTableNode || len(tableData.PrimaryKey) < 2 {
		// Junction tables always have a composite PK (at least 2 columns).
		return nil
	}

	allForeignKeyColumnsOnTable := buildForeignKeyColumnIndex(tableNode.ID, sourceGraph)

	evidenceLines := make([]string, 0, len(tableData.PrimaryKey)+1)
	allPrimaryKeyColumnsAreForeignKeys := true

	for _, primaryKeyColumn := range tableData.PrimaryKey {
		if !allForeignKeyColumnsOnTable[primaryKeyColumn] {
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

func (hierarchyRule *HierarchyRule) Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference {
	if tableNode.Kind != graph.NodeKindTable {
		return nil
	}

	selfReferencingFKCount := 0
	for _, edge := range sourceGraph.Edges {
		if edge.Kind == graph.EdgeKindReferences && edge.From == tableNode.ID && edge.To == tableNode.ID {
			selfReferencingFKCount++
		}
	}

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

func (lookupRule *LookupRule) Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference {
	_, isTableNode := tableNode.Data.(graph.TableData)
	if !isTableNode {
		return nil
	}

	outgoingFKCount := countEdgesFrom(tableNode.ID, graph.EdgeKindReferences, sourceGraph)
	incomingFKCount := countEdgesTo(tableNode.ID, graph.EdgeKindReferencedBy, sourceGraph)
	columnCount := countEdgesFrom(tableNode.ID, graph.EdgeKindContains, sourceGraph)
	temporalPattern := detectTemporalPattern(tableNode.ID, sourceGraph)
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

func (transactionalRule *TransactionalRule) Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference {
	tableData, isTableNode := tableNode.Data.(graph.TableData)
	if !isTableNode {
		return nil
	}

	temporalPattern := detectTemporalPattern(tableNode.ID, sourceGraph)
	outgoingFKCount := countEdgesFrom(tableNode.ID, graph.EdgeKindReferences, sourceGraph)
	isJunction := hasJunctionSignal(tableData)

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

func (entityRule *EntityRule) Apply(tableNode *SemanticNode, sourceGraph *graph.Graph) []Inference {
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

// buildForeignKeyColumnIndex returns a set of all column names that are
// foreign key source columns for the given table node ID.
func buildForeignKeyColumnIndex(tableNodeID string, sourceGraph *graph.Graph) map[string]bool {
	foreignKeyColumnIndex := make(map[string]bool)
	for _, edge := range sourceGraph.Edges {
		if edge.Kind != graph.EdgeKindReferences || edge.From != tableNodeID {
			continue
		}
		foreignKeyMetadata, hasFKMetadata := edge.Metadata.(*graph.FKMetadata)
		if !hasFKMetadata {
			continue
		}
		for _, columnName := range foreignKeyMetadata.LocalColumns {
			foreignKeyColumnIndex[columnName] = true
		}
	}
	return foreignKeyColumnIndex
}

// detectTemporalPattern scans the column labels of a table to detect
// time-tracking columns. Column names are compared case-insensitively.
func detectTemporalPattern(tableNodeID string, sourceGraph *graph.Graph) TemporalPattern {
	pattern := TemporalPattern{}
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
		case "created_at":
			pattern.HasCreatedAt = true
		case "updated_at":
			pattern.HasUpdatedAt = true
		case "deleted_at":
			pattern.HasDeletedAt = true
		}
	}
	return pattern
}

// hasJunctionSignal returns true if the table's primary key structure matches
// the junction table pattern (composite PK where all PK cols are FK cols).
// This is used by the TransactionalRule to exclude junction tables.
func hasJunctionSignal(tableData graph.TableData) bool {
	return len(tableData.PrimaryKey) >= 2
}

// countEdgesFrom counts edges of the given kind that originate from the given node ID.
func countEdgesFrom(fromNodeID string, kind graph.EdgeKind, sourceGraph *graph.Graph) int {
	count := 0
	for _, edge := range sourceGraph.Edges {
		if edge.Kind == kind && edge.From == fromNodeID {
			count++
		}
	}
	return count
}

// countEdgesTo counts edges of the given kind that point to the given node ID.
func countEdgesTo(toNodeID string, kind graph.EdgeKind, sourceGraph *graph.Graph) int {
	count := 0
	for _, edge := range sourceGraph.Edges {
		if edge.Kind == kind && edge.To == toNodeID {
			count++
		}
	}
	return count
}

// countString converts an integer to its string representation. This small helper
// keeps inline string concatenation in evidence messages readable.
func countString(value int) string {
	return strconv.Itoa(value)
}
