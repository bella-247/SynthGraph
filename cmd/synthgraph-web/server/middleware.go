package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

const maxRequestBodySize = 10 << 20

var timeoutlessRoutes = map[string]bool{
	"/api/generate/stream": true,
}

type responseStatusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func newResponseStatusRecorder(responseWriter http.ResponseWriter) *responseStatusRecorder {
	return &responseStatusRecorder{
		ResponseWriter: responseWriter,
		statusCode:     http.StatusOK,
	}
}

func (recorder *responseStatusRecorder) WriteHeader(statusCode int) {
	if !recorder.written {
		recorder.statusCode = statusCode
		recorder.written = true
		recorder.ResponseWriter.WriteHeader(statusCode)
	}
}

func (recorder *responseStatusRecorder) Write(data []byte) (int, error) {
	if !recorder.written {
		recorder.written = true
	}
	return recorder.ResponseWriter.Write(data)
}

func (recorder *responseStatusRecorder) Flush() {
	if flusher, supportsFlushing := recorder.ResponseWriter.(http.Flusher); supportsFlushing {
		flusher.Flush()
	}
}

type logEntry struct {
	Time       string `json:"time"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"statusCode"`
	Duration   string `json:"duration"`
	RequestID  string `json:"requestId"`
}

func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		requestID := generateRequestID()
		responseWriter.Header().Set("X-Request-Id", requestID)
		recorder := newResponseStatusRecorder(responseWriter)
		next.ServeHTTP(recorder, request)
		duration := time.Since(startTime)
		entry := logEntry{
			Time:       startTime.UTC().Format(time.RFC3339Nano),
			Method:     request.Method,
			Path:       request.URL.Path,
			StatusCode: recorder.statusCode,
			Duration:   duration.String(),
			RequestID:  requestID,
		}
		encoded, marshalError := json.Marshal(entry)
		if marshalError != nil {
			log.Printf("failed to marshal log entry: %v", marshalError)
			return
		}
		log.Println(string(encoded))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered on %s %s: %v\n%s", request.Method, request.URL.Path, recovered, debug.Stack())
				writeError(responseWriter, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(responseWriter, request)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Access-Control-Allow-Origin", "*")
		responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		responseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
		responseWriter.Header().Set("Access-Control-Max-Age", "86400")
		if request.Method == http.MethodOptions {
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(responseWriter, request)
	})
}

func bodySizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Body != nil {
			request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodySize)
		}
		next.ServeHTTP(responseWriter, request)
	})
}

func timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if timeoutlessRoutes[request.URL.Path] {
			next.ServeHTTP(responseWriter, request)
			return
		}
		http.TimeoutHandler(next, DefaultTimeout, "request timed out").ServeHTTP(responseWriter, request)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("X-Content-Type-Options", "nosniff")
		responseWriter.Header().Set("X-Frame-Options", "DENY")
		responseWriter.Header().Set("X-XSS-Protection", "0")
		responseWriter.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		responseWriter.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(responseWriter, request)
	})
}
