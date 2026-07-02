# SynthGraph — Architecture

**Version:** 1.0.0  
**Language:** Go  

---

## Directory Structure

```
synthgraph/
├── cmd/
│   └── synthgraph/
│       └── main.go                  # Entry point — wires CLI to core
│
├── internal/
│   ├── schema/
│   │   └── model.go                 # Unified internal representation (parser-agnostic)
│   │
│   ├── parser/
│   │   ├── parser.go                # SchemaParser interface definition
│   │   ├── postgres/
│   │   │   ├── parser.go            # PostgreSQL parser wrapping pg_query_go
│   │   │   ├── translator.go        # Translate PostgreSQL AST to schema.Model
│   │   │   └── translator_test.go
│   │   └── registry.go              # Auto-detect parser by file extension
│   │
│   ├── graph/
│   │   ├── graph.go                 # Node, Edge, SchemaGraph types
│   │   ├── builder.go               # BuildGraph() — schema model → graph
│   │   ├── topo.go                  # TopologicalSort() — Kahn's algorithm
│   │   ├── cycles.go                # FindCycles() — Tarjan's SCC
│   │   └── graph_test.go
│   │
│   ├── planner/
│   │   ├── planner.go               # Plan, TablePlan, DeferredFK types
│   │   ├── builder.go               # BuildPlan() — graph → generation plan
│   │   └── planner_test.go
│   │
│   ├── generator/
│   │   ├── generator.go             # GeneratorFunc type, GenerationContext
│   │   ├── registry.go              # GeneratorRegistry — name + type maps
│   │   ├── engine.go                # Generate() — executes plan, produces Dataset
│   │   ├── dataset.go               # Dataset, TableData types
│   │   ├── generators_identity.go   # id, uuid generators
│   │   ├── generators_person.go     # email, name, phone generators
│   │   ├── generators_location.go   # address, city, country, lat/lng
│   │   ├── generators_web.go        # url, ip, token, api_key
│   │   ├── generators_finance.go    # price, amount, currency, percentage
│   │   ├── generators_text.go       # description, title, bio
│   │   ├── generators_time.go       # created_at, updated_at, deleted_at
│   │   ├── generators_boolean.go    # is_*, has_*, can_*
│   │   ├── generators_types.go      # type-based fallback generators
│   │   └── engine_test.go
│   │
│   ├── validator/
│   │   ├── validator.go             # Validate() — full constraint check
│   │   ├── rules.go                 # Individual rule implementations
│   │   └── validator_test.go
│   │
│   ├── exporter/
│   │   ├── exporter.go              # Exporter interface
│   │   ├── sql/
│   │   │   ├── sql_exporter.go      # SQL INSERT exporter
│   │   │   └── sql_exporter_test.go
│   │   └── csv/
│   │       ├── csv_exporter.go      # CSV exporter
│   │       └── csv_exporter_test.go
│   │
│   └── cli/
│       ├── root.go                  # Root cobra command
│       ├── generate.go              # `generate` subcommand
│       ├── inspect.go               # `inspect` subcommand
│       └── version.go               # `version` subcommand
│
├── testdata/
│   ├── schemas/
│   │   ├── simple.sql
│   │   ├── chain.sql
│   │   ├── cycle_nullable.sql
│   │   ├── cycle_unresolvable.sql
│   │   ├── ecommerce.sql
│   │   └── hr.sql
│   └── golden/
│       ├── simple_100.sql
│       ├── chain_100.sql
│       ├── ecommerce_100.sql
│       └── hr_100.sql
│
├── docs/
│   ├── architecture.md              # This file
│   ├── graph_model.md               # Graph theory details
│   ├── constraint_system.md         # Constraint handling details
│   └── cli_reference.md             # Full CLI reference
│
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                   # Test + lint on every PR
│   │   └── release.yml              # Build binaries on tag push
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md
│   │   └── feature_request.md
│   └── PULL_REQUEST_TEMPLATE.md
│
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── CONTRIBUTING.md
├── ROADMAP.md
└── LICENSE                          # MIT
```

---

## Pipeline Data Flow

