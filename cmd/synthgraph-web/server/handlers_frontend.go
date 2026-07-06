package server

import (
	"net/http"
	"runtime"
	"strings"
	"time"

	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/schema"
)

var serverStartTime = time.Now()

// handleStyles serves the embedded CSS at GET /styles.css.
func (server *Server) handleStyles(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "text/css; charset=utf-8")
	responseWriter.Write([]byte(server.stylesCSS))
}

// handleAppJS serves the embedded JS at GET /app.js.
func (server *Server) handleAppJS(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	responseWriter.Write([]byte(server.appJS))
}

// handleFrontend serves the embedded SPA at GET /.
// Any path other than "/" returns a 404.
func (server *Server) handleFrontend(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(responseWriter, request)
		return
	}
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.Write([]byte(server.indexHTML))
}

// handleHealth returns a detailed status check at GET /api/health.
// Orchestrators (Kubernetes, Nomad) should use this for liveness probes.
// Returns version, uptime, goroutine count, and job count.
func (server *Server) handleHealth(responseWriter http.ResponseWriter, request *http.Request) {
	uptime := time.Since(serverStartTime).Truncate(time.Second).String()
	jobs := server.jobStore.List()
	writeJSON(responseWriter, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"version":  "0.1.0",
		"uptime":   uptime,
		"goroutines": runtime.NumGoroutine(),
		"jobs":     len(jobs),
	})
}

// handleParse accepts SQL DDL at POST /api/parse and returns the parsed schema.Model.
// The model includes all tables, columns, enums, and foreign keys extracted from the SQL.
// Request body: {"sql": "CREATE TABLE ..."}
// Response: {"tables": 3, "enums": 1, "model": {...}, "warnings": [...]}
func (server *Server) handleParse(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody parseRequest
	if decodeError := decodeJSONBody(request, &requestBody); decodeError != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid JSON: %v", decodeError)
		return
	}
	if strings.TrimSpace(requestBody.SQL) == "" {
		writeError(responseWriter, http.StatusBadRequest, "sql field is required")
		return
	}

	parsedModel, parseError := postgresql.New().Parse([]byte(requestBody.SQL))
	if parseError != nil {
		writeError(responseWriter, http.StatusBadRequest, "parse error: %v", parseError)
		return
	}

	var warnings []string
	if validationErrors := schema.Validate(parsedModel); len(validationErrors) > 0 {
		for _, validationError := range validationErrors {
			warnings = append(warnings, validationError.Error())
		}
	}

	writeJSON(responseWriter, http.StatusOK, parseResponse{
		Tables:   len(parsedModel.Tables),
		Enums:    len(parsedModel.Enums),
		Model:    parsedModel,
		Warnings: warnings,
	})
}
