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
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strconv"
	"strings"

	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

// ── Output types ──────────────────────────────────────────────────────────

// Dataset is the complete output of the generator. It contains all generated
// rows for every table in the schema.
type Dataset struct {
	// Tables maps table name → generated rows, preserving generation order.
	Tables []*GeneratedTable
}

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

// GenerationContext carries all configuration and reference data needed by the
// generator. It is constructed once and shared across all table generators.
type GenerationContext struct {
	// GlobalSeed is the master seed for the entire generation run.
	// Every table derives its own deterministic seed from this value.
	GlobalSeed uint64

	// Model is the canonical schema model with all table/column metadata.
	Model *schema.Model

	// Graph is the structural graph, used for FK metadata lookups.
	Graph *graph.Graph

	// SemanticGraph carries inferred roles, relationships, and patterns.
	SemanticGraph *semantic.SemanticGraph
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

// ── Generate ──────────────────────────────────────────────────────────────

// Generate consumes a GenerationPlan and GenerationContext and produces a
// complete Dataset with all rows for every table.
//
// The generation follows the plan's order exactly. For acyclic tables,
// referenced tables are already populated when FK-dependent tables are
// generated. For cyclic tables, the breakpoint FK column is inserted as
// NULL and backfilled in the final phase.
func Generate(plan *planner.GenerationPlan, ctx *GenerationContext) (*Dataset, error) {
	// Pre-compute FK column → referenced table mapping for efficient lookups.
	fkMap := buildFKColumnMap(ctx.Graph)

	// Build enum name → values map for enum column generation.
	enumValues := buildEnumValues(ctx.Model)

	// Track generated PK values per table for FK resolution.
	// PKs maps table name → list of generated primary key values.
	tablePKs := make(map[string][]any)

	dataset := &Dataset{
		Tables: make([]*GeneratedTable, 0, len(plan.Order)),
	}

	// Phase 1: Generate each table in planner order.
	for _, tablePlan := range plan.Order {
		generatedTable, err := generateTable(tablePlan, ctx, fkMap, enumValues, tablePKs)
		if err != nil {
			return nil, err
		}
		dataset.Tables = append(dataset.Tables, generatedTable)

		// Collect PK values for FK resolution by downstream tables.
		pkValues := extractPKValues(generatedTable, tablePlan.Table)
		tablePKs[tablePlan.TableName] = pkValues
	}

	// Phase 2: Backfill deferred FK columns.
	if len(plan.DeferredFKs) > 0 {
		if err := backfillDeferredFKs(dataset, plan.DeferredFKs, tablePKs, ctx); err != nil {
			return nil, err
		}
	}

	return dataset, nil
}

// generateTable generates all rows for a single table from the plan.
func generateTable(
	tablePlan planner.TablePlan,
	ctx *GenerationContext,
	fkMap map[string][]FKRefInfo,
	enumValues map[string][]string,
	tablePKs map[string][]any,
) (*GeneratedTable, error) {
	tableName := tablePlan.TableName
	table := tablePlan.Table
	rowCount := tablePlan.RowCount
	deferredCols := makeStringSet(tablePlan.DeferredCols)

	rng := newTableRNG(ctx.GlobalSeed, tableName)

	generated := &GeneratedTable{
		TableName: tableName,
		Rows:      make([]GeneratedRow, 0, rowCount),
	}

	// Build column-level FK ref info for this table.
	tableFKCols := fkMap[tableName]

	// Track generated values for UNIQUE constraint enforcement.
	uniqueTracker := newUniqueTracker(table)

	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		row := make(GeneratedRow, len(table.Columns))

		for _, column := range table.Columns {
			// Deferred FK columns: insert as NULL.
			if deferredCols[column.Name] {
				row[column.Name] = nil
				continue
			}

			// FK columns: pick a PK value from the referenced table.
			if fkInfo, isFK := isFKColumn(tableFKCols, column.Name); isFK {
				pkValues, referencedAvailable := tablePKs[fkInfo.RefTable]
				if !referencedAvailable || len(pkValues) == 0 {
					return nil, &GenError{
						Table:   tableName,
						Row:     rowIndex,
						Column:  column.Name,
						Message: fmt.Sprintf("referenced table %q has no generated PK values", fkInfo.RefTable),
					}
				}
				value := pkValues[rng.IntN(len(pkValues))]
				row[column.Name] = value
				continue
			}

			// Regular column: generate from type generator.
			generator := typeGeneratorFor(column.Type, ctx.Model, enumValues)
			value, err := generator.Generate(&column, rowIndex, rng)
			if err != nil {
				return nil, &GenError{
					Table:   tableName,
					Row:     rowIndex,
					Column:  column.Name,
					Message: err.Error(),
				}
			}

			// Retry on UNIQUE violation.
			if uniqueTracker.isUniqueColumn(column.Name) {
				for attempts := 0; attempts < 10; attempts++ {
					if !uniqueTracker.checkSeen(column.Name, value) {
						break
					}
					value, err = generator.Generate(&column, rowIndex+attempts+1, rng)
					if err != nil {
						return nil, &GenError{
							Table:   tableName,
							Row:     rowIndex,
							Column:  column.Name,
							Message: err.Error(),
						}
					}
				}
				uniqueTracker.record(column.Name, value)
			}

			row[column.Name] = value
		}

		generated.Rows = append(generated.Rows, row)
	}

	return generated, nil
}

