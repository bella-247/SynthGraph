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

	// Identify table nodes.
	tableNodes := make(map[string]*graph.Node)
	for _, node := range schemaGraph.NodeList {
		if node.Kind == graph.NodeKindTable {
			tableNodes[node.ID] = node
		}
	}

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

	// Phase 3: Find SCCs among unresolved nodes.
	unresolvedSet := makeStringSet(unresolved)
	adjacency := restrictEdgesToSet(schemaGraph, unresolvedSet)
	components := tarjanSCC(unresolved, adjacency)

	// Phase 4: Separate true cycles from blocked DAG nodes.
	// - true cycle = component with >1 node, or a single node with a self-reference FK.
	// - blocked     = size-1 component with no self-reference — it depends on a true cycle.
	var trueCycleComponents [][]string
	var blockedNodeIDs []string

	for _, comp := range components {
		if len(comp) == 0 {
			continue
		}
		if isTrueCycle(schemaGraph, comp) {
			trueCycleComponents = append(trueCycleComponents, comp)
		} else {
			blockedNodeIDs = append(blockedNodeIDs, comp...)
		}
	}

	// Phase 5: Resolve each true cycle.
	availableSet := makeStringSet(ordered) // tables we can already generate
	var dfkList []DeferredFK

	for _, cycle := range trueCycleComponents {
		cyclePlans, cycleDFKs, err := resolveCycle(schemaGraph, schemaModel, cycle, tableNodes, rowCount)
		if err != nil {
			return nil, err
		}
		allPlans = append(allPlans, cyclePlans...)
		dfkList = append(dfkList, cycleDFKs...)
		for _, nodeID := range cycle {
			availableSet[nodeID] = true
		}
	}

	// Phase 6: Process blocked DAG tables. These depend (directly or transitively)
	// on tables in cycles that have now been resolved. Use a second Kahn's pass
	// over only the blocked tables.
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

// ── Phase 1: Kahn's topological sort ──────────────────────────────────────

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
func topologicalSort(schemaGraph *graph.Graph, tableNodes map[string]*graph.Node) (ordered, unresolved []string) {
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

// ── Phase 3-4: Cycle detection ────────────────────────────────────────────

// restrictEdgesToSet builds an adjacency map containing only FK edges where
// both From and To are in the provided set.
func restrictEdgesToSet(schemaGraph *graph.Graph, nodeSet map[string]bool) map[string][]string {
	adj := make(map[string][]string)
	for _, edge := range schemaGraph.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}
		if !nodeSet[edge.From] || !nodeSet[edge.To] {
			continue
		}
		adj[edge.From] = append(adj[edge.From], edge.To)
	}
	return adj
}

// isTrueCycle returns true if the component represents a real cyclic dependency.
// Components of size > 1 are always true cycles. A single node is a true cycle
// only if it has a self-referencing FK edge.
func isTrueCycle(schemaGraph *graph.Graph, component []string) bool {
	if len(component) > 1 {
		return true
	}
	if len(component) == 0 {
		return false
	}
	nodeID := component[0]
	for _, edge := range schemaGraph.Edges {
		if edge.Kind == graph.EdgeKindReferences && edge.From == nodeID && edge.To == nodeID {
			return true
		}
	}
	return false
}

// ── Phase 5: Cycle resolution ─────────────────────────────────────────────

