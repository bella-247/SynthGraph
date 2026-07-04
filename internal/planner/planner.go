// Package planner consumes a graph.Graph and produces a GenerationPlan —
// a deterministic, dependency-resolved ordering of tables that the generator
// can execute to produce valid, constraint-compliant data.
//
// The planner is the "bridge" between the structural graph and the data
// generator. It resolves:
//   - What order to generate tables (topological sort via Kahn's algorithm)
//   - Which tables form circular FK dependencies (Tarjan's SCC)
//   - Where to break cycles via nullable deferred insertion
//   - Which FK columns must be backfilled after initial INSERT
//
// # Architecture position
//
//	Parser → schema.Model → graph.Build → graph.Graph
//	                                           │
//	                                           ▼
//	                                     planner.BuildPlan
//	                                           │
//	                                           ▼
//	                                     GenerationPlan
//	                                           │
//	                                           ▼
//	                                     generator.Generate
package planner

import (
	"fmt"
	"sort"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// GenerationPlan is the output of the Planner stage.
// It contains a deterministically ordered list of tables to generate
// and any foreign key columns that require deferred (post-INSERT) backfilling.
type GenerationPlan struct {
	// Order is the list of TablePlans in generation order.
	// Every table in the input schema appears exactly once.
	Order []TablePlan

	// DeferredFKs lists the FK columns that must be backfilled via UPDATE
	// after all tables in the plan have been inserted. This is the mechanism
	// for resolving circular dependencies.
	DeferredFKs []DeferredFK
}

// TablePlan describes the generation plan for a single table.
type TablePlan struct {
	// TableName is the fully-qualified name of the table.
	TableName string

	// Table is a pointer to the schema.Table for this table.
	// The generator uses this to resolve column metadata and constraints.
	Table *schema.Table

	// RowCount is the number of rows to generate for this table.
	RowCount int

	// DeferredCols lists FK column names whose values are inserted as NULL
	// and must be backfilled via UPDATE after all tables are inserted.
	// These columns belong to cycles that were broken via nullable edges.
	DeferredCols []string
}

// DeferredFK describes a single FK column that must be backfilled after
// initial INSERT. The backfill UPDATE sets this column to a valid value
// selected from the referenced table's generated PK pool.
type DeferredFK struct {
	// Table is the table containing the deferred column.
	Table string

	// Column is the FK column name that was inserted as NULL.
	Column string

	// References is the table whose PK values are used for backfill.
	References string

	// RefColumn is the PK column in the referenced table.
	RefColumn string
}

// PlanError describes a planning failure (typically an unresolvable cycle).
type PlanError struct {
	// Message describes the exact planning failure.
	Message string

	// CycleTables lists the tables involved in the unresolvable cycle.
	CycleTables []string

	// Hint provides an actionable suggestion for resolving the error.
	Hint string
}

// Error implements the error interface.
func (planError *PlanError) Error() string {
	return "planner: " + planError.Message
}

// BuildPlan consumes a graph.Graph and schema.Model and produces a
// GenerationPlan — the complete, dependency-resolved generation order.
//
// The algorithm:
//
//  1. Topological sort (Kahn's) over all table nodes using FK dependency edges.
//  2. Remaining nodes (never reaching in-degree zero) are analysed for cycles.
//  3. Tarjan's SCC separates true cycles (mutual FK dependencies) from tables
//     that are merely blocked (they depend on a table inside a cycle).
//  4. Each true cycle is analysed for a nullable breakpoint edge. If found,
//     that FK column is deferred (inserted as NULL, backfilled via UPDATE).
//  5. After cycles are resolved, blocked tables are processed via a second
//     Kahn's pass — their dependencies are now available, so they resolve.
//  6. If no nullable edge exists in any cycle, BuildPlan returns a PlanError.
func BuildPlan(schemaGraph *graph.Graph, schemaModel *schema.Model, rowCount int) (*GenerationPlan, error) {
	ensureTableMap(schemaModel)

	tableNodes := collectTableNodes(schemaGraph)

	// Phase 1: Topological sort via Kahn's algorithm.
	ordered, unresolved := topologicalSort(schemaGraph, tableNodes)

	// Phase 2: Build TablePlans for the ordered (acyclic) portion.
	allPlans, err := buildTablePlans(ordered, tableNodes, schemaModel, rowCount)
	if err != nil {
		return nil, err
	}

	if len(unresolved) == 0 {
		return &GenerationPlan{Order: allPlans}, nil
	}

	// Phase 3-4: Find SCCs among unresolved nodes and separate true cycles
	// from blocked DAG nodes.
	trueCycleComponents, blockedNodeIDs := classifyUnresolvedComponents(schemaGraph, unresolved)

	// Phase 5: Resolve each true cycle.
	cyclePlans, dfkList, availableSet, err := resolveAllCycles(
		schemaGraph, schemaModel, trueCycleComponents, tableNodes, ordered, rowCount,
	)
	if err != nil {
		return nil, err
	}
	allPlans = append(allPlans, cyclePlans...)

	// Phase 6: Process blocked DAG tables.
	blockedPlans, err := processBlockedTables(schemaGraph, schemaModel, blockedNodeIDs, availableSet, tableNodes, rowCount)
	if err != nil {
		return nil, err
	}
	allPlans = append(allPlans, blockedPlans...)

	return &GenerationPlan{
		Order:       allPlans,
		DeferredFKs: dfkList,
	}, nil
}

// collectTableNodes builds a map of table node IDs to graph.Node from the graph.
func collectTableNodes(schemaGraph *graph.Graph) map[string]*graph.Node {
	tableNodes := make(map[string]*graph.Node)
	for _, node := range schemaGraph.NodeList {
		if node.Kind == graph.NodeKindTable {
			tableNodes[node.ID] = node
		}
	}
	return tableNodes
}

// ensureTableMap builds the TableMap if it hasn't been populated yet.
// The parser populates TableMap during Translate, but models constructed
// directly in tests or by other means may have a nil map.
func ensureTableMap(model *schema.Model) {
	if model.TableMap != nil {
		return
	}
	model.TableMap = make(map[string]*schema.Table, len(model.Tables))
	for _, table := range model.Tables {
		model.TableMap[table.Name] = table
	}
}

// buildTablePlans converts a list of node IDs (already in generation order)
// into TablePlan entries.
func buildTablePlans(nodeIDs []string, tableNodes map[string]*graph.Node, schemaModel *schema.Model, rowCount int) ([]TablePlan, error) {
	plans := make([]TablePlan, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		plan, err := makeTablePlan(id, tableNodes, schemaModel, rowCount)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

// makeTablePlan creates a single TablePlan from a node ID.
func makeTablePlan(nodeID string, tableNodes map[string]*graph.Node, schemaModel *schema.Model, rowCount int) (TablePlan, error) {
	node := tableNodes[nodeID]
	tableData := node.Data.(graph.TableData)
	table := schemaModel.TableMap[tableData.Name]
	if table == nil {
		return TablePlan{}, &PlanError{
			Message: fmt.Sprintf("table %q not found in schema model", tableData.Name),
		}
	}
	return TablePlan{
		TableName: tableData.Name,
		Table:     table,
		RowCount:  rowCount,
	}, nil
}

// ── Utility ───────────────────────────────────────────────────────────────

// makeStringSet converts a string slice to a set map.
func makeStringSet(values []string) map[string]bool {
	s := make(map[string]bool, len(values))
	for _, v := range values {
		s[v] = true
	}
	return s
}

// dedupeStrings removes duplicate strings, returning a sorted result.
func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	sort.Strings(unique)
	return unique
}
