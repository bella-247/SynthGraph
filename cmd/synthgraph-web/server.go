package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"synthgraph/internal/exporter"
	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

//go:embed index.html
var indexHTML string

type server struct {
	mu   sync.Mutex
	jobs []*job
	nextID int
}

type job struct {
	ID       int       `json:"id"`
	Status   string    `json:"status"`
	Created  time.Time `json:"created"`
	Config   genRequest `json:"config"`
	Tables   int       `json:"tables"`
	Errors   []string  `json:"errors,omitempty"`
	Data     []byte    `json:"-"`
	Format   string    `json:"format"`
}

type parseRequest struct {
	SQL string `json:"sql"`
}

type parseResponse struct {
	Tables  int              `json:"tables"`
	Enums   int              `json:"enums"`
	Model   *schema.Model    `json:"model"`
	Warnings []string        `json:"warnings,omitempty"`
}

type graphResponse struct {
	Nodes []nodeJSON  `json:"nodes"`
	Edges []edgeJSON  `json:"edges"`
}

type nodeJSON struct {
	ID       string `json:"id"`
	Table    string `json:"table"`
	Columns  int    `json:"columns"`
	IsJunction bool `json:"is_junction"`
}

type edgeJSON struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Label    string `json:"label"`
	Nullable bool   `json:"nullable"`
}

type semanticResponse struct {
	Nodes []semNodeJSON `json:"nodes"`
	Edges []semEdgeJSON `json:"edges"`
}

type semNodeJSON struct {
	ID     string   `json:"id"`
	Roles  []string `json:"roles"`
}

type semEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type genRequest struct {
	Input      string `json:"input"`
	Rows       int    `json:"rows"`
	Seed       int64  `json:"seed"`
	Format     string `json:"format"`
	SchemaName string `json:"schema_name"`
}

func newServer() *server {
	return &server{
		jobs:   make([]*job, 0),
		nextID: 1,
	}
}

func (s *server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func (s *server) handleParse(w http.ResponseWriter, r *http.Request) {
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}
	if strings.TrimSpace(req.SQL) == "" {
		writeError(w, http.StatusBadRequest, "sql field is required")
		return
	}

	model, err := postgresql.New().Parse([]byte(req.SQL))
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse error: %v", err)
		return
	}

	var warnings []string
	if validationErrors := schema.Validate(model); len(validationErrors) > 0 {
		for _, ve := range validationErrors {
			warnings = append(warnings, ve.Error())
		}
	}

	writeJSON(w, http.StatusOK, parseResponse{
		Tables:   len(model.Tables),
		Enums:    len(model.Enums),
		Model:    model,
		Warnings: warnings,
	})
}

func (s *server) handleGraph(w http.ResponseWriter, r *http.Request) {
	var req parseResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}
	if req.Model == nil {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	g, err := graph.Build(req.Model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graph build error: %v", err)
		return
	}

	// Identify junction tables (have a depends_on edge with many_to_many cardinality)
	junctionTables := make(map[string]bool)
	for _, edge := range g.Edges {
		if edge.Kind == graph.EdgeKindDependsOn {
			if fkMeta, ok := edge.Metadata.(*graph.FKMetadata); ok && fkMeta.Cardinality == graph.CardinalityManyToMany {
				junctionTables[edge.From] = true
			}
		}
	}

	// Count columns per table (nodes reachable via contains edges)
	tableCols := make(map[string]int)
	for _, edge := range g.Edges {
		if edge.Kind == graph.EdgeKindContains {
			tableCols[edge.From]++
		}
	}

	var nodes []nodeJSON
	var edges []edgeJSON
	for _, node := range g.NodeList {
		if node.Kind != graph.NodeKindTable {
			continue
		}
		nodes = append(nodes, nodeJSON{
			ID:          node.ID,
			Table:       node.Label,
			Columns:     tableCols[node.ID],
			IsJunction:  junctionTables[node.ID],
		})
	}
	for _, edge := range g.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}
		fkMeta, _ := edge.Metadata.(*graph.FKMetadata)
		localCols := ""
		if fkMeta != nil && len(fkMeta.LocalColumns) > 0 {
			localCols = fkMeta.LocalColumns[0]
		}
		edges = append(edges, edgeJSON{
			ID:       fmt.Sprintf("%s->%s", edge.From, edge.To),
			Source:   edge.From,
			Target:   edge.To,
			Label:    localCols,
			Nullable: fkMeta != nil && fkMeta.Cardinality == graph.CardinalityOneToOne,
		})
	}

	writeJSON(w, http.StatusOK, graphResponse{Nodes: nodes, Edges: edges})
}

