# SynthGraph — Architecture

> **Who this is for:** Developers and contributors who want to understand the internal structure — how packages relate, where to add new features, and what rules to follow.
>
> **If you're new here:** Start with the [README](../README.md) or the [Development Guide](DEVELOPMENT.md). This document is a deep dive into the codebase structure and pipeline design.

**Version:** 2.0.0  
**Language:** Go  

---

## Directory Structure

```
synthgraph/
├── cmd/
│   ├── synthgraph/
│   │   ├── main.go                    # Entry point — wires CLI to core
│   │   ├── generate.go                # `synthgraph generate` subcommand
│   │   ├── inspect.go                 # `synthgraph inspect` subcommand
│   │   ├── parse.go                   # `synthgraph parse` subcommand
│   │   ├── config.go                  # YAML config file support
│   │   ├── config_test.go
│   │   ├── log.go                     # Structured logger
│   │   └── version.go                 # `--version` support
│   │
│   ├── synthgraph-web/
│   │   ├── main.go                    # Web API server entry point
│   │   └── server/
│   │       ├── handlers_frontend.go   # Frontend page handlers
│   │       ├── handlers_generate.go   # /api/generate endpoint
│   │       ├── handlers_graph.go      # /api/graph endpoint
│   │       ├── handlers_stream.go     # SSE streaming
│   │       ├── helpers.go             # Shared response helpers
│   │       ├── job_store.go           # In-memory + disk job persistence
│   │       ├── middleware.go          # CORS, recovery, body limit, timeout
│   │       ├── ratelimit.go           # Per-connection rate limiting
│   │       ├── server.go              # HTTP router + middleware setup
│   │       ├── server_test.go
│   │       └── types.go               # Shared request/response types
│
├── internal/
│   ├── schema/
│   │   ├── model.go                   # Model, Table, Column, ForeignKey, EnumType
│   │   ├── validate.go                # Pre-generation model validation
│   │   └── validate_test.go
│   │
│   ├── parser/
│   │   ├── parser.go                  # SchemaParser interface definition
│   │   ├── registry.go                # Auto-detect parser by file extension
│   │   └── postgresql/
│   │       ├── parser.go              # PostgreSQL parser wrapping pg_query_go
│   │       ├── adapter_cgo.go         # CGO-enabled pg_query wrapper
│   │       ├── adapter_nocgo.go       # CGO-disabled: returns clear error
│   │       ├── ast.go                 # PostgreSQL AST types
│   │       ├── types.go               # Type mapping utilities
│   │       ├── translate.go           # Pipeline: extract → normalize → link → validate → build
│   │       ├── normalize.go           # Stage 2: type canonicalization
│   │       ├── link.go                # Stage 3: FK and enum cross-reference resolution
│   │       ├── validate.go            # Stage 4: schema consistency validation
│   │       ├── dedupe.go              # Dedup logic for build stage
│   │       ├── extract.go             # Stage 1: raw table state from AST
│   │       ├── translate_test.go
│   │       └── types_test.go
│   │
│   ├── graph/
│   │   ├── graph.go                   # Graph, Node, Edge types
│   │   ├── builder.go                 # Build() — schema model → graph
│   │   ├── cycles.go                  # Tarjan's SCC (iterative, explicit stack)
│   │   ├── cardinality.go             # Edge cardinality inference
│   │   ├── node.go / nodes.go         # Node helpers
│   │   ├── edge.go / edges.go         # Edge helpers
│   │   ├── validate.go                # Graph consistency validation
│   │   └── *_test.go                  # Test files
│   │
│   ├── semantic/
│   │   ├── doc.go                     # Package documentation — "the brain of SynthGraph"
│   │   ├── builder.go                 # Build() — graph → SemanticGraph
│   │   ├── semantic_graph.go          # SemanticGraph, SemanticNode types
│   │   ├── models.go                  # Inference models (TemporalInfo, RoleInfo, etc.)
│   │   ├── column.go                  # Column-level semantic resolution
│   │   ├── rule.go                    # Rule interface and inference engine
│   │   ├── infer_roles.go             # Column role inference rules
│   │   ├── infer_relationships.go     # Relationship kind inference rules
│   │   ├── infer_temporal.go          # Temporal column pattern inference
│   │   ├── infer_audit.go             # Audit column pattern inference
│   │   └── *_test.go                  # Test files
│   │
│   ├── planner/
│   │   ├── planner.go                 # Plan, TablePlan, DeferredFK types
│   │   ├── builder.go                 # BuildPlan() — graph → generation plan
│   │   ├── cycles.go                  # Cycle resolution strategy
│   │   ├── topsort.go                 # Topological ordering for generation
│   │   ├── blocked.go                 # Blocked dependency detection
│   │   └── *_test.go                  # Test files
│   │
│   ├── generator/
│   │   ├── generator.go               # GenerationContext, GenError, Dataset types
│   │   ├── generate.go                # Generate() — executes plan, produces Dataset
│   │   ├── generate_row.go            # Row-level generation loop
│   │   ├── context.go                 # Pre-computation for correlated values
│   │   ├── registry.go                # GeneratorRegistry — semantic + type maps
│   │   ├── rng.go                     # Deterministic per-table RNG (FNV-64a + PCG)
│   │   ├── rng_test.go
│   │   ├── fk.go                      # FK column map + PK extraction
│   │   ├── tracker.go                 # UNIQUE constraint tracking
│   │   ├── backfill.go                # Deferred FK backfill for cycle resolution
│   │   ├── datasets.go                # Word lists (names, cities, etc.)
│   │   ├── types.go / types_test.go   # Type-based fallback generators
│   │   ├── scalars.go                 # int, serial, float generators
│   │   ├── name.go                    # first_name, last_name, full_name generators
│   │   ├── contact.go                 # email, phone, address generators
│   │   ├── temporal.go                # timestamp, date generators
│   │   ├── numeric.go                 # price, amount, currency generators
│   │   ├── misc.go                    # Code, slug generators
│   │   ├── web.go                     # URL, IP generators
│   │   ├── content.go                 # title, description generators
│   │   └── *_test.go                  # Test files
│   │
│   ├── validator/
│   │   ├── validator.go               # Validate() — full post-generation constraint check
│   │   └── validator_test.go          # Tests covering all constraint rules
│   │
│   └── exporter/
│       ├── exporter.go                # ExportConfig, ExportSQL, ExportCSV
│       ├── sql.go                     # SQL INSERT implementation
│       ├── csv.go                     # CSV implementation
│       └── exporter_test.go
│
├── testdata/
│   └── schemas/
│       ├── users.sql                  # Minimal single-table (PK, NOT NULL, UNIQUE)
│       ├── edge_cases.sql             # Composite PKs, cycles, self-ref FKs
│       └── ecommerce.sql              # Multi-table e-commerce with FKs and enums
│
├── docs/
│   ├── ARCHITECTURE.md                # This file
│   ├── DEVELOPMENT.md                 # Build, test, run guide
│   ├── CONTRIBUTING.md                # How to contribute
│   ├── cli_reference.md               # Full CLI reference
│   ├── graph_model.md                 # Graph theory details
│   ├── constraint_system.md           # Constraint handling details
│   ├── DESIGN.md                      # Web UI design tokens
│   └── Future-Plan.md                 # Upcoming features
│
├── scripts/
│   ├── install.sh                     # Linux/macOS install script
│   └── install.ps1                    # Windows install script
│
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                     # Test + vet + build on every PR
│   │   └── release.yml                # Test + cross-compile + Docker on tag push
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md
│   │   └── feature_request.md
│   ├── PULL_REQUEST_TEMPLATE.md
│   └── copilot-instructions.md
│
├── go.mod
├── go.sum
├── Makefile
├── dev.ps1                            # Windows development automation
├── Dockerfile
├── README.md
└── LICENSE                            # MIT
```

