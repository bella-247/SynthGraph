package server

import (
	"context"
	"log"
	"net/http"
	"time"
)

const (
	DefaultPort         = 8080
	DefaultTimeout      = 10 * time.Minute
	serverReadTimeout   = 30 * time.Second
	serverWriteTimeout  = 0
	serverIdleTimeout   = 120 * time.Second
	shutdownGracePeriod = 30 * time.Second
)

type Server struct {
	jobStore  *JobStore
	indexHTML string
	handler   http.Handler
	httpServer *http.Server
}

func New(indexHTML string, jobPersistPath string) *Server {
	var jobStore *JobStore
	if jobPersistPath != "" {
		jobStore = NewJobStoreWithPersistence(jobPersistPath)
	} else {
		jobStore = NewJobStore()
	}

	serverInstance := &Server{
		jobStore:  jobStore,
		indexHTML: indexHTML,
	}

	requestMux := http.NewServeMux()
	requestMux.HandleFunc("GET /api/jobs", serverInstance.handleListJobs)
	requestMux.HandleFunc("GET /api/jobs/{id}", serverInstance.handleGetJob)
	requestMux.HandleFunc("DELETE /api/jobs/{id}", serverInstance.handleDeleteJob)
	requestMux.HandleFunc("POST /api/parse", serverInstance.handleParse)
	requestMux.HandleFunc("POST /api/graph", serverInstance.handleGraph)
	requestMux.HandleFunc("POST /api/semantic", serverInstance.handleSemantic)
	requestMux.HandleFunc("POST /api/generate", serverInstance.handleGenerate)
	requestMux.HandleFunc("GET /api/generate/stream", serverInstance.handleGenerateStream)
	requestMux.HandleFunc("GET /api/health", serverInstance.handleHealth)
	requestMux.HandleFunc("GET /", serverInstance.handleFrontend)

	wrappedHandler := recoveryMiddleware(
		requestLoggingMiddleware(
			securityHeadersMiddleware(
				corsMiddleware(
					rateLimitMiddleware(
						timeoutMiddleware(
							bodySizeLimitMiddleware(requestMux),
						),
					),
				),
			),
		),
	)

	serverInstance.handler = wrappedHandler
	return serverInstance
}

func (serverInstance *Server) Handler() http.Handler {
	return serverInstance.handler
}

func (serverInstance *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	serverInstance.handler.ServeHTTP(responseWriter, request)
}

func (serverInstance *Server) ListenAndServe(address string) error {
	serverInstance.httpServer = &http.Server{
		Addr:         address,
		Handler:      serverInstance,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}
	log.Printf("synthgraph-web running at http://localhost%s", address)
	return serverInstance.httpServer.ListenAndServe()
}

func (serverInstance *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	log.Print("shutting down server...")
	return serverInstance.httpServer.Shutdown(ctx)
}