// resolveCycle handles a single true cycle component.
//   - Finds a nullable breakpoint edge
//   - Produces TablePlans for all tables in the cycle
//   - Marks one table's FK columns as deferred
//   - Returns the plans and DeferredFK entries
func resolveCycle(schemaGraph *graph.Graph, schemaModel *schema.Model, component []string, tableNodes map[string]*graph.Node, rowCount int) ([]TablePlan, []DeferredFK, error) {
	breakpoint := findBreakpoint(schemaGraph, component)

	if breakpoint == nil {
		return nil, nil, &PlanError{
			Message: fmt.Sprintf(
				"unresolvable circular dependency detected in cycle involving %d tables",
				len(component),
			),
			CycleTables: component,
			Hint: "make at least one foreign key column in the cycle nullable " +
				"to enable deferred insertion, or remove the circular dependency " +
				"from the schema",
		}
	}

	fkMetadata, ok := breakpoint.Metadata.(*graph.FKMetadata)
	if !ok {
		return nil, nil, &PlanError{
			Message: fmt.Sprintf("breakpoint edge missing FK metadata: %s → %s", breakpoint.From, breakpoint.To),
		}
	}

	cycleOrder := orderCycleMembers(component, breakpoint.From, breakpoint.To)

	var plans []TablePlan
	var dfks []DeferredFK

	for _, tableID := range cycleOrder {
		node := tableNodes[tableID]
		tableData := node.Data.(graph.TableData)
		table := schemaModel.TableMap[tableData.Name]
		if table == nil {
			return nil, nil, &PlanError{
				Message: fmt.Sprintf("table %q not found in schema model", tableData.Name),
			}
		}

		plan := TablePlan{
			TableName: tableData.Name,
			Table:     table,
			RowCount:  rowCount,
		}

		if tableID == breakpoint.From {
			plan.DeferredCols = make([]string, len(fkMetadata.LocalColumns))
			copy(plan.DeferredCols, fkMetadata.LocalColumns)

			for i, localCol := range fkMetadata.LocalColumns {
				refCol := ""
				if i < len(fkMetadata.ForeignColumns) {
					refCol = fkMetadata.ForeignColumns[i]
				}
				dfks = append(dfks, DeferredFK{
					Table:      tableData.Name,
					Column:     localCol,
					References: tableData.Name, // filled below for cross-table refs
					RefColumn:  refCol,
				})
			}
		}

		plans = append(plans, plan)
	}

	// Fix ReferencedTable in DeferredFK entries: it should point to the referenced table,
	// not the current table.
	for i, dfk := range dfks {
		for _, edge := range schemaGraph.Edges {
			if edge.Kind != graph.EdgeKindReferences {
				continue
			}
			if edge.From != breakpoint.From {
				continue
			}
			meta, ok := edge.Metadata.(*graph.FKMetadata)
			if !ok {
				continue
			}
			for j, lc := range meta.LocalColumns {
				if lc == dfk.Column {
					toNodeID := edge.To
					if toNode, exists := tableNodes[toNodeID]; exists {
						toData := toNode.Data.(graph.TableData)
						dfks[i].References = toData.Name
						if j < len(meta.ForeignColumns) {
							dfks[i].RefColumn = meta.ForeignColumns[j]
						}
					}
					break
				}
			}
		}
	}

	return plans, dfks, nil
}

// findBreakpoint searches for a nullable FK edge within a component.
// A breakpoint is an EdgeKindReferences edge where all FK source columns
// are nullable. Returns nil if no such edge exists.
func findBreakpoint(schemaGraph *graph.Graph, component []string) *graph.Edge {
	componentSet := makeStringSet(component)

	for _, edge := range schemaGraph.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}
		if !componentSet[edge.From] || !componentSet[edge.To] {
			continue
		}
		if allColumnsNullable(schemaGraph, edge) {
			return edge
		}
	}

	return nil
}

// allColumnsNullable returns true when every FK source column in the edge
// is nullable. A non-nullable column means we cannot insert NULL to defer
// the FK, so the edge cannot serve as a breakpoint.
func allColumnsNullable(schemaGraph *graph.Graph, edge *graph.Edge) bool {
	fkMetadata, ok := edge.Metadata.(*graph.FKMetadata)
	if !ok || len(fkMetadata.LocalColumns) == 0 {
		return false
	}

	localColumnSet := make(map[string]bool, len(fkMetadata.LocalColumns))
	for _, col := range fkMetadata.LocalColumns {
		localColumnSet[col] = true
	}

	// Collect nullable status per FK column.
	nullableStatus := make(map[string]bool, len(fkMetadata.LocalColumns))
	for _, col := range fkMetadata.LocalColumns {
		nullableStatus[col] = false
	}

	for _, graphEdge := range schemaGraph.Edges {
		if graphEdge.Kind != graph.EdgeKindContains {
			continue
		}
		if graphEdge.From != edge.From {
			continue
		}

		columnNode, exists := schemaGraph.Nodes[graphEdge.To]
		if !exists {
			continue
		}

		if !localColumnSet[columnNode.Label] {
			continue
		}

		columnData, ok := columnNode.Data.(graph.ColumnData)
		if !ok {
			continue
		}

		if columnData.Nullable {
			nullableStatus[columnNode.Label] = true
		}
	}

	// Every FK column must be nullable.
	for _, nullable := range nullableStatus {
		if !nullable {
			return false
		}
	}
	return true
}

// orderCycleMembers produces a valid generation order for a cycle.
//
// The breakpoint child (breakpointFrom) is placed FIRST so that its PK values
// are available when other cycle members reference it via non-deferred FK
// columns. The breakpoint parent (breakpointTo) is placed LAST so that its
// own FK references to other cycle members can be satisfied.
func orderCycleMembers(component []string, breakpointFrom, breakpointTo string) []string {
	ordered := make([]string, 0, len(component))
	seen := make(map[string]bool, len(component))

	// Breakpoint child first — its PKs must be available for other cycle members.
	ordered = append(ordered, breakpointFrom)
	seen[breakpointFrom] = true

	// Other cycle members.
	for _, id := range component {
		if !seen[id] && id != breakpointTo {
			ordered = append(ordered, id)
			seen[id] = true
		}
	}

	// Breakpoint parent last — it references other cycle members via FKs.
	if !seen[breakpointTo] {
		ordered = append(ordered, breakpointTo)
	}

	return ordered
}

