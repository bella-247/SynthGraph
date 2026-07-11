package server

import (
	"context"
	"encoding/json"
	"errors"
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
	"synthgraph/internal/parser"
	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/planner"
	"synthgraph/internal/semantic"
)

type streamState struct {
	responseWriter http.ResponseWriter
	flusher        http.Flusher
}

func newStreamState(responseWriter http.ResponseWriter) (*streamState, bool) {
	flusher, supportsFlushing := responseWriter.(http.Flusher)
	if !supportsFlushing {
		return nil, false
	}
	return &streamState{
		responseWriter: responseWriter,
		flusher:        flusher,
	}, true
}

func (stream *streamState) sendEvent(eventType string, data interface{}) {
	jsonData, marshalError := json.Marshal(data)
	if marshalError != nil {
		log.Printf("SSE marshal error: %v", marshalError)
		return
	}
	fmt.Fprintf(stream.responseWriter, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	stream.flusher.Flush()
}

func (stream *streamState) sendError(message string) {
	stream.sendEvent("error", map[string]string{"message": message})
}

// handleGenerateStream runs the full pipeline at GET /api/generate/stream
// and streams progress events via Server-Sent Events.
//
// Query params: input, rows, seed, format, schema_name
//
// Events emitted in order:
//
//	event: stage     data: {"stage":"parsing","status":"processing"}
//	event: stage     data: {"stage":"parsing","status":"done"}
//	event: stage     data: {"stage":"graphing","status":"processing"}
//	event: stage     data: {"stage":"graphing","status":"done"}
//	event: stage     data: {"stage":"semantic","status":"processing"}
//	event: stage     data: {"stage":"semantic","status":"done"}
//	event: stage     data: {"stage":"planning","status":"processing"}
//	event: stage     data: {"stage":"planning","status":"done"}
//	event: stage     data: {"stage":"generating","status":"processing"}
//	event: progress  data: {"table":"users","current":1,"total":5}
//	event: progress  data: {"table":"orders","current":2,"total":5}
//	event: stage     data: {"stage":"generating","status":"done"}
//	event: stage     data: {"stage":"exporting","status":"processing"}
//	event: stage     data: {"stage":"exporting","status":"done"}
//	event: complete  data: {"job_id":1,"tables":5,"data":"...","format":"csv"}
//	event: error     data: {"message":"parse error: ..."}
func (server *Server) handleGenerateStream(responseWriter http.ResponseWriter, request *http.Request) {
	flusher, supportsFlushing := responseWriter.(http.Flusher)
	if !supportsFlushing {
		writeError(responseWriter, http.StatusInternalServerError, "streaming not supported")
		return
	}

	responseWriter.Header().Set("Content-Type", "text/event-stream")
	responseWriter.Header().Set("Cache-Control", "no-cache")
	responseWriter.Header().Set("Connection", "keep-alive")

	stream := &streamState{
		responseWriter: responseWriter,
		flusher:        flusher,
	}

	rawSQL := request.URL.Query().Get("input")
	rowCount, err := strconv.Atoi(request.URL.Query().Get("rows"))
	if err != nil {
		log.Printf("invalid rows parameter %q, defaulting to 10", request.URL.Query().Get("rows"))
	}
	seed, err := strconv.ParseInt(request.URL.Query().Get("seed"), 10, 64)
	if err != nil {
		log.Printf("invalid seed parameter %q, defaulting to 0", request.URL.Query().Get("seed"))
	}
	outputFormat := request.URL.Query().Get("format")
	schemaName := request.URL.Query().Get("schema_name")

	if strings.TrimSpace(rawSQL) == "" {
		stream.sendError("input (SQL) is required")
		return
	}
	if len(rawSQL) > maxSchemaBodySize {
		stream.sendError(fmt.Sprintf("schema too large (%d bytes, max %d)", len(rawSQL), maxSchemaBodySize))
		return
	}
	if rowCount <= 0 {
		rowCount = 10
	}
	if rowCount > 100000 {
		stream.sendError("rows exceeds maximum (100000)")
		return
	}
	if outputFormat == "" {
		outputFormat = "csv"
	}

	// Compute a fingerprint so reconnecting EventSource clients
	// get the cached result instead of re-running the pipeline.
	reqKey := generationFingerprint(rawSQL, rowCount, seed, outputFormat, schemaName)
	if cached := server.getCachedStreamResult(reqKey); cached != nil {
		log.Printf("serving cached stream result for key=%s (job %d)", reqKey[:12], cached.jobID)
		stream.sendEvent("stage", map[string]string{"stage": "parsing", "status": "done"})
		stream.sendEvent("stage", map[string]string{"stage": "graphing", "status": "done"})
		stream.sendEvent("stage", map[string]string{"stage": "semantic", "status": "done"})
		stream.sendEvent("stage", map[string]string{"stage": "planning", "status": "done"})
		stream.sendEvent("stage", map[string]string{"stage": "generating", "status": "done"})
		stream.sendEvent("stage", map[string]string{"stage": "exporting", "status": "done"})
		stream.sendEvent("complete", map[string]interface{}{
			"job_id": cached.jobID,
			"tables": cached.tables,
			"errors": cached.errors,
			"data":   string(cached.data),
			"format": cached.format,
		})
		return
	}

	requestContext := request.Context()
	requestContext, cancel := context.WithTimeout(requestContext, DefaultTimeout)
	defer cancel()

	stream.sendEvent("stage", map[string]string{"stage": "parsing", "status": "processing"})

	parsedModel, parseError := postgresql.New().Parse([]byte(rawSQL))
	if parseError != nil {
		if pe := (*parser.ParseError)(nil); errors.As(parseError, &pe) {
			stream.sendError(pe.Error())
		} else {
			stream.sendError(fmt.Sprintf("parse error: %v", parseError))
		}
		return
	}
	stream.sendEvent("stage", map[string]string{"stage": "parsing", "status": "done"})
	if contextCancelled(requestContext) {
		stream.sendError("request cancelled")
		return
	}

	stream.sendEvent("stage", map[string]string{"stage": "graphing", "status": "processing"})
	graphInstance, graphError := graph.Build(parsedModel)
	if graphError != nil {
		stream.sendError(fmt.Sprintf("graph build error: %v", graphError))
		return
	}
	stream.sendEvent("stage", map[string]string{"stage": "graphing", "status": "done"})
	if contextCancelled(requestContext) {
		stream.sendError("request cancelled")
		return
	}

	stream.sendEvent("stage", map[string]string{"stage": "semantic", "status": "processing"})
	semantic.ResolveColumns(parsedModel)

	semanticGraph, semanticError := semantic.Build(graphInstance)
	if semanticError != nil {
		stream.sendError(fmt.Sprintf("semantic build error: %v", semanticError))
		return
	}
	stream.sendEvent("stage", map[string]string{"stage": "semantic", "status": "done"})
	if contextCancelled(requestContext) {
		stream.sendError("request cancelled")
		return
	}

	stream.sendEvent("stage", map[string]string{"stage": "planning", "status": "processing"})
	generationPlan, planError := planner.BuildPlan(graphInstance, parsedModel, rowCount)
	if planError != nil {
		stream.sendError(fmt.Sprintf("plan build error: %v", planError))
		return
	}
	stream.sendEvent("stage", map[string]string{"stage": "planning", "status": "done"})
	if contextCancelled(requestContext) {
		stream.sendError("request cancelled")
		return
	}

	stream.sendEvent("stage", map[string]string{"stage": "generating", "status": "processing"})

	generationContext := &generator.GenerationContext{
		Progress: func(tableName string, current int, total int) {
			stream.sendEvent("progress", map[string]interface{}{
				"table":   tableName,
				"current": current,
				"total":   total,
			})
		},
		GlobalSeed:    uint64(seed),
		Model:         parsedModel,
		Graph:         graphInstance,
		SemanticGraph: semanticGraph,
		Context:       requestContext,
	}

	dataset, generationError := generator.Generate(generationPlan, generationContext)

	partialErrors := make([]string, 0)
	if dataset != nil {
		for _, partialError := range dataset.Errors {
			partialErrors = append(partialErrors, fmt.Sprintf("%s: %v", partialError.Table, partialError.Err))
		}
	}

	if generationError != nil && (dataset == nil || len(dataset.Tables) == 0) {
		stream.sendError(fmt.Sprintf("generation failed: %v", generationError))
		return
	}
	stream.sendEvent("stage", map[string]string{"stage": "generating", "status": "done"})
	if contextCancelled(requestContext) {
		stream.sendError("request cancelled")
		return
	}

	stream.sendEvent("stage", map[string]string{"stage": "exporting", "status": "processing"})

	var exportedData []byte
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer pipeWriter.Close()
		// If the request is cancelled, stop exporting and close the pipe.
		select {
		case <-requestContext.Done():
			return
		default:
		}
		exportConfig := &exporter.ExportConfig{
			SchemaName:    schemaName,
			IncludeHeader: true,
		}
		var exportError error
		switch outputFormat {
		case "sql":
			exportError = exporter.ExportSQL(pipeWriter, dataset, parsedModel, exportConfig)
		default:
			exportError = exporter.ExportCSV(pipeWriter, dataset, parsedModel, exportConfig)
		}
		pipeWriter.CloseWithError(exportError)
	}()
	exportedData, readError := io.ReadAll(pipeReader)
	if readError != nil {
		stream.sendError(fmt.Sprintf("export error: %v", readError))
		return
	}
	stream.sendEvent("stage", map[string]string{"stage": "exporting", "status": "done"})

	job := &Job{
		Status:  "completed",
		Created: time.Now(),
		Config: generationRequest{
			Input:      rawSQL,
			Rows:       rowCount,
			Seed:       seed,
			Format:     outputFormat,
			SchemaName: schemaName,
		},
		Tables: len(dataset.Tables),
		Errors: partialErrors,
		Data:   exportedData,
		Format: outputFormat,
	}
	server.jobStore.Add(job)

	// Cache the result for reconnecting EventSource clients.
	server.cacheStreamResult(reqKey, &streamCacheEntry{
		jobID:   job.ID,
		tables:  len(dataset.Tables),
		errors:  partialErrors,
		data:    exportedData,
		format:  outputFormat,
		expires: time.Now().Add(streamCacheTTL),
	})

	stream.sendEvent("complete", map[string]interface{}{
		"job_id": job.ID,
		"tables": len(dataset.Tables),
		"errors": partialErrors,
		"data":   string(exportedData),
		"format": outputFormat,
	})
}

func contextCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
