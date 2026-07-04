package planner

import (
	"fmt"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// ── Adjacency ─────────────────────────────────────────────────────────────

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

// ── Tarjan's SCC ──────────────────────────────────────────────────────────

// tarjanEngine holds mutable state for Tarjan's strongly connected components
// algorithm. The struct-with-methods approach replaces a deeply-nested closure
// and keeps per-method cognitive complexity well below the threshold.
type tarjanEngine struct {
	frames       map[string]*tarjanFrame
	currentIndex int
	stack        []string
	components   [][]string
	adjacency    map[string][]string
}

type tarjanFrame struct {
	index   int
	lowlink int
	onStack bool
}

func newTarjanEngine(nodeOrder []string, adjacency map[string][]string) *tarjanEngine {
	engine := &tarjanEngine{
		frames:    make(map[string]*tarjanFrame, len(nodeOrder)),
		adjacency: adjacency,
	}
	for _, nodeID := range nodeOrder {
		engine.frames[nodeID] = &tarjanFrame{index: -1, lowlink: -1}
	}
	return engine
}

func (engine *tarjanEngine) strongConnect(nodeID string) {
	frame := engine.frames[nodeID]
	frame.index = engine.currentIndex
	frame.lowlink = engine.currentIndex
	engine.currentIndex++
	engine.stack = append(engine.stack, nodeID)
	frame.onStack = true

	for _, neighborID := range engine.adjacency[nodeID] {
		engine.processNeighbor(nodeID, neighborID)
	}

	if frame.lowlink == frame.index {
		engine.collectComponent(nodeID)
	}
}

func (engine *tarjanEngine) processNeighbor(nodeID, neighborID string) {
	neighborFrame, exists := engine.frames[neighborID]
	if !exists {
		return
	}
	currentFrame := engine.frames[nodeID]
	if neighborFrame.index == -1 {
		engine.strongConnect(neighborID)
		if neighborFrame.lowlink < currentFrame.lowlink {
			currentFrame.lowlink = neighborFrame.lowlink
		}
	} else if neighborFrame.onStack {
		if neighborFrame.index < currentFrame.lowlink {
			currentFrame.lowlink = neighborFrame.index
		}
	}
}

func (engine *tarjanEngine) collectComponent(nodeID string) {
	var component []string
	for {
		popped := engine.stack[len(engine.stack)-1]
		engine.stack = engine.stack[:len(engine.stack)-1]
		engine.frames[popped].onStack = false
		component = append(component, popped)
		if popped == nodeID {
			break
		}
	}
	engine.components = append(engine.components, component)
}

// tarjanSCC finds all strongly connected components in the given graph
// using Tarjan's algorithm. Returns components in topological order.
func tarjanSCC(nodeOrder []string, adjacency map[string][]string) [][]string {
	engine := newTarjanEngine(nodeOrder, adjacency)
	for _, nodeID := range nodeOrder {
		if engine.frames[nodeID].index == -1 {
			engine.strongConnect(nodeID)
		}
	}
	return engine.components
}

// ── Cycle resolution ──────────────────────────────────────────────────────

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

	plans, dfks, buildError := buildCycleTablePlans(cycleOrder, tableNodes, schemaModel, rowCount, breakpoint.From, fkMetadata)
	if buildError != nil {
		return nil, nil, buildError
	}

	fixDeferredFKReferences(dfks, schemaGraph, breakpoint.From, tableNodes)

	return plans, dfks, nil
}

// buildCycleTablePlans produces TablePlans and initial DeferredFK entries for
// all tables in a cycle. The breakpoint table's FK columns are marked as
// deferred (inserted as NULL, backfilled via UPDATE later).
func buildCycleTablePlans(cycleOrder []string, tableNodes map[string]*graph.Node, schemaModel *schema.Model, rowCount int, breakpointFrom string, fkMetadata *graph.FKMetadata) ([]TablePlan, []DeferredFK, error) {
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

		if tableID == breakpointFrom {
			plan.DeferredCols = make([]string, len(fkMetadata.LocalColumns))
			copy(plan.DeferredCols, fkMetadata.LocalColumns)
			dfks = buildDeferredFKs(fkMetadata, tableData.Name)
		}

		plans = append(plans, plan)
	}

	return plans, dfks, nil
}

// buildDeferredFKs creates DeferredFK entries for the breakpoint table's FK
// columns. The References field is initially set to the table itself and is
// corrected later by fixDeferredFKReferences.
func buildDeferredFKs(fkMetadata *graph.FKMetadata, tableName string) []DeferredFK {
	dfks := make([]DeferredFK, 0, len(fkMetadata.LocalColumns))
	for index, localColumn := range fkMetadata.LocalColumns {
		refColumn := ""
		if index < len(fkMetadata.ForeignColumns) {
			refColumn = fkMetadata.ForeignColumns[index]
		}
		dfks = append(dfks, DeferredFK{
			Table:      tableName,
			Column:     localColumn,
			References: tableName,
			RefColumn:  refColumn,
		})
	}
	return dfks
}

