package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"synthgraph/internal/exporter"
	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

func runGenerationPipeline(rawSQL string, rowCount int, seed int64) (*generator.Dataset, *schema.Model, error) {
	parsedModel, parseError := postgresql.New().Parse([]byte(rawSQL))
	if parseError != nil {
		return nil, nil, fmt.Errorf("parse error: %w", parseError)
	}

	graphInstance, graphError := graph.Build(parsedModel)
	if graphError != nil {
		return nil, nil, fmt.Errorf("graph build error: %w", graphError)
	}

	semanticGraph, semanticError := semantic.Build(graphInstance)
	if semanticError != nil {
		return nil, nil, fmt.Errorf("semantic build error: %w", semanticError)
	}

	generationPlan, planError := planner.BuildPlan(graphInstance, parsedModel, rowCount)
	if planError != nil {
		return nil, nil, fmt.Errorf("plan build error: %w", planError)
	}

	generationContext := &generator.GenerationContext{
		GlobalSeed:    uint64(seed),
		Model:         parsedModel,
		Graph:         graphInstance,
		SemanticGraph: semanticGraph,
	}

	dataset, generationError := generator.Generate(generationPlan, generationContext)
	if generationError != nil && dataset == nil {
		return nil, nil, fmt.Errorf("generation failed: %w", generationError)
	}

	return dataset, parsedModel, nil
}

func exportDataset(dataset *generator.Dataset, model *schema.Model, format string, schemaName string) ([]byte, error) {
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		exportConfig := &exporter.ExportConfig{
			SchemaName:   schemaName,
			IncludeHeader: true,
		}
		var exportError error
		switch format {
		case "sql":
			exportError = exporter.ExportSQL(pipeWriter, dataset, model, exportConfig)
		default:
			exportError = exporter.ExportCSV(pipeWriter, dataset, model, exportConfig)
		}
		pipeWriter.CloseWithError(exportError)
	}()

	exportedData, readError := io.ReadAll(pipeReader)
	if readError != nil {
		return nil, fmt.Errorf("export error: %w", readError)
	}
	return exportedData, nil
}

func collectPartialErrors(dataset *generator.Dataset) []string {
	var errors []string
	if dataset == nil {
		return errors
	}
	for _, partialError := range dataset.Errors {
		errors = append(errors, fmt.Sprintf("%s: %v", partialError.Table, partialError.Err))
	}
	return errors
}

// handleGenerate runs the full pipeline at POST /api/generate:
// parse → graph → semantic → plan → generate → export.
// On success it stores a job record and returns the exported data.
// Request body: {"input": "CREATE TABLE ...", "rows": 10, "seed": 42, "format": "csv"}
// Response: {"job_id": 1, "status": "completed", "tables": 3, "data": "...", "format": "csv"}
func (server *Server) handleGenerate(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody generationRequest
	if decodeError := decodeJSONBody(request, &requestBody); decodeError != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid JSON: %v", decodeError)
		return
	}
	if strings.TrimSpace(requestBody.Input) == "" {
		writeError(responseWriter, http.StatusBadRequest, "input (SQL) is required")
		return
	}
	if requestBody.Rows <= 0 {
		requestBody.Rows = 10
	}
	if requestBody.Rows > 100000 {
		writeError(responseWriter, http.StatusBadRequest, "rows exceeds maximum (100000)")
		return
	}
	if requestBody.Format == "" {
		requestBody.Format = "csv"
	}

	dataset, parsedModel, pipelineError := runGenerationPipeline(requestBody.Input, requestBody.Rows, requestBody.Seed)
	if pipelineError != nil {
		writeError(responseWriter, http.StatusInternalServerError, "%v", pipelineError)
		return
	}

	exportedData, exportError := exportDataset(dataset, parsedModel, requestBody.Format, requestBody.SchemaName)
	if exportError != nil {
		writeError(responseWriter, http.StatusInternalServerError, "%v", exportError)
		return
	}

	partialErrors := collectPartialErrors(dataset)

	job := &Job{
		Status:  "completed",
		Created: time.Now(),
		Config:  requestBody,
		Tables:  len(dataset.Tables),
		Errors:  partialErrors,
		Data:    exportedData,
		Format:  requestBody.Format,
	}
	server.jobStore.Add(job)

	writeJSON(responseWriter, http.StatusOK, map[string]interface{}{
		"job_id":  job.ID,
		"status":  "completed",
		"tables":  len(dataset.Tables),
		"errors":  partialErrors,
		"data":    string(exportedData),
		"format":  requestBody.Format,
	})
}

func (server *Server) handleListJobs(responseWriter http.ResponseWriter, request *http.Request) {
	summaries := server.jobStore.List()
	writeJSON(responseWriter, http.StatusOK, summaries)
}

func (server *Server) handleGetJob(responseWriter http.ResponseWriter, request *http.Request) {
	jobIDString := request.PathValue("id")
	jobID, parseError := strconv.Atoi(jobIDString)
	if parseError != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid job ID: %s", jobIDString)
		return
	}

	job := server.jobStore.GetByID(jobID)
	if job == nil {
		writeError(responseWriter, http.StatusNotFound, "job %d not found", jobID)
		return
	}

	writeJSON(responseWriter, http.StatusOK, &jobDetail{
		ID:     job.ID,
		Status: job.Status,
		Created: job.Created,
		Config: job.Config,
		Tables: job.Tables,
		Errors: job.Errors,
		Data:   string(job.Data),
		Format: job.Format,
	})
}

func (server *Server) identifyJunctionTables(graphInstance *graph.Graph) map[string]bool {
	junctionTables := make(map[string]bool)
	for _, graphEdge := range graphInstance.Edges {
		if graphEdge.Kind != graph.EdgeKindDependsOn {
			continue
		}
		foreignKeyMeta, isFK := graphEdge.Metadata.(*graph.FKMetadata)
		if isFK && foreignKeyMeta.Cardinality == graph.CardinalityManyToMany {
			junctionTables[graphEdge.From] = true
		}
	}
	return junctionTables
}

func (server *Server) countColumnsPerTable(graphInstance *graph.Graph) map[string]int {
	tableColumns := make(map[string]int)
	for _, graphEdge := range graphInstance.Edges {
		if graphEdge.Kind == graph.EdgeKindContains {
			tableColumns[graphEdge.From]++
		}
	}
	return tableColumns
}

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}