---

## Pipeline Data Flow

Every stage consumes one typed artifact and produces another. Stages after the parser never see dialect-specific types — only `schema.Model`.

```
CLI flags
    │
    ▼
[Parser Registry]                    internal/parser/registry.go
    │  detects format by file extension (.sql → PostgreSQL)
    │
    ▼
[PostgreSQL Parser]                  internal/parser/postgresql/parser.go
    │  calls pg_query_go.Parse()
    │  returns PostgreSQL AST
    │
    ▼   ── PostgreSQL AST (pg_query_go types) ──
    │
    ▼
[Translator Internal Pipeline]       internal/parser/postgresql/translate.go
    │
    │  1. Extract → raw table state from AST (newTranslator)
    │  2. Normalize → canonical types, resolve SERIAL, set nullability (normalize.go)
    │  3. Link → resolve FK targets, connect enums (link.go)
    │  4. Validate → check consistency, detect duplicates (validate.go)
    │  5. Build → deduplicate, mark PK cols, produce final output (build in translate.go)
    │
    │  returns *schema.Model
    │
    ▼   ── schema.Model (parser-agnostic) ──
    │
    ▼
[Graph Builder]                      internal/graph/builder.go
    │  tables → nodes, foreign keys → edges
    │  returns *SchemaGraph
    │
    ▼   ── SchemaGraph ──
    │
    ▼
[Semantic Analysis]                  internal/semantic/builder.go
    │  infers column roles, relationship kinds, temporal patterns
    │  from node/edge structure + column naming conventions
    │  returns *SemanticGraph
    │
    ▼   ── SemanticGraph ──
    │
    ▼
[Topological Sort + Cycle Detection] internal/planner/topsort.go, internal/graph/cycles.go
    │  Kahn's algorithm + Tarjan's SCC
    │  returns ordered tables + cycle members + nullable breakpoints
    │
    ▼   ── ordered graph ──
    │
    ▼
[Planner]                            internal/planner/
    │  builds TablePlan list, records DeferredFKs
    │  returns *GenerationPlan
    │
    ▼   ── GenerationPlan ──
    │
    ▼
[Generator Engine]                   internal/generator/generate.go
    │  iterates TablePlans
    │  resolves GeneratorFunc per column (seeded per-table RNG)
    │  enforces PK/unique via value pools
    │  selects FK values from generated PK pools
    │  cross-column correlation (city→state→zip, created_at < updated_at)
    │  returns *Dataset
    │
    ▼   ── Dataset ──
    │
    ▼
[Post-generation Validator]          internal/validator/validator.go
    │  checks all constraints (FK, PK, unique, enum, NOT NULL, length)
    │  returns []ValidationError (empty = pass)
    │
    ▼   ── ValidatedDataset ──
    │
    ▼
[Exporter]                           internal/exporter/
    │  SQL INSERT or CSV
    │  writes to io.Writer
    │
    ▼
Output file / stdout
```

