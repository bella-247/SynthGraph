package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func writeJSON(responseWriter http.ResponseWriter, statusCode int, value interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	json.NewEncoder(responseWriter).Encode(value)
}

func writeError(responseWriter http.ResponseWriter, statusCode int, format string, arguments ...interface{}) {
	writeJSON(responseWriter, statusCode, map[string]string{"error": fmt.Sprintf(format, arguments...)})
}
