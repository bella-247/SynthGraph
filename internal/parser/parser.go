// Package parser defines the SchemaParser interface that all dialect-specific
// parsers must implement.
//
// Every parser, regardless of format, must produce a *schema.Model.
// Downstream stages (graph, planner, generator, validator, exporter)
// never know or care about the original schema format.
package parser

import (
	"fmt"

	"synthgraph/internal/schema"
)

// ParseError is a structured error returned when parsing a schema fails.
// It carries positional information (line:col) when available, plus an
// optional wrapped Err for error unwrapping (errors.Is / errors.As).
type ParseError struct {
	Line    int
	Col     int
	Message string
	Err     error // optional wrapped error, e.g. from pg_query
}

func (e *ParseError) Error() string {
	if e.Line > 0 && e.Col > 0 {
		return fmt.Sprintf("line %d:%d: %s", e.Line, e.Col, e.Message)
	}
	return e.Message
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// SchemaParser is the interface that all dialect-specific parsers implement.
//
// V1: The only implementation is the PostgreSQL parser (internal/parser/postgresql).
// V2+: MySQL (via vitess), SQLite, Prisma, Drizzle, etc.
type SchemaParser interface {
	// Parse reads a schema source and returns the unified internal model.
	// The parser handles all dialect-specific AST transformations internally.
	Parse(source []byte) (*schema.Model, error)

	// Name returns the parser identifier (e.g., "postgresql", "mysql", "prisma").
	Name() string

	// SupportedExtensions returns file extensions this parser handles (e.g., [".sql"]).
	SupportedExtensions() []string
}
