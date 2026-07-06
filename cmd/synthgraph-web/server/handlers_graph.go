package server

import (
	"fmt"
	"net/http"

	"synthgraph/internal/graph"
	"synthgraph/internal/semantic"
)

// handleGraph accepts a schema.Model at POST /api/graph and returns a node/edge
// representation suitable for Cytoscape.js rendering. The model should come
// from a previous /api/parse call.
// Request body: {"model": {...}}
// Response: {"nodes": [...], "edges": [...]}
func (serverInstance *Server) handleGraph(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody parseResponse
	if decodeError := decodeJSONBody(request, &requestBody); decodeError != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid JSON: %v", decodeError)
		return
	}
	if requestBody.Model == nil {
		writeError(responseWriter, http.StatusBadRequest, "model is required")
		return
	}

	graphInstance, buildError := graph.Build(requestBody.Model)
	if buildError != nil {
		writeError(responseWriter, http.StatusInternalServerError, "graph build error: %v", buildError)
		return
	}

	junctionTables := identifyJunctionTables(graphInstance)
	tableColumns := countColumnsPerTable(graphInstance)

	var nodes []nodeJSON
	var edges []edgeJSON
	for _, graphNode := range graphInstance.NodeList {
		if graphNode.Kind != graph.NodeKindTable {
			continue
		}
		nodes = append(nodes, nodeJSON{
			ID:         graphNode.ID,
			Table:      graphNode.Label,
			Columns:    tableColumns[graphNode.ID],
			IsJunction: junctionTables[graphNode.ID],
		})
	}
	for _, graphEdge := range graphInstance.Edges {
		if graphEdge.Kind != graph.EdgeKindReferences {
			continue
		}
		foreignKeyMeta, hasMeta := graphEdge.Metadata.(*graph.FKMetadata)
		localColumnLabel := ""
		if hasMeta && len(foreignKeyMeta.LocalColumns) > 0 {
			localColumnLabel = foreignKeyMeta.LocalColumns[0]
		}
		isNullable := hasMeta && foreignKeyMeta.Cardinality == graph.CardinalityOneToOne
		edges = append(edges, edgeJSON{
			ID:       fmt.Sprintf("%s->%s", graphEdge.From, graphEdge.To),
			Source:   graphEdge.From,
			Target:   graphEdge.To,
			Label:    localColumnLabel,
			Nullable: isNullable,
		})
	}

	writeJSON(responseWriter, http.StatusOK, graphResponse{Nodes: nodes, Edges: edges})
}

// handleSemantic accepts a schema.Model at POST /api/semantic and returns
// inferred table roles and relationship kinds. The model should come
// from a previous /api/parse call.
// Request body: {"model": {...}}
// Response: {"nodes": [{"id": "...", "roles": ["entity"]}], "edges": [...]}
func (serverInstance *Server) handleSemantic(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody parseResponse
	if decodeError := decodeJSONBody(request, &requestBody); decodeError != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid JSON: %v", decodeError)
		return
	}
	if requestBody.Model == nil {
		writeError(responseWriter, http.StatusBadRequest, "model is required")
		return
	}

	graphInstance, buildError := graph.Build(requestBody.Model)
	if buildError != nil {
		writeError(responseWriter, http.StatusInternalServerError, "graph build error: %v", buildError)
		return
	}

	semanticGraph, semanticError := semantic.Build(graphInstance)
	if semanticError != nil {
		writeError(responseWriter, http.StatusInternalServerError, "semantic build error: %v", semanticError)
		return
	}

	var nodes []semNodeJSON
	var edges []semEdgeJSON
	for _, semanticNode := range semanticGraph.Nodes {
		roles := make([]string, len(semanticNode.Roles))
		for roleIndex, role := range semanticNode.Roles {
			roles[roleIndex] = string(role)
		}
		nodes = append(nodes, semNodeJSON{
			ID:    semanticNode.Label,
			Roles: roles,
		})
	}
	for _, relationship := range semanticGraph.Relationships {
		edges = append(edges, semEdgeJSON{
			From: relationship.From,
			To:   relationship.To,
			Kind: string(relationship.Kind),
		})
	}

	writeJSON(responseWriter, http.StatusOK, semanticResponse{Nodes: nodes, Edges: edges})
}

func identifyJunctionTables(graphInstance *graph.Graph) map[string]bool {
	junctionTables := make(map[string]bool)
	for _, graphEdge := range graphInstance.Edges {
		if graphEdge.Kind != graph.EdgeKindDependsOn {
			continue
		}
		foreignKeyMeta, isForeignKey := graphEdge.Metadata.(*graph.FKMetadata)
		if isForeignKey && foreignKeyMeta.Cardinality == graph.CardinalityManyToMany {
			junctionTables[graphEdge.From] = true
		}
	}
	return junctionTables
}

func countColumnsPerTable(graphInstance *graph.Graph) map[string]int {
	tableColumns := make(map[string]int)
	for _, graphEdge := range graphInstance.Edges {
		if graphEdge.Kind == graph.EdgeKindContains {
			tableColumns[graphEdge.From]++
		}
	}
	return tableColumns
}