// ── FK resolution ─────────────────────────────────────────────────────────

// FKRefInfo describes a single FK column and its referenced table+column.
type FKRefInfo struct {
	Column     string
	RefTable   string
	RefColumns []string
}

// buildFKColumnMap builds a map from table name → list of FK ref info.
// Uses the graph's EdgeKindReferences edges for metadata.
func buildFKColumnMap(schemaGraph *graph.Graph) map[string][]FKRefInfo {
	fkMap := make(map[string][]FKRefInfo)

	for _, edge := range schemaGraph.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}

		// Resolve table name from node ID.
		fromNode, exists := schemaGraph.Nodes[edge.From]
		if !exists {
			continue
		}
		tableData, ok := fromNode.Data.(graph.TableData)
		if !ok {
			continue
		}

		toNode, exists := schemaGraph.Nodes[edge.To]
		if !exists {
			continue
		}
		toData, ok := toNode.Data.(graph.TableData)
		if !ok {
			continue
		}

		fkMeta, ok := edge.Metadata.(*graph.FKMetadata)
		if !ok {
			continue
		}

		for _, col := range fkMeta.LocalColumns {
			fkMap[tableData.Name] = append(fkMap[tableData.Name], FKRefInfo{
				Column:     col,
				RefTable:   toData.Name,
				RefColumns: fkMeta.ForeignColumns,
			})
		}
	}

	return fkMap
}

// isFKColumn checks if the given column name is an FK column and returns its ref info.
func isFKColumn(fkCols []FKRefInfo, columnName string) (FKRefInfo, bool) {
	for _, info := range fkCols {
		if info.Column == columnName {
			return info, true
		}
	}
	return FKRefInfo{}, false
}

// ── PK extraction ─────────────────────────────────────────────────────────

// extractPKValues extracts the primary key values from a generated table.
// Supports single-column PKs and composite PKs (returned as string keys).
func extractPKValues(generatedTable *GeneratedTable, table *schema.Table) []any {
	if len(table.PrimaryKey) == 0 {
		return nil
	}

	if len(table.PrimaryKey) == 1 {
		pkCol := table.PrimaryKey[0]
		values := make([]any, len(generatedTable.Rows))
		for i, row := range generatedTable.Rows {
			values[i] = row[pkCol]
		}
		return values
	}

	// Composite PK: return string-keyed map values.
	values := make([]any, len(generatedTable.Rows))
	for i, row := range generatedTable.Rows {
		var parts []string
		for _, pkCol := range table.PrimaryKey {
			parts = append(parts, fmt.Sprintf("%v", row[pkCol]))
		}
		values[i] = strings.Join(parts, "::")
	}
	return values
}

// ── Unique constraint tracking ────────────────────────────────────────────

// uniqueTracker ensures generated values for UNIQUE columns don't repeat.
type uniqueTracker struct {
	uniqueCols map[string]bool
	seen       map[string]map[any]bool
}

func newUniqueTracker(table *schema.Table) *uniqueTracker {
	tracker := &uniqueTracker{
		uniqueCols: make(map[string]bool),
		seen:       make(map[string]map[any]bool),
	}

	for _, col := range table.Columns {
		if col.Name == "" {
			continue
		}
		// Columns in PrimaryKey are UNIQUE by definition.
		if isPrimaryKeyColumn(col.Name, table.PrimaryKey) {
			tracker.uniqueCols[col.Name] = true
		}
	}

	// Also check UNIQUE constraints from the table definition.
	for _, uniqueConstraint := range table.Unique {
		for _, col := range uniqueConstraint {
			tracker.uniqueCols[col] = true
		}
	}

	return tracker
}