---

## Key Interfaces

### SchemaParser

```go
// internal/parser/parser.go
type SchemaParser interface {
    // Parse reads the schema source and returns the unified internal model.
    // All dialect-specific logic (AST transformation) is internal to the parser.
    // The rest of SynthGraph only ever sees schema.Model.
    Parse(source []byte) (*schema.Model, error)

    Name() string // e.g., "postgresql"
    SupportedExtensions() []string // e.g., [".sql"]
}

// V1: PostgreSQL Parser wraps pg_query_go
// V2+: MySQL, Prisma, etc. each implement SchemaParser with their own translators
// All feed the same schema.Model. Graph, planner, generator are parser-agnostic.
```

### Semantic Rule

```go
// internal/semantic/rule.go
type Rule interface {
    Name() string
    Apply(nodeID string, node *graph.Node, g *graph.Graph) []Inference
}
```

### Exporter

```go
// internal/exporter/exporter.go
type Exporter interface {
    Export(dataset *generator.Dataset, model *schema.Model, out io.Writer) error
    Name() string
    FileExtension() string
}
```

### GeneratorFunc

```go
// internal/generator/generator.go
type TypeGenerator interface {
    Generate(col *schema.Column, rowIndex int, rng *rand.Rand) (any, error)
}
```

---

## Dependency Rules

These rules are enforced. Violations are architectural bugs.

| Package | May import | Must NOT import |
|---|---|---|
| `schema` | stdlib only | anything internal |
| `parser/*` | `schema`, stdlib | `graph`, `semantic`, `generator`, `validator`, `exporter` |
| `parser/postgresql` | `schema`, stdlib, `pg_query_go` | nothing (parser-specific layer) |
| `graph` | `schema`, stdlib | `parser`, `semantic`, `generator`, `validator`, `exporter` |
| `semantic` | `schema`, `graph`, stdlib | `parser`, `generator`, `validator`, `exporter` |
| `planner` | `schema`, `graph`, stdlib | `parser`, `semantic`, `generator`, `validator`, `exporter` |
| `generator` | `schema`, `graph`, `semantic`, `planner`, stdlib | `parser`, `validator`, `exporter` |
| `validator` | `schema`, `generator`, stdlib | `parser`, `graph`, `semantic`, `planner`, `exporter` |
| `exporter` | `schema`, `generator`, stdlib | `parser`, `graph`, `semantic`, `planner`, `validator` |
| `cmd/synthgraph` | all `internal/` packages | nothing outside the module |
| `cmd/synthgraph-web` | all `internal/` packages | nothing outside the module |

---

## Stage Purity

Every stage MUST be a pure function of its input artifact.

```
output = F(input)
```

No globals.
No shared state.
No file writes.
No caches.
No database connections.
No randomness from system sources.

