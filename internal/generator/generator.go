// Package generator consumes a GenerationPlan and produces a Dataset —
// a complete set of synthetically generated rows that satisfy all FK constraints,
// with circular dependencies resolved via nullable deferred backfill.
//
// The generator is the "execution engine" of the pipeline. It consumes the
// planner's deterministic ordering and produces real data.
//
// # Architecture position
//
//	Parser → schema.Model → Graph → planner.BuildPlan → GenerationPlan
//	                                                          │
//	                                                          ▼
//	                                                    generator.Generate
//	                                                          │
//	                                                          ▼
//	                                                    Dataset
//	                                                          │
//	                                                          ▼
//	                                                    validator.Validate
//
// # RNG determinism
//
// Every table uses a deterministic seed derived from the global seed and the
// table name: TableSeed = FNV-64a("globalSeed:tableName"). This ensures that
// re-running the generator with the same seed always produces identical data.
// There are no calls to rand.Int(), time.Now(), or crypto/rand in the hot path.
//
// # Generation phases
//
//  1. Generate each table in planner order. For acyclic dependencies, the
//     planner guarantees that referenced tables are generated before the
//     tables that reference them, so FK values can be sampled from an
//     already-populated PK pool.
//  2. Deferred FK columns (part of a cycle) are inserted as NULL.
//  3. After all tables are generated, each DeferredFK is backfilled: every
//     NULL value is replaced with a random PK from the referenced table.
package generator

import (
	"context"
	"fmt"
	"math/rand/v2"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// ── Output types ──────────────────────────────────────────────────────────

// GeneratedTable holds all generated rows for a single table.
type GeneratedTable struct {
	// TableName is the fully-qualified table name.
	TableName string

	// Rows is the list of generated rows in generation order (0..RowCount-1).
	Rows []GeneratedRow
}

// GeneratedRow is a single generated row. Values are keyed by column name.
type GeneratedRow map[string]any

// ── Configuration ─────────────────────────────────────────────────────────

// ProgressCallback is an optional hook that the generator calls after each table
// is fully generated. tableName is the table just completed, current is the
// 1-based index of that table, and total is the total number of tables in the plan.
// The callback MUST be safe to call concurrently (the generator calls it
// synchronously from a single goroutine).
type ProgressCallback func(tableName string, current int, total int)

// GenerationContext carries all configuration and reference data needed by the
// generator. It is constructed once and shared across all table generators.
type GenerationContext struct {
	// Context controls cancellation. When cancelled, the generator stops after
	// the current table and returns partial results. If nil, no cancellation
	// is performed (backward compatible default).
	Context context.Context

	// Progress is an optional callback invoked after each table is generated.
	// It is called synchronously from the generator's single goroutine.
	Progress ProgressCallback

	// GlobalSeed is the master seed for the entire generation run.
	// Every table derives its own deterministic seed from this value.
	GlobalSeed uint64

	// Model is the canonical schema model with all table/column metadata.
	Model *schema.Model

	// Graph is the structural graph, used for FK metadata lookups.
	Graph *graph.Graph

	// SemanticGraph carries inferred roles, relationships, and patterns.
	SemanticGraph *semantic.SemanticGraph

	// Registry is an optional pluggable generator registry. When set, it
	// overrides the default package-level registry, allowing callers to
	// extend or replace semantic type → generator mappings without
	// modifying global state. When nil, the defaultRegistry is used.
	Registry *Registry
}

// PartialError records a generation failure for a single table when the
// generator is configured to continue on error.
type PartialError struct {
	// Table is the table that failed to generate.
	Table string

	// Err is the underlying error.
	Err error
}

// Dataset is the complete output of the generator. It contains all generated
// rows for every table in the schema.
type Dataset struct {
	Tables []*GeneratedTable
	Errors []PartialError
}

// GenError describes a generation failure.
type GenError struct {
	// Table is the table that was being generated when the error occurred.
	Table string

	// Row is the 0-based row index that caused the error, or -1 if the error
	// is not row-specific.
	Row int

	// Column is the column that caused the error, or empty if not column-specific.
	Column string

	// Message describes the exact failure.
	Message string
}

// Error implements the error interface.
func (genError *GenError) Error() string {
	where := fmt.Sprintf("table %q", genError.Table)
	if genError.Row >= 0 {
		where = fmt.Sprintf("%s row %d", where, genError.Row)
	}
	if genError.Column != "" {
		where = fmt.Sprintf("%s column %q", where, genError.Column)
	}
	return fmt.Sprintf("generator: %s: %s", where, genError.Message)
}

// ── Type generator interface ──────────────────────────────────────────────

// TypeGenerator produces a single synthetic value for a column of a specific type.
type TypeGenerator interface {
	// Generate returns a single value for the given column. The rowIndex (0-based)
	// and per-table RNG are provided for deterministic generation.
	Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error)
}

// TypeGeneratorFunc is a convenience adapter that turns a function into a TypeGenerator.
type TypeGeneratorFunc func(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error)

// Generate implements TypeGenerator.
func (function TypeGeneratorFunc) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return function(column, rowIndex, rng)
}