func (tracker *uniqueTracker) isUniqueColumn(name string) bool {
	return tracker.uniqueCols[name]
}

func (tracker *uniqueTracker) checkSeen(column string, value any) bool {
	if tracker.seen[column] == nil {
		return false
	}
	return tracker.seen[column][value]
}

func (tracker *uniqueTracker) record(column string, value any) {
	if tracker.seen[column] == nil {
		tracker.seen[column] = make(map[any]bool)
	}
	tracker.seen[column][value] = true
}

func isPrimaryKeyColumn(columnName string, pk []string) bool {
	for _, pkCol := range pk {
		if pkCol == columnName {
			return true
		}
	}
	return false
}

// ── Deferred FK backfill ──────────────────────────────────────────────────

// backfillDeferredFKs replaces NULL values in deferred FK columns with random
// PK values from the referenced tables.
func backfillDeferredFKs(
	dataset *Dataset,
	deferredFKs []planner.DeferredFK,
	tablePKs map[string][]any,
	ctx *GenerationContext,
) error {
	for _, dfk := range deferredFKs {
		refPKs, ok := tablePKs[dfk.References]
		if !ok || len(refPKs) == 0 {
			return &GenError{
				Table:   dfk.Table,
				Column:  dfk.Column,
				Message: fmt.Sprintf("referenced table %q has no generated PK values for backfill", dfk.References),
			}
		}

		tableRNG := newTableRNG(ctx.GlobalSeed, "backfill:"+dfk.Table+":"+dfk.Column)

		table := findGeneratedTable(dataset, dfk.Table)
		if table == nil {
			return &GenError{
				Table:   dfk.Table,
				Message: fmt.Sprintf("table %q not found in generated dataset", dfk.Table),
			}
		}

		for rowIndex := range table.Rows {
			if table.Rows[rowIndex][dfk.Column] == nil {
				table.Rows[rowIndex][dfk.Column] = refPKs[tableRNG.IntN(len(refPKs))]
			}
		}
	}

	return nil
}

// findGeneratedTable finds a GeneratedTable by name in the dataset.
func findGeneratedTable(dataset *Dataset, tableName string) *GeneratedTable {
	for _, table := range dataset.Tables {
		if table.TableName == tableName {
			return table
		}
	}
	return nil
}

// ── RNG / Seeding ─────────────────────────────────────────────────────────

// newTableRNG creates a deterministic RNG for a specific table.
// Seed = FNV-64a("globalSeed:tableName").
func newTableRNG(globalSeed uint64, tableName string) *rand.Rand {
	hash := fnv.New64a()
	hash.Write([]byte(strconv.FormatUint(globalSeed, 10)))
	hash.Write([]byte(":"))
	hash.Write([]byte(tableName))
	seed := hash.Sum64()

	// Split the 64-bit seed into two 32-bit values for PCG,
	// mixed with golden ratio constants for good bit distribution.
	high := seed ^ 0x9e3779b97f4a7c15
	low := seed ^ 0xbf58476d1ce4e5b9
	return rand.New(rand.NewPCG(high, low))
}

// ── Enum values ───────────────────────────────────────────────────────────

// buildEnumValues builds a map from enum type name to its list of values.
func buildEnumValues(model *schema.Model) map[string][]string {
	values := make(map[string][]string, len(model.Enums))
	for _, enumType := range model.Enums {
		vals := make([]string, len(enumType.Values))
		copy(vals, enumType.Values)
		values[enumType.Name] = vals
	}
	return values
}

// ── Utility ───────────────────────────────────────────────────────────────

// makeStringSet converts a string slice to a set.
func makeStringSet(values []string) map[string]bool {
	s := make(map[string]bool, len(values))
	for _, v := range values {
		s[v] = true
	}
	return s
}

// ── Column UUID generation with binary encoding ────────────────────────────
// UUID generation uses the per-table RNG, ensuring determinism.

// generateUUID produces a random UUID v4 string using the given RNG.
func generateUUID(rng *rand.Rand) string {
	var buffer [16]byte
	for index := range buffer {
		buffer[index] = byte(rng.Uint32() & 0xFF)
	}

	// Set version 4 (random) — RFC 4122, section 4.4.
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	// Set variant (10xx) — RFC 4122.
	buffer[8] = (buffer[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buffer[0:4],
		buffer[4:6],
		buffer[6:8],
		buffer[8:10],
		buffer[10:16],
	)
}

// Uvarint generation for enum selection uses the RNG directly.
var _ = binary.PutUvarint // silence unused import warning (used in tests via binary encoding)
