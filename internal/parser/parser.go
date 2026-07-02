// Package parser defines the SchemaParser interface that all dialect-specific
// parsers must implement.
//
// Every parser, regardless of format, must produce a *schema.Schema.
// Downstream stages (graph, planner, generator, validator, exporter)
// never know or care about the original schema format.
package parser

import "synthgraph/internal/schema"

// SchemaParser is the interface that all dialect-specific parsers implement.
//
// V1: The only implementation is the PostgreSQL parser (internal/parser/postgresql).
// V2+: MySQL (via vitess), SQLite, Prisma, Drizzle, etc.
type SchemaParser interface {
	// Parse reads a schema source and returns the unified internal model.
	// The parser handles all dialect-specific AST transformations internally.
	Parse(source []byte) (*schema.Schema, error)

	// Name returns the parser identifier (e.g., "postgresql", "mysql", "prisma").
	Name() string

	// SupportedExtensions returns file extensions this parser handles (e.g., [".sql"]).
	SupportedExtensions() []string
}
