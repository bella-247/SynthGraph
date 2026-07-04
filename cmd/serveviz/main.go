package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"synthgraph/internal/graph"
	pg "synthgraph/internal/parser/postgresql"
)

//go:embed index.html
var htmlContent string

func main() {
	schemaFile := flag.String("schema", "", "Path to SQL schema file")
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	if *schemaFile == "" {
		log.Fatal("--schema flag is required")
	}

	schemaBytes, readError := os.ReadFile(*schemaFile)
	if readError != nil {
		log.Fatalf("Cannot read schema file: %v", readError)
	}

	schemaModel, translateError := pg.New().Parse(schemaBytes)
	if translateError != nil {
		log.Fatalf("Failed to parse schema: %v", translateError)
	}

	schemaGraph, buildError := graph.Build(schemaModel)
	if buildError != nil {
		log.Fatalf("Failed to build graph: %v", buildError)
	}

	graphJSON, marshalError := json.Marshal(schemaGraph)
	if marshalError != nil {
		log.Fatalf("Failed to serialize graph: %v", marshalError)
	}

	http.HandleFunc("/api/graph", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Write(graphJSON)
	})

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Write([]byte(htmlContent))
	})

	address := fmt.Sprintf(":%d", *port)
	log.Printf("SynthGraph Viz running at http://localhost%s", address)
	log.Printf("Visualizing: %s", *schemaFile)
	log.Fatal(http.ListenAndServe(address, nil))
}