// fixDeferredFKReferences corrects the References and RefColumn fields in
// DeferredFK entries. The initial build sets them to the source table; this
// function resolves them to the actual FK target by scanning foreign key edges.
func fixDeferredFKReferences(dfks []DeferredFK, schemaGraph *graph.Graph, breakpointFrom string, tableNodes map[string]*graph.Node) {
	for dfkIndex := range dfks {
		for _, edge := range schemaGraph.Edges {
			if edge.Kind != graph.EdgeKindReferences {
				continue
			}
			if edge.From != breakpointFrom {
				continue
			}
			updateDeferredFKFromEdge(&dfks[dfkIndex], edge, tableNodes)
		}
	}
}

// updateDeferredFKFromEdge scans a single FK edge's metadata and updates
// the deferred FK entry with the correct referenced table and column names.
func updateDeferredFKFromEdge(deferredFK *DeferredFK, edge *graph.Edge, tableNodes map[string]*graph.Node) {
	fkMetadata, hasMetadata := edge.Metadata.(*graph.FKMetadata)
	if !hasMetadata {
		return
	}
	for columnIndex, localColumn := range fkMetadata.LocalColumns {
		if localColumn == deferredFK.Column {
			if targetNode, nodeExists := tableNodes[edge.To]; nodeExists {
				targetData := targetNode.Data.(graph.TableData)
				deferredFK.References = targetData.Name
				if columnIndex < len(fkMetadata.ForeignColumns) {
					deferredFK.RefColumn = fkMetadata.ForeignColumns[columnIndex]
				}
			}
			return
		}
	}
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

	nullableStatus := collectNullableStatus(schemaGraph, edge, fkMetadata)

	for _, nullable := range nullableStatus {
		if !nullable {
			return false
		}
	}
	return true
}

// collectNullableStatus scans the graph's Contains edges for the FK source
// table and records whether each FK-local column is nullable or not.
func collectNullableStatus(schemaGraph *graph.Graph, edge *graph.Edge, fkMetadata *graph.FKMetadata) map[string]bool {
	localColumnSet := make(map[string]bool, len(fkMetadata.LocalColumns))
	nullableStatus := make(map[string]bool, len(fkMetadata.LocalColumns))
	for _, column := range fkMetadata.LocalColumns {
		localColumnSet[column] = true
		nullableStatus[column] = false
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
		columnData, hasData := columnNode.Data.(graph.ColumnData)
		if !hasData {
			continue
		}
		if columnData.Nullable {
			nullableStatus[columnNode.Label] = true
		}
	}

	return nullableStatus
}

// classifyUnresolvedComponents runs Tarjan's SCC on the unresolved nodes
// and separates the resulting components into true cycles (mutual FK
// dependencies) and blocked nodes (DAG nodes that depend on a cycle node).
func classifyUnresolvedComponents(schemaGraph *graph.Graph, unresolved []string) (trueCycles [][]string, blockedNodes []string) {
	unresolvedSet := makeStringSet(unresolved)
	adjacency := restrictEdgesToSet(schemaGraph, unresolvedSet)
	components := tarjanSCC(unresolved, adjacency)

	for _, comp := range components {
		if len(comp) == 0 {
			continue
		}
		if isTrueCycle(schemaGraph, comp) {
			trueCycles = append(trueCycles, comp)
		} else {
			blockedNodes = append(blockedNodes, comp...)
		}
	}
	return trueCycles, blockedNodes
}

// resolveAllCycles processes all true cycle components, producing TablePlans
// and DeferredFK entries for each. It also builds the availableSet — a set of
// node IDs whose data has been generated — by marking cycle nodes as available
// after resolution. The availableSet is seeded with already-ordered nodes and is
// used by processBlockedTables to determine when blocked dependencies are met.
func resolveAllCycles(
	schemaGraph *graph.Graph,
	schemaModel *schema.Model,
	trueCycleComponents [][]string,
	tableNodes map[string]*graph.Node,
	ordered []string,
	rowCount int,
) ([]TablePlan, []DeferredFK, map[string]bool, error) {
	availableSet := makeStringSet(ordered)
	var allPlans []TablePlan
	var dfkList []DeferredFK

	for _, cycle := range trueCycleComponents {
		cyclePlans, cycleDFKs, resolveError := resolveCycle(schemaGraph, schemaModel, cycle, tableNodes, rowCount)
		if resolveError != nil {
			return nil, nil, nil, resolveError
		}
		allPlans = append(allPlans, cyclePlans...)
		dfkList = append(dfkList, cycleDFKs...)
		for _, nodeID := range cycle {
			availableSet[nodeID] = true
		}
	}

	return allPlans, dfkList, availableSet, nil
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

	ordered = append(ordered, breakpointFrom)
	seen[breakpointFrom] = true

	for _, id := range component {
		if !seen[id] && id != breakpointTo {
			ordered = append(ordered, id)
			seen[id] = true
		}
	}

	if !seen[breakpointTo] {
		ordered = append(ordered, breakpointTo)
	}

	return ordered
}