```
CLI flags
    │
    ▼
[Parser Registry]
    │  detects format by file extension (.sql → PostgreSQL)
    │
    ▼
[PostgreSQL Parser]                  internal/parser/postgres/parser.go
    │  calls pg_query_go.Parse()
    │  gets PostgreSQL AST
    │
    ▼
[AST Translator]                     internal/parser/postgres/translator.go
    │  walks PostgreSQL AST
    │  maps to schema.Model (tables, columns, constraints)
    │  returns *schema.Model
    │
    ▼
[Graph Builder]                      internal/graph/builder.go
    │  tables → nodes
    │  foreign keys → edges
    │  returns *SchemaGraph
    │
    ▼
[Topological Sort]                   internal/graph/topo.go
    │  Kahn's algorithm
    │  returns ordered tables + cycle members
    │
    ▼
[Cycle Detector]                     internal/graph/cycles.go
    │  Tarjan's SCC on cycle members
    │  identifies nullable breakpoints
    │
    ▼
[Planner]                            internal/planner/
    │  builds TablePlan list
    │  records DeferredFKs
    │  returns *Plan
    │
    ▼
[Generator Engine]                   internal/generator/engine.go
    │  iterates TablePlans
    │  resolves GeneratorFunc per column
    │  enforces PK/unique via value pools
    │  selects FK values from generated PK pools
    │  returns *Dataset
    │
    ▼
[Validator]                          internal/validator/
    │  checks all constraints
    │  returns []ValidationError (empty = pass)
    │
    ▼
[Exporter]                           internal/exporter/
    │  SQL or CSV
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
type GeneratorFunc func(col *schema.Column, ctx *GenerationContext) (any, error)
```

---

## Dependency Rules

These rules are enforced. Violations are architectural bugs.

| Package | May import | Must NOT import |
|---|---|---|
| `schema` | stdlib only | anything internal |
| `parser/*` | `schema`, stdlib | `graph`, `generator`, `validator`, `exporter` |
| `parser/postgres` | `schema`, stdlib, `pg_query_go` | nothing (parser-specific layer) |
| `graph` | `schema`, stdlib | `parser`, `generator`, `validator`, `exporter` |
| `planner` | `schema`, `graph`, stdlib | `parser`, `generator`, `validator`, `exporter` |
| `generator` | `schema`, `planner`, stdlib | `parser`, `graph`, `validator`, `exporter` |
| `validator` | `schema`, `generator`, stdlib | `parser`, `graph`, `planner`, `exporter` |
| `exporter/*` | `schema`, `generator`, stdlib | `parser`, `graph`, `planner`, `validator` |
| `cli` | all of the above | nothing outside `internal/` |

---

## Error Types

```go
// Each stage defines its own error type for clear categorization

type ParseError struct {
    Line    int
    Column  int
    Message string
}

type GraphError struct {
    Kind    GraphErrorKind  // CycleUnresolvable, SelfReferenceNotNullable
    Tables  []string
    Message string
    Hint    string
}

type GenerationError struct {
    Table   string
    Column  string
    Rule    string
    Retries int
    Message string
}

type ValidationError struct {
    Table      string
    Column     string
    RowIndex   int
    Value      any
    Rule       string
    Message    string
    IsInternal bool
}
```

---

## Makefile Targets

```makefile
build:
    go build -ldflags="-X main.version=$(VERSION)" -o bin/synthgraph ./cmd/synthgraph

test:
    go test ./... -race -count=1

test-golden:
    go test ./... -run TestGolden -update=false

lint:
    golangci-lint run ./...

bench:
    go test ./... -bench=. -benchmem

clean:
    rm -rf bin/

release:
    GOOS=linux   GOARCH=amd64 go build -o dist/synthgraph-linux-amd64   ./cmd/synthgraph
    GOOS=darwin  GOARCH=amd64 go build -o dist/synthgraph-darwin-amd64  ./cmd/synthgraph
    GOOS=darwin  GOARCH=arm64 go build -o dist/synthgraph-darwin-arm64  ./cmd/synthgraph
    GOOS=windows GOARCH=amd64 go build -o dist/synthgraph-windows-amd64.exe ./cmd/synthgraph
```

---

## CI Pipeline (GitHub Actions)

**On every PR:**
1. `go vet ./...`
2. `golangci-lint run`
3. `go test ./... -race`
4. Build binary (sanity check)

**On tag push (`v*`):**
1. Run full test suite
2. Build release binaries for all 4 platforms
3. Create GitHub Release with binaries attached
