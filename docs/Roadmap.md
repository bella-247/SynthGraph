# SynthGraph — Implementation Roadmap

**Version:** 1.0.0  
**Language:** Go  
**Status:** Pre-Implementation  

---

## Table of Contents

1. [Milestone Overview](#1-milestone-overview)
2. [V1 — Core System](#2-v1--core-system)
3. [V2 — Expansion Layer](#3-v2--expansion-layer)
4. [V3 — Platform Layer](#4-v3--platform-layer)
5. [Weekly Execution Plan (Summer)](#5-weekly-execution-plan-summer)
6. [Definition of Done](#6-definition-of-done)

---

## 1. Milestone Overview

```
V1 (Summer Goal)         V2 (Fall→Winter)        V3 (Long-term)
─────────────────        ─────────────────        ─────────────────
PostgreSQL Parser        Additional Parsers       Web Application
Graph Engine             Statistical Dists        REST API Server
Planner                  Business Rules           Plugin System
Generator Engine         Schema Diff              Incremental Gen
Two Validators           Enhanced inspect         Semantic Fingerprint
SQL + CSV Export         JSON / COPY Export       Multi-dialect
generate + inspect CLI   Direct DB Insert         Cloud/SaaS (opt)
Code Audit (22 items)    Workspace Config
Production Readiness     Artifact Caching
(CI/CD, benchmarks,      Web Application
 graceful shutdown,
 error recovery,
 preflight validation,
 structured logging,
 config file)
```

V1 is the only goal this summer. V2 and V3 are defined so architectural decisions in V1 do not accidentally close doors.

---

## 2. V1 — Core System

### Phase 1 — Foundation (Week 1–2)

**Goal:** The skeleton works. Schema goes in, model comes out. Nothing generates yet.

#### 1.1 Project Scaffold

- Initialize Go module: `go mod init github.com/your-org/synthgraph`
- Set up directory structure (see Architecture section below)
- Configure linting: `golangci-lint` with standard ruleset
- Set up `Makefile` with targets: `build`, `test`, `lint`, `run`
- Write empty interface stubs for `SchemaParser`, `Exporter`, `GeneratorFunc`
- Commit: `chore: project scaffold and interfaces`

#### 1.2 Internal Schema Model

- Implement all types in `internal/schema/model.go`
  - `Model`, `Table`, `Column`, `DataType`
  - `PrimaryKey`, `ForeignKey`, `UniqueConstraint`, `CheckConstraint`
  - `FKAction`, `EnumType`
- Write unit tests for model construction (no parsing yet — build models by hand in tests)
- Commit: `feat(schema): internal schema model types`

#### 1.3 PostgreSQL Parser + Translator

- Integrate `github.com/pganalyze/pg_query_go` (wrapper around PostgreSQL's C parser)
  - `go get github.com/pganalyze/pg_query_go`
  - Verify CGO build works: `go build -v ./cmd/synthgraph` (requires libpq-dev or equivalent)
- Implement `PostgreSQLParser` satisfying the `SchemaParser` interface
  - Call `pg_query.Parse()` to get the PostgreSQL AST
  - Walk the AST and translate to `schema.Model` (this is the only dialect-specific code)
- The translator implements a five-stage internal pipeline:
  1. **Extract** — build initial raw state from AST (no normalization)
  2. **Normalize** — canonicalize types, resolve SERIAL, set nullability (`normalize.go`)
  3. **Link** — resolve FK target references, connect enums (`link.go`)
  4. **Validate** — schema consistency check: duplicate tables/columns, missing references (`validate.go`)
  5. **Build** — deduplicate constraints, mark PK columns, produce final `schema.Model`
- Translator must handle:
  - All CREATE TABLE statement variations
  - Column definitions with all PostgreSQL data types
  - All constraint types: PK, FK, UNIQUE, NOT NULL, DEFAULT, CHECK
  - PostgreSQL-specific: SERIAL, GENERATED, ARRAY, named ENUM types
  - Quoted identifiers, comments (automatically handled by pg_query_go)
- Parser wrapper stores the original SQL string for error reporting
- Return clear `ParseError` with line number and description on failure (from pg_query_go)
- Unit tests:
  - Simple table (no relationships)
  - Table with single FK and ON DELETE CASCADE
  - Table with composite PK
  - Table with composite unique constraint
  - Table with self-referencing FK
  - PostgreSQL ENUM type declaration
  - Multiple tables with cross-references
  - Quoted identifiers: `"user"`, `"Order"` (PostgreSQL-style)
  - Comments: `-- inline` and `/* block */`
  - Malformed SQL → expect error with line number
  - All V1 data types from SRS §4.1
- Commit: `feat(parser): PostgreSQL DDL parser via pg_query_go + translator`

**No hand-written lexer or recursive descent parser.** PostgreSQL's parser is production-ready; we translate its AST.

---

### Phase 2 — Graph Engine (Week 2–3)

**Parser Status:** By the end of Week 1, the PostgreSQL parser and translator are complete. Phases 2–7 have zero parser code to write — the schema.Model is guaranteed correct. All following stages (graph, planner, generator, validator, exporter, CLI) are parser-agnostic.

**Goal:** The schema is a graph. We know what depends on what.

#### 2.1 Graph Construction

- Implement `internal/graph/graph.go`: `Node`, `Edge`, `SchemaGraph`
- Implement `BuildGraph(model *schema.Model) (*SchemaGraph, error)`
  - Every table → one node
  - Every FK → one directed edge
  - Self-referencing FKs → self-loop edge
  - Populate `Nullable` on each edge based on source column nullability
- Implement reverse lookups: `Dependents()`, `Dependencies()`, `TransitiveDependencies()`
- Unit tests:
  - Linear chain: A → B → C
  - Diamond dependency: A → B, A → C, B → D, C → D
  - Self-loop: A → A
  - Two-node cycle: A → B → A
  - Three-node cycle: A → B → C → A
  - Isolated table (no edges)
- Commit: `feat(graph): schema graph construction and traversal`

#### 2.2 Topological Sort (Kahn's Algorithm)

- Implement `TopologicalSort(g *SchemaGraph) ([]string, []string, error)`
  - Returns: ordered table names, unresolved table names (cycle members), error
- Unit tests with all graph shapes above
- Commit: `feat(graph): topological sort via Kahn's algorithm`

#### 2.3 Cycle Detection (Tarjan's SCC)

- Implement `FindCycles(g *SchemaGraph) [][]*Node`
  - Returns list of SCCs (each SCC is a list of nodes in that cycle)
- For each SCC: identify the nullable edge(s) that can serve as breakpoints
- If no nullable edge exists: return unresolvable cycle error
- Unit tests:
  - No cycles → empty result
  - Two-node cycle with nullable edge → resolvable
  - Two-node cycle with no nullable edge → unresolvable error
  - Three-node cycle → correct SCC grouping
- Commit: `feat(graph): cycle detection via Tarjan's SCC`

#### 2.4 Generation Plan

- Implement `Planner` in `internal/planner/planner.go`
- `GenerationPlan` struct:
  ```go
  type GenerationPlan struct {
      Order       []TablePlan    // generation order
      DeferredFKs []DeferredFK   // FK updates to run after all inserts
  }

  type TablePlan struct {
      TableName   string
      Table       *schema.Table
      RowCount    int
      DeferredCols []string     // FK columns to insert as NULL initially
  }

  type DeferredFK struct {
      Table      string
      Column     string
      References string
      RefColumn  string
  }
  ```
- `BuildPlan(g *SchemaGraph, rowCount int) (*GenerationPlan, error)`
- Unit tests: verify correct order and deferred FK identification
- Commit: `feat(planner): generation plan builder`

---

### Phase 3 — Generator Engine (Week 3–4)

**Goal:** The plan produces data. Every value respects its constraint.

#### 3.1 Generator Registry and GenerationContext

- Implement `internal/generator/registry.go`
- `GeneratorRegistry` with name-based and type-based maps
- `Register(pattern string, fn GeneratorFunc)` for name patterns
- `RegisterType(dt schema.DataType, fn GeneratorFunc)` for type fallbacks
- `Resolve(col *schema.Column) GeneratorFunc` — name match first, type fallback second
- Implement `GenerationContext` with per-table RNG derivation:
  - `TableSeed = hash(GlobalSeed, TableName)` using FNV-64a
  - Each table gets its own deterministic RNG stream
  - Rationale: deterministic output independent of generation order, future parallel generation, future caching, isolated table generation
- Commit: `feat(generator): generator registry and generation context`

#### 3.2 Semantic Name-Based Generators

- Implement all generators from SRS §9.2
- Each generator in its own file grouped by category:
  - `generators_identity.go` — id, uuid
  - `generators_person.go` — email, first_name, last_name, full_name, phone
  - `generators_location.go` — address, city, country, zip, lat, lng
  - `generators_web.go` — url, ip, token, api_key
  - `generators_finance.go` — price, amount, currency, percentage
  - `generators_text.go` — description, title, bio
  - `generators_time.go` — created_at, updated_at, deleted_at
  - `generators_boolean.go` — is_*, has_*, can_*
- All generators use seeded RNG from `GenerationContext`
- Unit tests: every generator produces a value of expected format
- Commit: `feat(generator): semantic name-based generators`

#### 3.3 Type-Based Fallback Generators

- Implement all fallbacks from SRS §9.3
- Register them in the registry under their DataType keys
- Commit: `feat(generator): type-based fallback generators`

#### 3.4 Row Generation Engine

- Implement `internal/generator/engine.go`
- `Generate(plan *GenerationPlan, model *schema.Model, seed int64) (*Dataset, error)`
- For each table in plan order:
  - For each row (0 to RowCount-1):
    - For each column: resolve generator, call it, store value
    - Enforce PK uniqueness: retry if collision (max 100 retries)
    - Enforce unique constraints: retry if collision (max 100 retries)
    - Select FK values from pool of already-generated PK values
    - Set deferred FK columns to NULL
- `Dataset` struct:
  ```go
  type Dataset struct {
      Tables map[string]*TableData
  }

  type TableData struct {
      TableName string
      Columns   []string
      Rows      [][]any
  }
  ```
- Unit tests:
  - Single table, no constraints — generates correct row count
  - FK column values all exist in referenced table
  - Unique columns have no duplicates
  - Serial columns are sequential
  - Deferred FK columns are NULL in initial rows
- Commit: `feat(generator): row generation engine`

---

### Phase 4 — Post-generation Validator (Week 4–5)

**Note:** The Translator Validator (schema consistency check) is implemented as Stage 4 of the parser pipeline in Phase 1. This Phase 4 is the separate Post-generation Validator that checks the generated dataset.

**Goal:** No invalid dataset ever leaves the system.

#### 4.1 Constraint Validator

- Implement `internal/validator/validator.go`
- `Validate(dataset *Dataset, model *schema.Model) []ValidationError`
- Implement each check from SRS §10.1:
  - PK uniqueness check
  - FK referential integrity check
  - Unique constraint check
  - Enum value check
  - Not-null check
  - Length check
- `ValidationError` struct:
  ```go
  type ValidationError struct {
      Table      string
      Column     string
      RowIndex   int
      Value      any
      Rule       string
      Message    string
      IsInternal bool  // true = bug, false = schema issue
  }
  ```
- Unit tests — each rule tested with a deliberately broken dataset:
  - Duplicate PK → caught
  - Orphaned FK → caught
  - Duplicate unique value → caught
  - Invalid enum value → caught
  - NULL in NOT NULL column → caught
  - String exceeding VARCHAR length → caught
- Commit: `feat(validator): constraint validation engine`

---

### Phase 5 — Exporters (Week 5)

**Goal:** Valid data becomes usable output.

#### 5.1 Exporter Interface

- Define `internal/exporter/exporter.go` with `Exporter` interface (from SRS §11.1)

#### 5.2 SQL INSERT Exporter

- Implement `internal/exporter/sql.go`
- Output format per SRS §11.2:
  - Header comment block
  - `BEGIN;` ... `COMMIT;` transaction wrapper
  - Tables in generation plan order
  - Deferred FK `UPDATE` statements at end
  - Proper SQL escaping for all value types
  - NULL written as literal `NULL`
  - Strings single-quoted with escaped internal quotes
  - Booleans as `TRUE` / `FALSE`
  - Timestamps in `'YYYY-MM-DD HH:MM:SS'` format
- Unit tests: export known dataset → compare against golden `.sql` file
- Commit: `feat(exporter): SQL INSERT exporter`

#### 5.3 CSV Exporter

- Implement `internal/exporter/csv.go`
- One file per table, RFC 4180 compliant
- Header row with column names
- All values properly quoted and escaped
- Unit tests: export known dataset → compare against golden `.csv` files
- Commit: `feat(exporter): CSV exporter`

---

### Phase 6 — CLI (Week 5–6)

**Goal:** A developer can run a single command and get a seed file.

#### 6.1 CLI Framework

- Use `github.com/spf13/cobra` for CLI structure
- Root command: `synthgraph`
- Subcommands: `generate`, `inspect`, `version`
- Global flags: `--verbose` (debug-level logging to stderr)
- Commit: `feat(cli): cobra CLI scaffold`

#### 6.2 `generate` Command

- Wire full pipeline: parse → build graph → plan → generate → validate → export
- Handle all flags from SRS §13.1
- Print progress to stderr (table name being generated)
- On success: print summary to stderr, output to stdout or file
- On error: print structured error to stderr, exit with correct code
- Commit: `feat(cli): generate command`

#### 6.3 `inspect` Command

- Run parse → build graph → plan (no generation)
- Print full inspection report per SRS §13.1
- Commit: `feat(cli): inspect command`

#### 6.4 `version` Command

- Print `SynthGraph vX.Y.Z`
- Version injected at build time via `ldflags`
- Commit: `feat(cli): version command`

---

### Phase 7 — Polish & Release (Week 6–7)

**Goal:** The project is production-quality, not just functional.

#### 7.1 Golden Test Suite

- Build a suite of end-to-end golden tests:
  - `simple.sql` — 3 tables, no cycles
  - `chain.sql` — 6 tables, linear FK chain
  - `cycle_nullable.sql` — 2-table cycle with nullable FK
  - `cycle_unresolvable.sql` — expect error
  - `ecommerce.sql` — realistic 10-table e-commerce schema
  - `hr.sql` — self-referencing employees table
- Each test: run `generate`, compare output against golden file (ignoring timestamps)
- Commit: `test: golden test suite`

#### 7.2 Error Message Polish

- Audit every error path for clarity and actionability
- Ensure every error follows the format in SRS §12.2
- Add `--verbose` debug output for parser and graph stages
- Commit: `fix: error message audit and polish`

#### 7.3 Performance Benchmarks

- Benchmark: 100 rows × 50 tables
- Benchmark: 1000 rows × 20 tables
- Ensure both meet SRS §14.1 targets
- Commit: `bench: add performance benchmarks`

#### 7.4 README and Docs

- Write `README.md` (see OSS structure below)
- Write `docs/architecture.md`
- Write `docs/graph_model.md`
- Write `docs/cli_reference.md`
- Commit: `docs: complete documentation for v1.0.0`

#### 7.5 V1.0.0 Release

- Tag `v1.0.0`
- Build binaries for linux/amd64, darwin/amd64, darwin/arm64, windows/amd64
- GitHub Release with binaries attached
- Announce on relevant communities (r/golang, Hacker News, dev.to)

---

## 3. V2 — Expansion Layer

> These are defined for architectural awareness in V1. Do not implement in summer.  
> Placeholder — design details deferred until V1 ships.

### V2.1 — Additional Schema Parsers
Additional parsers implementing the `SchemaParser` interface: MySQL, SQLite, Prisma schema, Drizzle schema. Zero changes to the pipeline.

### V2.2 — Statistical Distribution Engine
Replace uniform random selection with configurable distributions (normal, exponential, etc.).

### V2.3 — Business Rule Engine
Conditional cross-column logic evaluated after row generation, before validation.

### V2.4 — Schema Diff Command
`synthgraph diff` comparing two schema files and reporting structural changes.

### V2.5 — Enhanced `inspect`
Terminal graph rendering, dependency and cycle visualization, cardinality recommendations.

### V2.6 — Additional Exporters
JSON and PostgreSQL COPY format exporters via the exporter registry.

### V2.7 — Direct Database Insertion
`synthgraph generate --db` to write generated data directly to a live database.

### V2.8 — Workspace Configuration (`.synthgraph`)
Per-project configuration file for row counts, seeds, and generation profiles (fast, balanced, realistic, stress).

### V2.9 — Artifact Caching
Cache `schema.Model`, `SchemaGraph`, and `GenerationPlan` artifacts. Cache invalidation on schema change.

---

## 4. V3 — Platform Layer

> Placeholder — no design work until V1 and V2 are stable.

### V3.1 — Web Application
Browser-based UI wrapping the same Go engine. Schema upload, graph visualization, dataset preview, SQL/CSV download.

### V3.2 — REST API Server Mode
`synthgraph serve` exposing the engine as an HTTP API for CI/CD and third-party tools.

### V3.3 — Plugin System
Third-party generator plugins extending the generator registry at startup.

### V3.4 — Incremental Generation
Regenerate only tables whose schema or row count changed since the last run.

### V3.5 — Semantic Fingerprinting
Content-aware generation that produces stable outputs across minor schema changes.

### V3.6 — Multi-dialect Ecosystem Expansion
Expand parser support to cover all major schema formats (MySQL, SQLite, Prisma, Drizzle, DBML).

---

## 5. Weekly Execution Plan (Summer)

This plan assumes a 7-week summer timeline with focused daily work sessions.

| Week | Focus | Deliverable |
|---|---|---|
| **Week 1** | Project scaffold, schema model, PostgreSQL parser + translator (extract → normalize → link → validate → build) | Parser tests all passing |
| **Week 2** | Graph construction, topological sort | Graph engine fully tested |
| **Week 3** | Cycle detection, planner, per-table RNG, generator registry | Plan builder working |
| **Week 4** | Semantic generators, row engine | Data generation working end-to-end |
| **Week 5** | Post-generation validator, translator validator, SQL exporter, CSV exporter | Full pipeline working |
| **Week 6** | CLI commands, golden tests, error polish | CLI fully functional |
| **Week 7** | Performance, docs, release | V1.0.0 tagged and released |

### Daily Development Discipline

- **Morning:** Write the tests first (TDD where possible)
- **Midday:** Implement to make tests pass
- **Evening:** Commit, push, update progress notes

Every week ends with a working, testable state. Never let a week end mid-pipeline-stage.

---

## 6. Definition of Done

### V1 is done when:

- [x] `synthgraph generate --schema ecommerce.sql --rows 100` produces a valid, runnable SQL seed file
- [x] `synthgraph inspect schema.sql` prints the correct graph analysis for any schema
- [x] All unit tests pass (`go test ./...`)
- [x] Code audit completed (22 items, all resolved)
- [x] CI/CD pipeline working (Go 1.26, CGO, Docker, cross-compile)
- [x] Benchmarks for core pipeline stages
- [x] Graceful shutdown on SIGINT
- [x] Per-table error recovery
- [x] Pre-flight schema validation
- [x] Structured logging
- [x] YAML config file support

### V3 (Web Application) is done when:

- [ ] `POST /api/parse` returns model JSON for uploaded SQL
- [ ] `POST /api/graph` returns nodes/edges for client-side rendering
- [ ] `POST /api/semantic` returns semantic graph
- [ ] `POST /api/generate` streams generation progress via SSE
- [ ] Frontend graph visualizer renders interactive DAG with Cytoscape.js
- [ ] Schema upload page accepts drag-and-drop SQL files
- [ ] Generation config form supports all CLI flags
- [ ] Data preview shows generated rows inline
- [ ] Job history tracks past runs with status and config
- [ ] Export downloads CSV/SQL from any completed job
- [ ] All endpoints covered by integration tests
- [ ] Docker image includes web frontend
