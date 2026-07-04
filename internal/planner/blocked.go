package planner

import (
	"fmt"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

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

	var remaining []string
	for _, id := range blockedNodeIDs {
		if !availableSet[id] {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) > 0 {
		return nil, &PlanError{
			Message:     fmt.Sprintf("cannot resolve %d table(s) after processing all cycles — they depend on unresolvable cycles", len(remaining)),
			CycleTables: remaining,
			Hint:        "review the table dependencies; a larger cycle may involve these tables indirectly",
		}
	}

	return plans, nil
}
