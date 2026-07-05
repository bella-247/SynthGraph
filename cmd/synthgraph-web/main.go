package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"synthgraph/cmd/synthgraph-web/server"
)

//go:embed index.html
var indexHTML string

func main() {
	port := flag.Int("port", server.DefaultPort, "HTTP server port")
	jobPersistPath := flag.String("job-file", "synthgraph-jobs.json", "path to job persistence file (empty for in-memory only)")
	flag.Parse()

	serverInstance := server.New(indexHTML, *jobPersistPath)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChannel
		if shutdownError := serverInstance.Shutdown(); shutdownError != nil {
			fmt.Fprintf(os.Stderr, "shutdown error: %v\n", shutdownError)
		}
	}()

	address := fmt.Sprintf(":%d", *port)
	if listenError := serverInstance.ListenAndServe(address); listenError != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", listenError)
		os.Exit(1)
	}
}
