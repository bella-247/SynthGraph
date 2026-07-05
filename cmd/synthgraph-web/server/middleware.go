package server

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

const maxRequestBodySize = 10 << 20

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

func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		recorder := newResponseStatusRecorder(responseWriter)
		next.ServeHTTP(recorder, request)
		duration := time.Since(startTime)
		log.Printf("%s %s %d %v", request.Method, request.URL.Path, recorder.statusCode, duration)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v\n%s", recovered, debug.Stack())
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
		responseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