func (s *server) handleSemantic(w http.ResponseWriter, r *http.Request) {
	var req parseResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}
	if req.Model == nil {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	g, err := graph.Build(req.Model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graph build error: %v", err)
		return
	}

	sg, err := semantic.Build(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "semantic build error: %v", err)
		return
	}

	var nodes []semNodeJSON
	var edges []semEdgeJSON
	for _, sn := range sg.Nodes {
		roles := make([]string, len(sn.Roles))
		for j, r := range sn.Roles {
			roles[j] = string(r)
		}
		nodes = append(nodes, semNodeJSON{
			ID:    sn.Label,
			Roles: roles,
		})
	}
	for _, rel := range sg.Relationships {
		edges = append(edges, semEdgeJSON{
			From: rel.From,
			To:   rel.To,
			Kind: string(rel.Kind),
		})
	}

	writeJSON(w, http.StatusOK, semanticResponse{Nodes: nodes, Edges: edges})
}

func (s *server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req genRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}
	if req.Input == "" {
		writeError(w, http.StatusBadRequest, "input (SQL) is required")
		return
	}
	if req.Rows <= 0 {
		req.Rows = 10
	}
	if req.Format == "" {
		req.Format = "csv"
	}

	model, err := postgresql.New().Parse([]byte(req.Input))
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse error: %v", err)
		return
	}

	g, err := graph.Build(model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graph build error: %v", err)
		return
	}

	sg, err := semantic.Build(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "semantic build error: %v", err)
		return
	}

	plan, err := planner.BuildPlan(g, model, req.Rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan build error: %v", err)
		return
	}

	genCtx := &generator.GenerationContext{
		GlobalSeed:    uint64(req.Seed),
		Model:         model,
		Graph:         g,
		SemanticGraph: sg,
	}

	dataset, err := generator.Generate(plan, genCtx)
	if err != nil && dataset == nil {
		writeError(w, http.StatusInternalServerError, "generation failed: %v", err)
		return
	}

	// Collect partial errors
	var partialErrors []string
	if dataset != nil {
		for _, pe := range dataset.Errors {
			partialErrors = append(partialErrors, fmt.Sprintf("%s: %v", pe.Table, pe.Err))
		}
	}

	// Export to CSV string
	reader, writer := io.Pipe()
	go func() {
		exportCfg := &exporter.ExportConfig{
			SchemaName:   req.SchemaName,
			IncludeHeader: true,
		}
		var exErr error
		switch req.Format {
		case "sql":
			exErr = exporter.ExportSQL(writer, dataset, model, exportCfg)
		default:
			exErr = exporter.ExportCSV(writer, dataset, model, exportCfg)
		}
		writer.CloseWithError(exErr)
	}()

	exported, readErr := io.ReadAll(reader)
	if readErr != nil {
		writeError(w, http.StatusInternalServerError, "export error: %v", readErr)
		return
	}

	// Create job record
	s.mu.Lock()
	j := &job{
		ID:      s.nextID,
		Status:  "completed",
		Created: time.Now(),
		Config:  req,
		Tables:  len(dataset.Tables),
		Errors:  partialErrors,
		Data:    exported,
		Format:  req.Format,
	}
	s.nextID++
	s.jobs = append(s.jobs, j)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"job_id":   j.ID,
		"status":   "completed",
		"tables":   len(dataset.Tables),
		"errors":   partialErrors,
		"data":     string(exported),
		"format":   req.Format,
	})
}

func (s *server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type jobSummary struct {
		ID      int       `json:"id"`
		Status  string    `json:"status"`
		Created time.Time `json:"created"`
		Tables  int       `json:"tables"`
		Format  string    `json:"format"`
		Rows    int       `json:"rows"`
		Errors  []string  `json:"errors,omitempty"`
	}

	summaries := make([]jobSummary, len(s.jobs))
	for i, j := range s.jobs {
		summaries[i] = jobSummary{
			ID:      j.ID,
			Status:  j.Status,
			Created: j.Created,
			Tables:  j.Tables,
			Format:  j.Format,
			Rows:    j.Config.Rows,
			Errors:  j.Errors,
		}
	}
	// Newest first
	for i, j := 0, len(summaries)-1; i < j; i, j = i+1, j-1 {
		summaries[i], summaries[j] = summaries[j], summaries[i]
	}

	writeJSON(w, http.StatusOK, summaries)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, format string, args ...interface{}) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}