// ── Phase 6: Process blocked tables ───────────────────────────────────────

// processBlockedTables handles tables that were not part of any true cycle
// but were blocked (their FK targets include nodes in unresolved cycles).
// After those cycles are resolved (made available), these tables' outCount
// may reach zero, making them generate-able.
//
// Uses a Kahn's-style pass over the blocked set.
func processBlockedTables(schemaGraph *graph.Graph, schemaModel *schema.Model, blockedNodeIDs []string, availableSet map[string]bool, tableNodes map[string]*graph.Node, rowCount int) ([]TablePlan, error) {
	if len(blockedNodeIDs) == 0 {
		return nil, nil
	}

	// Count how many FK targets per blocked table are NOT yet available.
	blockedCounts := make(map[string]int, len(blockedNodeIDs))
	var queue []string

	for _, id := range blockedNodeIDs {
		count := 0
		for _, edge := range schemaGraph.Edges {
			if edge.Kind == graph.EdgeKindReferences && edge.From == id {
				if !availableSet[edge.To] {
					count++
				}
			}
		}
		if count == 0 {
			queue = append(queue, id)
		} else {
			blockedCounts[id] = count
		}
	}

	var plans []TablePlan

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		availableSet[current] = true

		plan, err := makeTablePlan(current, tableNodes, schemaModel, rowCount)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)

		// Decrement dependents that are still in the blocked set.
		for _, edge := range schemaGraph.Edges {
			if edge.Kind == graph.EdgeKindReferences && edge.To == current {
				if _, stillBlocked := blockedCounts[edge.From]; stillBlocked {
					blockedCounts[edge.From]--
					if blockedCounts[edge.From] == 0 {
						queue = append(queue, edge.From)
					}
				}
			}
		}
	}

	// Any remaining blocked tables indicate an error.
	var remaining []string
	for _, id := range blockedNodeIDs {
		if !availableSet[id] {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) > 0 {
		return nil, &PlanError{
			Message:    fmt.Sprintf("cannot resolve %d table(s) after processing all cycles — they depend on unresolvable cycles", len(remaining)),
			CycleTables: remaining,
			Hint:       "review the table dependencies; a larger cycle may involve these tables indirectly",
		}
	}

	return plans, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

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

// ── Tarjan's SCC ──────────────────────────────────────────────────────────

// tarjanSCC finds all strongly connected components in the given graph
// using Tarjan's algorithm. Returns components in topological order.
func tarjanSCC(nodeOrder []string, adjacency map[string][]string) [][]string {
	type frame struct {
		index   int
		lowlink int
		onStack bool
	}

	state := make(map[string]*frame, len(nodeOrder))
	for _, nodeID := range nodeOrder {
		state[nodeID] = &frame{index: -1, lowlink: -1}
	}

	var (
		currentIndex int
		stack        []string
		components   [][]string
	)

	var strongConnect func(nodeID string)
	strongConnect = func(nodeID string) {
		f := state[nodeID]
		f.index = currentIndex
		f.lowlink = currentIndex
		currentIndex++
		stack = append(stack, nodeID)
		f.onStack = true

		for _, neighborID := range adjacency[nodeID] {
			neighborState, exists := state[neighborID]
			if !exists {
				continue
			}
			if neighborState.index == -1 {
				strongConnect(neighborID)
				if neighborState.lowlink < f.lowlink {
					f.lowlink = neighborState.lowlink
				}
			} else if neighborState.onStack {
				if neighborState.index < f.lowlink {
					f.lowlink = neighborState.index
				}
			}
		}

		if f.lowlink == f.index {
			var component []string
			for {
				popped := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				state[popped].onStack = false
				component = append(component, popped)
				if popped == nodeID {
					break
				}
			}
			components = append(components, component)
		}
	}

	for _, nodeID := range nodeOrder {
		if state[nodeID].index == -1 {
			strongConnect(nodeID)
		}
	}

	return components
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
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	unique := make([]string, 0, len(result))
	seen2 := make(map[string]bool, len(result))
	for _, v := range result {
		if !seen2[v] {
			seen2[v] = true
			unique = append(unique, v)
		}
	}
	return unique
}
