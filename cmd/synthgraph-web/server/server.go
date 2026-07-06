package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	DefaultPort         = 8080
	DefaultTimeout      = 10 * time.Minute
	serverReadTimeout   = 30 * time.Second
	serverWriteTimeout  = 0
	serverIdleTimeout   = 120 * time.Second
	shutdownGracePeriod = 30 * time.Second
	streamCacheTTL      = 60 * time.Second
)

type streamCacheEntry struct {
	jobID   int
	tables  int
	errors  []string
	data    []byte
	format  string
	expires time.Time
}

type Server struct {
	jobStore        *JobStore
	indexHTML       string
	handler         http.Handler
	httpServer      *http.Server
	streamCache     sync.Map
	streamCacheMu   sync.Mutex
	streamCacheDone chan struct{}
}

func New(indexHTML string, jobPersistPath string) *Server {
	var jobStore *JobStore
	if jobPersistPath != "" {
		jobStore = NewJobStoreWithPersistence(jobPersistPath)
	} else {
		jobStore = NewJobStore()
	}

	server := &Server{
		jobStore:        jobStore,
		indexHTML:       indexHTML,
		streamCacheDone: make(chan struct{}),
	}

	go server.evictStreamCacheLoop()

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

// generationFingerprint creates a unique key for a stream request
// so reconnecting EventSource clients can be served from cache.
func generationFingerprint(rawSQL string, rowCount int, seed int64, format, schemaName string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%s|%s", rawSQL, rowCount, seed, format, schemaName)))
	return fmt.Sprintf("%x", hash[:16])
}

func (server *Server) cacheStreamResult(key string, entry *streamCacheEntry) {
	server.streamCacheMu.Lock()
	defer server.streamCacheMu.Unlock()
	server.streamCache.Store(key, entry)
}

func (server *Server) getCachedStreamResult(key string) *streamCacheEntry {
	value, loaded := server.streamCache.Load(key)
	if !loaded {
		return nil
	}
	entry := value.(*streamCacheEntry)
	if time.Now().After(entry.expires) {
		server.streamCache.Delete(key)
		return nil
	}
	return entry
}

func (server *Server) evictStreamCacheLoop() {
	ticker := time.NewTicker(streamCacheTTL / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			server.streamCache.Range(func(key, value interface{}) bool {
				entry := value.(*streamCacheEntry)
				if now.After(entry.expires) {
					server.streamCache.Delete(key)
				}
				return true
			})
		case <-server.streamCacheDone:
			return
		}
	}
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
	close(server.streamCacheDone)
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	log.Print("shutting down server...")
	return server.httpServer.Shutdown(ctx)
}