**Why pure?**
- **Testable** — a pure stage needs no mocking, no setup, no teardown.
- **Deterministic** — same input always produces the same output.
- **Cacheable** — outputs can be cached and reused when inputs haven't changed.
- **Parallelizable** — independent stages can run concurrently with zero coordination.

The only exception is the Parser stage, which reads a file from disk. After that, the pipeline is pure.

All randomness is channeled through the seeded per-table RNG derived from `TableSeed = FNV-64a("globalSeed:tableName")`. No stage calls `rand.Int()` or `time.Now()` directly.

---

## Artifact Invariants

Each pipeline artifact has a well-defined invariant. A stage may assume its input satisfies the invariant and must guarantee its output satisfies its own.

| Artifact | Invariant | Produced By |
|---|---|---|
| `AST` | Represents exactly what the source parser produced. No transformations applied. | Parser |
| `schema.Model` | Types are canonical. Enum references resolved. FK targets verified to exist. No duplicate tables or columns. Internally consistent. | Translator (`build()`) |
| `SchemaGraph` | Every table has exactly one node. Every FK has exactly one directed edge. Self-referencing FKs are self-loop edges. Edge nullability recorded. No duplicate edges. | Graph Builder |
| `SemanticGraph` | Every table node has inferred roles, temporal patterns, and relationship kinds. All inferences are additive — the original graph is preserved. | Semantic Analysis |
| `GenerationPlan` | Generation order is finalized. Cycles are identified. Each cycle has a resolution strategy (nullable breakpoint or error). Deferred FKs are enumerated. | Planner |
| `Dataset` | Row count per table matches plan. PK values are unique per table. FK values reference existing rows. Unique constraints hold. Serial columns are sequential. Deferred FK columns are NULL. | Generator |
| `ValidatedDataset` | All constraints verified: PK uniqueness, FK referential integrity, unique constraints, enum values, NOT NULL, length limits. Safe to export. | Post-generation Validator |

---

## Error Types

```go
// Each stage defines its own error type for clear categorization

// schema.ValidationError — found in schema/validate.go
type ValidationError struct {
    Table   string
    Message string
}

// graph.ValidationError — found in graph/validate.go
type GraphValidationError struct {
    Message string
}

// planner errors — found in planner/cycles.go, planner/blocked.go
type CycleError struct {
    Tables  []string
    Message string
}

// generator.GenError — found in generator/generator.go
type GenError struct {
    Table   string
    Row     int
    Column  string
    Message string
}

// validator.ValidationError — found in validator/validator.go
type PostGenValidationError struct {
    Table    string
    Column   string
    RowIndex int
    Value    any
    Rule     string   // "NOT_NULL", "PK_UNIQUE", "UNIQUE", "ENUM", "LENGTH", "FK"
    Message  string
}
```

---

## Makefile Targets

```makefile
build-cli:
    CGO_ENABLED=1 go build -o bin/synthgraph ./cmd/synthgraph/

build-web:
    CGO_ENABLED=1 go build -o bin/synthgraph-web ./cmd/synthgraph-web/

build-all: build-cli build-web   # also: build (alias)

run-web:
    CGO_ENABLED=1 go run ./cmd/synthgraph-web/ --port $(PORT)

run-cli:
    CGO_ENABLED=1 go run ./cmd/synthgraph/ generate --input $(SCHEMA) --rows $(ROWS)

test:
    CGO_ENABLED=1 go test ./... -count=1

test-quick:
    go test ./internal/... ./cmd/synthgraph/... ./cmd/synthgraph-web/server/... -count=1

test-server:
    go test ./cmd/synthgraph-web/server/... -v -count=1

lint:
    go vet ./...

clean:
    rm -rf bin/ coverage.out

ci: lint build-all test
```

Note: cross-compilation requires `CGO_ENABLED=1` (or `$env:CGO_ENABLED='1'` on PowerShell) for the PostgreSQL parser (depends on `pg_query_go` via CGO). Ensure a C cross-compiler is available for each target platform, or see `docs/DEVELOPMENT.md` for alternative build setups.

---

## CI Pipeline (GitHub Actions)

**On every PR/push to main or dev:**
1. `go vet ./...`
2. `go test ./... -race` (full suite, CGO enabled)
3. Build CLI + web binaries (sanity check)
4. Test server package (no CGO needed)

**On tag push (`v*`):**
1. Run full test suite with race detector
2. Build Linux release binaries (amd64 + arm64) for CLI and web app
3. Build and push Docker image to GHCR
4. Create GitHub Release with binaries attached
