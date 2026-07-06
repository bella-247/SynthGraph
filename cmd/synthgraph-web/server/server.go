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

	server := &Server{
		jobStore:  jobStore,
		indexHTML: indexHTML,
	}

	requestMux := http.NewServeMux()
	requestMux.HandleFunc("GET /api/jobs", server.handleListJobs)
	requestMux.HandleFunc("GET /api/jobs/{id}", server.handleGetJob)
	requestMux.HandleFunc("DELETE /api/jobs/{id}", server.handleDeleteJob)
	requestMux.HandleFunc("POST /api/parse", server.handleParse)
	requestMux.HandleFunc("POST /api/graph", server.handleGraph)
	requestMux.HandleFunc("POST /api/semantic", server.handleSemantic)
	requestMux.HandleFunc("POST /api/generate", server.handleGenerate)
	requestMux.HandleFunc("GET /api/generate/stream", server.handleGenerateStream)
	requestMux.HandleFunc("GET /api/health", server.handleHealth)
	requestMux.HandleFunc("GET /", server.handleFrontend)

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

	server.handler = wrappedHandler
	return server
}

func (server *Server) Handler() http.Handler {
	return server.handler
}

func (server *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	server.handler.ServeHTTP(responseWriter, request)
}

func (server *Server) ListenAndServe(address string) error {
	server.httpServer = &http.Server{
		Addr:         address,
		Handler:      server,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}
	log.Printf("synthgraph-web running at http://localhost%s", address)
	return server.httpServer.ListenAndServe()
}

func (server *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	log.Print("shutting down server...")
	return server.httpServer.Shutdown(ctx)
}
