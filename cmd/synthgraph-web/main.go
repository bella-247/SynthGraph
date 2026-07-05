package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	mux := http.NewServeMux()
	server := newServer()

	mux.HandleFunc("GET /api/jobs", server.handleListJobs)
	mux.HandleFunc("POST /api/parse", server.handleParse)
	mux.HandleFunc("POST /api/graph", server.handleGraph)
	mux.HandleFunc("POST /api/semantic", server.handleSemantic)
	mux.HandleFunc("POST /api/generate", server.handleGenerate)
	mux.HandleFunc("GET /", server.handleFrontend)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("synthgraph-web running at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
