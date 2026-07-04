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
					References: tableData.Name,
					RefColumn:  refCol,
				})
			}
		}

		plans = append(plans, plan)
	}

	// Fix ReferencedTable in DeferredFK entries: it should point to the
	// referenced table, not the current table.
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
