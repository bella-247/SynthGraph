package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"synthgraph/internal/parser"
	"synthgraph/internal/parser/postgresql"
	"synthgraph/internal/schema"
)

// newDefaultRegistry creates a parser registry with all supported parsers.
func newDefaultRegistry() *parser.Registry {
	registry := parser.NewRegistry()
	registry.Register(postgresql.New())
	return registry
}

// parseSQLFile reads a file and parses it using a parser that matches the
// file extension. If no parser claims the extension, it falls back to trying
// all registered parsers.
func parseSQLFile(path string) (*schema.Model, error) {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading input file: %w", err)
	}

	registry := newDefaultRegistry()
	extension := strings.ToLower(filepath.Ext(path))

	// Try extension-based dispatch first.
	for _, name := range registry.Names() {
		parserInstance, lookupError := registry.Get(name)
		if lookupError != nil {
			continue
		}
		for _, supportedExtension := range parserInstance.SupportedExtensions() {
			if extension == supportedExtension {
				model, parseError := parserInstance.Parse(sqlBytes)
				if parseError == nil {
					return model, nil
				}
				return nil, fmt.Errorf("parsing with %s: %w", name, parseError)
			}
		}
	}

	// Fallback: try each parser silently (legacy behaviour).
	for _, name := range registry.Names() {
		parserInstance, lookupError := registry.Get(name)
		if lookupError != nil {
			continue
		}
		model, parseError := parserInstance.Parse(sqlBytes)
		if parseError == nil {
			return model, nil
		}
	}

	return nil, fmt.Errorf("no parser could process %q (supported: %s)",
		path, strings.Join(registry.Names(), ", "))
}
