package postgresql

import (
	"fmt"
	"strings"

	"synthgraph/internal/schema"
)

// PostgreSQLParser implements the parser.SchemaParser interface for PostgreSQL DDL.
//
// It works in two phases:
//
// Phase 1 — Parse SQL into our intermediate AST (Stmt types).
//
//	With CGO: uses pg_query_go to parse.
//	Without CGO: returns an error — install CGO + gcc.
//
// Phase 2 — Translate our AST into schema.Model.
//
//	Pure Go, always works.
type PostgreSQLParser struct{}

// New creates a new PostgreSQLParser.
func New() *PostgreSQLParser {
	return &PostgreSQLParser{}
}

// Name returns "postgresql".
func (p *PostgreSQLParser) Name() string {
	return "postgresql"
}

// SupportedExtensions returns [".sql"].
func (p *PostgreSQLParser) SupportedExtensions() []string {
	return []string{".sql"}
}

// Parse reads PostgreSQL DDL and returns the canonical schema.Model.
func (p *PostgreSQLParser) Parse(source []byte) (*schema.Model, error) {
	text := string(source)

	// Phase 1: parse SQL into our intermediate AST
	stmts, err := parseSQL(text)
	if err != nil {
		return nil, fmt.Errorf("postgresql parse: %w", err)
	}

	// Phase 2: translate to canonical schema model
	result, err := Translate(stmts)
	if err != nil {
		return nil, fmt.Errorf("postgresql translate: %w", err)
	}

	return result, nil
}

// Preprocess splits SQL text into individual statements and removes comments.
// This runs before the CGO parser to handle statement boundaries.
func preprocessSQL(text string) []string {
	var results []string
	current := strings.Builder{}

	lines := strings.Split(text, "\n")
	inBlockComment := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" && current.Len() == 0 {
			continue
		}

		// Handle block comments spanning lines
		if inBlockComment {
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				trimmed = strings.TrimSpace(trimmed[idx+2:])
				inBlockComment = false
			} else {
				continue
			}
		}

		// Remove inline comments
		if idx := strings.Index(trimmed, "--"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}

		// Handle block comment starts
		if idx := strings.Index(trimmed, "/*"); idx >= 0 {
			endIdx := strings.Index(trimmed[idx+2:], "*/")
			if endIdx >= 0 {
				prefix := strings.TrimRight(trimmed[:idx], " \t")
				suffix := strings.TrimSpace(trimmed[idx+2+endIdx+2:])
				trimmed = strings.TrimSpace(prefix + suffix)
			} else {
				trimmed = strings.TrimSpace(trimmed[:idx])
				inBlockComment = true
			}
		}

		if trimmed == "" {
			continue
		}

		current.WriteString(trimmed)
		current.WriteString(" ")

		// Check if this statement ends with a semicolon
		if strings.HasSuffix(strings.TrimSpace(trimmed), ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				results = append(results, stmt)
			}
			current.Reset()
		}
	}

	// Flush any remaining statement
	remaining := strings.TrimSpace(current.String())
	if remaining != "" && !strings.HasSuffix(remaining, ";") {
		remaining += ";"
	}
	if remaining != "" && remaining != ";" {
		results = append(results, remaining)
	}

	return results
}

// parseSQL parses PostgreSQL SQL text and returns our intermediate AST statements.
// The actual parsing is in adapter_cgo.go (with CGO) or adapter_nocgo.go (stub).
var parseSQL func(text string) ([]Stmt, error)
