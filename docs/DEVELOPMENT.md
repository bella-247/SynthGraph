# SynthGraph — Development Guide

Everything you need to build, test, run, and extend SynthGraph during development.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Project Layout](#project-layout)
- [Building](#building)
- [Running Tests](#running-tests)
- [Running the CLI](#running-the-cli)
- [Running the Web Application](#running-the-web-application)
- [Running the Lightweight Graph Visualizer](#running-the-lightweight-graph-visualizer)
- [Adding a New Parser Dialect](#adding-a-new-parser-dialect)
- [Adding a New Inference Rule](#adding-a-new-inference-rule)
- [Adding a New Type Generator](#adding-a-new-type-generator)
- [Code Style & Conventions](#code-style--conventions)
- [Debugging Tips](#debugging-tips)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| **Go** | 1.21+ | Language runtime and compiler |
| **GCC / MinGW-w64** | any | Required by CGO for `pg_query_go` (PostgreSQL parser bindings) |

### Installing GCC

**Windows:** Install [MinGW-w64](https://www.mingw-w64.org/) via [MSYS2](https://www.msys2.org/):

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
# Add C:\msys64\ucrt64\bin to your PATH
```

**macOS:**
```bash
xcode-select --install
```

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get install gcc libpq-dev
```

### Verify setup

```bash
go version       # → go 1.21+
gcc --version    # → gcc (x86_64-posix) ...
```

---

## Project Layout

```
synthgraph/
 ├── cmd/
 │   ├── synthgraph/
 │   │   ├── main.go           # CLI entry point — subcommand dispatch
 │   │   ├── config.go         # YAML config file types and loader
 │   │   ├── generate.go       # "synthgraph generate" — full generation pipeline
 │   │   ├── inspect.go        # "synthgraph inspect" — schema analysis
 │   │   └── version.go        # "synthgraph version" — version info
 │   ├── synthgraph-web/
 │   │   ├── main.go             # Entry point: embed, flags, signal handling, server start
 │   │   ├── index.html          # Embedded SPA (Cytoscape.js graph viz, 4-page pipeline UI)
 │   │   └── server/
 │   │       ├── server.go       # Server struct, constructor, route registration, graceful shutdown
 │   │       ├── types.go        # Request/response types (parseRequest, graphResponse, etc.)
 │   │       ├── middleware.go   # Logging, panic recovery, CORS, body size limit
 │   │       ├── job_store.go    # In-memory job store with optional file persistence
 │   │       ├── helpers.go      # writeJSON, writeError helpers
 │   │       ├── handlers_frontend.go  # Frontend + health + parse handlers
 │   │       ├── handlers_graph.go     # Graph + semantic handlers
 │   │       ├── handlers_generate.go  # Generation pipeline + job listing handlers
 │   │       └── server_test.go        # 14 unit tests for handlers, middleware, job store
 │   └── serveviz/
 │       ├── main.go            # Lightweight read-only graph visualizer
 │       └── index.html         # Embedded HTML (Cytoscape.js)
│
├── internal/
│   ├── schema/
│   │   └── model.go           # Canonical IR — Model, Table, Column, ForeignKey
│   │
│   ├── parser/
│   │   ├── parser.go          # SchemaParser interface
│   │   ├── registry.go        # Parser auto-discovery registry
│   │   └── postgresql/
│   │       ├── ast.go         # Postgres-specific AST types
│   │       ├── types.go       # Type canonicalization map
│   │       ├── adapter_cgo.go  # CGO adapter (pg_query_go wrapper)
│   │       ├── adapter_nocgo.go # Stub (no CGO — returns error)
│   │       ├── translate.go   # Pipeline orchestrator
│   │       ├── normalize.go   # Stage: type normalization
│   │       ├── link.go        # Stage: FK cross-reference resolution
│   │       ├── validate.go    # Stage: consistency checking
│   │       └── translate_test.go
│   │
│   ├── graph/
│   │   ├── graph.go           # Graph, Node, Edge types
│   │   ├── builder.go         # Build() — schema.Model → Graph
│   │   ├── topo.go            # TopologicalSort() — Kahn's algorithm
│   │   ├── cycles.go          # FindCycles() — Tarjan's SCC
│   │   ├── validate.go        # Graph validation helpers
│   │   └── graph_test.go
│   │
│   ├── semantic/
│   │   ├── semantic_graph.go  # SemanticGraph, SemanticNode, SemanticRelationship types
│   │   ├── models.go          # TableRole, Inference, TemporalPattern, AuditPattern
│   │   ├── builder.go         # Build() — Graph → SemanticGraph
│   │   ├── rule.go            # Rule interface + InferenceContext
│   │   ├── infer_roles.go     # EntityRule, JunctionRule, HierarchyRule, LookupRule, TransactionalRule
│   │   ├── infer_temporal.go  # TemporalRule
│   │   ├── infer_audit.go     # AuditRule
│   │   ├── infer_relationships.go  # Relationship inference logic
│   │   └── builder_test.go
│   │
 │   ├── planner/
 │   │   ├── planner.go         # GenerationPlan, TablePlan types
 │   │   ├── builder.go         # BuildPlan() — topological sort + cycle detection
 │   │   └── planner_test.go
 │   │
 │   ├── generator/
│   │   ├── dataset.go         # Dataset, GeneratedTable, GeneratedRow types
│   │   ├── engine.go          # Generate() — main generation orchestrator
│   │   ├── generate_row.go    # Per-table row generation loop
│   │   ├── backfill.go        # Cyclic FK deferred backfill
│   │   ├── rng.go             # Deterministic per-table RNG (FNV-64a + PCG)
│   │   ├── types.go           # Type generator interface + built-in generators
│   │   ├── temporal.go        # Timestamp generator
│   │   ├── tracker.go         # UNIQUE constraint tracking
│   │   ├── errors.go          # GenError type
│   │   └── generator_test.go
│   │
│   ├── validator/
│   │   ├── validator.go       # Validate() — constraint verification
│   │   ├── unique.go          # UNIQUE constraint checker
│   │   ├── foreign_key.go     # FK constraint checker
│   │   └── validator_test.go
│   │
│   └── exporter/
│       ├── exporter.go        # ExportConfig type
│       ├── sql.go             # ExportSQL() — INSERT statements
│       ├── csv.go             # ExportCSV() — RFC 4180 CSV
│       └── exporter_test.go
│
├── testdata/
│   └── schemas/
│       ├── users.sql          # Single-table schema
│       ├── ecommerce.sql      # Multi-table e-commerce schema
│       └── edge_cases.sql     # Composite PKs, cycles, enums
│
├── docs/
│   ├── DEVELOPMENT.md         # ← This file
│   ├── ARCHITECTURE.md        # Full architectural deep-dive
│   ├── cli_reference.md       # User-facing CLI reference
│   ├── graph_model.md         # Graph data model
│   ├── constraint_system.md   # Constraint system design
│   ├── CONTRIBUTING.md        # How to contribute
│   ├── ROADMAP.md             # Future plans
│   └── SRS.md                 # Software requirements spec
│
├── go.mod
├── go.sum
└── README.md
```

### Pipeline stages (in order)

```
SQL File → Parser (CGO) → AST → Normalize → Link → Validate → Build
                                                              │
                                                              ▼
                                                        schema.Model
                                                              │
                                                              ▼
                                                       Graph.Build()
                                                              │
                                                              ▼
                                                         Graph
                                                          / \
                                                         /   \
                                                        ▼     ▼
                                              semantic.Build  planner.BuildPlan
                                                        │     │
                                                        │     ▼
                                                        │  GenerationPlan
                                                        │     │
                                                        │     ▼
                                                         │  generator.Generate()
                                                         │     │
                                                         │     ▼
                                                         │  Dataset
                                                         │     │
                                                         │     ▼
                                                         │  exporter.ExportSQL/ExportCSV
```

---

## Building

### Full build (with CGO — includes PostgreSQL parser)

```bash
# Build the synthgraph CLI
rtk CGO_ENABLED=1 go build -o synthgraph.exe ./cmd/synthgraph/

# Build all packages (no binary output)
rtk CGO_ENABLED=1 go build ./...

# Build the web application
rtk CGO_ENABLED=1 go build -o synthgraph-web.exe ./cmd/synthgraph-web/

# Build the lightweight visualizer
rtk CGO_ENABLED=1 go build -o serveviz.exe ./cmd/serveviz/
```

> **On Windows with MinGW-w64**, if CGO complains about missing `gcc`, verify it's on your PATH:
> ```bash
> where gcc
> ```

### Build without CGO (parser tests are skipped)

```bash
rtk go build ./...
```
Only packages that don't depend on the PostgreSQL parser will compile. The parser package itself compiles (it uses a build-tag stub when CGO is off), but `go test` skips parser tests.

### Cross-compilation with CGO

CGO cross-compilation is complex. Build natively on the target platform, or use Docker:

```dockerfile
FROM golang:1.22-bookworm
RUN apt-get update && apt-get install -y gcc libpq-dev
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -o synthgraph ./cmd/synthgraph/
```

---

## Running Tests

### Run all tests

```bash
rtk CGO_ENABLED=1 go test ./... -v -count=1
```

**Flags explained:**
| Flag | Meaning |
|---|---|
| `-v` | Verbose — print each test name and result |
| `-count=1` | Disable result caching — always run fresh |
| `-run <pattern>` | Run only tests matching `<pattern>` (regex) |
| `-failfast` | Stop on first failure |
| `-cover` | Show code coverage |
| `-timeout 60s` | Fail tests that exceed 60 seconds |

### Run specific packages

```bash
# Parser only
rtk CGO_ENABLED=1 go test ./internal/parser/postgresql/ -v -count=1

# Semantic layer only
rtk go test ./internal/semantic/ -v -count=1

# Generator only
rtk go test ./internal/generator/ -v -count=1

# Graph only
rtk go test ./internal/graph/ -v -count=1

# Planner only
rtk go test ./internal/planner/ -v -count=1

# Exporters only
rtk go test ./internal/exporter/ -v -count=1

# CLI
rtk go test ./cmd/synthgraph/ -v -count=1
```

### Run a specific test

```bash
# Exact test name
rtk CGO_ENABLED=1 go test ./internal/parser/postgresql/ -run TestTranslate_SingleTable -v -count=1

# Pattern match
rtk CGO_ENABLED=1 go test ./internal/parser/postgresql/ -run "TestTranslate_*" -v -count=1

# Semantic tests for junction inference
rtk go test ./internal/semantic/ -run "Junction" -v -count=1

# Planner tests for cycle handling
rtk go test ./internal/planner/ -run "Cycle" -v -count=1
```

### Run without CGO (fast, for non-parser work)

```bash
# Types, utilities, CLI, graph, planner, generator, exporter, semantic
rtk go test ./internal/schema/ ./internal/graph/ ./internal/planner/ ./internal/generator/ ./internal/exporter/ ./internal/semantic/ -v -count=1
```

### Coverage report

```bash
# Run with coverage
rtk CGO_ENABLED=1 go test ./... -coverprofile=coverage.out -count=1

# View coverage in browser
rtk go tool cover -html=coverage.out

# View coverage per function (terminal)
rtk go tool cover -func=coverage.out
```

---

## Running the CLI

### `synthgraph generate`

Generates synthetic data from a SQL schema file.

```bash
# Required: --input (or -i)
rtk go run ./cmd/synthgraph/ generate --input testdata/schemas/ecommerce.sql

# Custom row count
rtk go run ./cmd/synthgraph/ generate --input testdata/schemas/ecommerce.sql --rows 50

# Shorthand flags
rtk go run ./cmd/synthgraph/ generate -i testdata/schemas/ecommerce.sql -r 100 -s 12345

# Output to file (SQL)
rtk go run ./cmd/synthgraph/ generate -i testdata/schemas/ecommerce.sql -o output.sql

# CSV output
rtk go run ./cmd/synthgraph/ generate -i testdata/schemas/ecommerce.sql -f csv -o output.csv

# With schema name (for INSERT INTO "schema"."table")
rtk go run ./cmd/synthgraph/ generate -i testdata/schemas/ecommerce.sql --schema-name public

# Using the compiled binary
rtk ./synthgraph.exe generate -i testdata/schemas/ecommerce.sql -r 100
```

**Flags:**

| Flag | Short | Required | Default | Description |
|---|---|---|---|---|
| `--input` | `-i` | Yes | — | Path to SQL schema file |
| `--output` | `-o` | No | stdout | Output file path |
| `--format` | `-f` | No | `sql` | `sql` or `csv` |
| `--rows` | `-r` | No | `10` | Rows per table |
| `--seed` | `-s` | No | `42` | Random seed for determinism |
| `--schema-name` | — | No | `""` | Schema name for SQL output |
| `--config` | `-c` | No | — | Path to YAML config file |
| `--init-config` | — | No | — | Write a default YAML config file and exit |

### `synthgraph inspect`

Analyzes a schema file and prints structured information.

```bash
# Basic overview (tables, columns, enums)
rtk go run ./cmd/synthgraph/ inspect --input testdata/schemas/ecommerce.sql

# Show graph structure (nodes, edges by kind)
rtk go run ./cmd/synthgraph/ inspect -i testdata/schemas/edge_cases.sql --graph

# Show semantic inference results (roles, temporal patterns, relationships)
rtk go run ./cmd/synthgraph/ inspect -i testdata/schemas/ecommerce.sql --semantic

# Verbose output = --graph + --semantic
rtk go run ./cmd/synthgraph/ inspect -i testdata/schemas/ecommerce.sql -v

# Shorthand flags
rtk go run ./cmd/synthgraph/ inspect -i testdata/schemas/ecommerce.sql -v
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--input` | `-i` | — | Path to SQL schema file (required) |
| `--graph` | — | `false` | Show graph structure summary |
| `--semantic` | — | `false` | Show semantic inference summary |
| `--verbose` | `-v` | `false` | Equivalent to `--graph --semantic` |

**Example output:**
```
Schema Overview
===============
Tables: 3
Enums:  1

Table: users
  Columns: 6
  Primary Key: id
    id integer PK
    email text NOT NULL
    ...

Graph Summary
=============
Nodes: 14 (tables=3, enums=1, columns=10)
Edges: 17
  contains: 10
  references: 2
  uses_enum: 1
  referenced_by: 2
  depends_on: 2

Semantic Summary
================
Node: table:users
  Roles: entity
Node: table:products
  Roles: entity, lookup
Node: table:orders
  Roles: transactional
  Temporal: created=true updated=false deleted=false
```

### `synthgraph version`

```bash
rtk go run ./cmd/synthgraph/ version
# → synthgraph version 0.1.0
```

---

## Running the Web Application

The `synthgraph-web` command starts an HTTP server with a full pipeline UI (REST API + embedded SPA). It reuses the same `internal/` packages as the CLI.

### Starting the server

```bash
# Start the web app (default port 8080, in-memory job storage)
rtk CGO_ENABLED=1 go run ./cmd/synthgraph-web/

# Custom port
rtk CGO_ENABLED=1 go run ./cmd/synthgraph-web/ --port 9090

# With job persistence (jobs survive restarts)
rtk CGO_ENABLED=1 go run ./cmd/synthgraph-web/ --job-file jobs.json

# Disable job persistence (in-memory only)
rtk CGO_ENABLED=1 go run ./cmd/synthgraph-web/ --job-file ""

# Using the compiled binary
rtk CGO_ENABLED=1 go build -o synthgraph-web.exe ./cmd/synthgraph-web/
rtk ./synthgraph-web.exe
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP server port |
| `--job-file` | `synthgraph-jobs.json` | Path to job persistence file (empty string = in-memory-only) |

Then open http://localhost:8080 in your browser.

### Pipeline UI (4 pages)

The SPA walks you through the full SynthGraph pipeline:

| Page | Endpoint | What it does |
|------|----------|-------------|
| **Schema** | `POST /api/parse` | Paste or upload SQL DDL, returns parsed model |
| **Graph** | `POST /api/graph` | Interactive Cytoscape.js graph (fcose layout), shows tables, FK edges, junction highlighting |
| **Semantic** | `POST /api/semantic` | Table role inference (entity, junction, lookup, transactional, hierarchical) |
| **Generate** | `POST /api/generate` | Configure rows/seed/format, run generation, download CSV or SQL |

### Job history

Completed generations are listed at `GET /api/jobs` (newest first). Each job stores its config, table count, output format, and any partial errors.

### REST API directly

You can call the API without the UI:

```bash
# Parse
curl -X POST http://localhost:8080/api/parse -H 'Content-Type: application/json' -d '{"sql":"CREATE TABLE users (id INT PRIMARY KEY);"}'

# Graph (requires model from parse step)
curl -X POST http://localhost:8080/api/graph -H 'Content-Type: application/json' -d '{"model":{...}}'

# Generate (accepts raw SQL + generation config)
curl -X POST http://localhost:8080/api/generate -H 'Content-Type: application/json' -d '{"input":"CREATE TABLE users (id INT PRIMARY KEY);","rows":10,"seed":42,"format":"csv"}'
```

### Supported output formats

- `csv` — RFC 4180 CSV (one file per table concatenated, with headers)
- `sql` — `INSERT INTO` statements wrapped in a transaction (`BEGIN/COMMIT`)

---

## Running the Lightweight Graph Visualizer

The older `serveviz` command provides a simpler read-only graph view (no generation UI):

```bash
# Start with a schema file
rtk CGO_ENABLED=1 go run ./cmd/serveviz/ --schema testdata/schemas/ecommerce.sql

# Custom port
rtk CGO_ENABLED=1 go run ./cmd/serveviz/ --schema testdata/schemas/ecommerce.sql --port 9090

# Using the compiled binary
rtk ./serveviz.exe --schema testdata/schemas/ecommerce.sql
```

Then open http://localhost:8080 in your browser.

**Features:**
- Interactive graph layout (fcose) — drag, zoom, pan
- Table cards with column details (PK/FK badges, types, nullable)
- Edge tooltips showing FK column mappings
- Detail panel on click — columns, FKs, reverse references
- Cardinality indicators (1—1, 1—*, *—*)
- Enum nodes shown as diamond shapes
- Neighborhood highlighting on hover

---

## Test Data Schemas

Located in `testdata/schemas/`:

| File | Purpose | Key Features |
|---|---|---|
| `users.sql` | Minimal single-table | 1 table, PK, NOT NULL, UNIQUE |
| `ecommerce.sql` | Multi-table e-commerce | 3 tables, FKs, enums, temporal columns |
| `edge_cases.sql` | Stress-test | Composite PKs, circular dependencies, self-referencing FKs, all types |

---

## Adding a New Parser Dialect

1. **Create a new package** under `internal/parser/<dialect>/`

2. **Implement the `SchemaParser` interface:**
   ```go
   type SchemaParser interface {
       Name() string
       Parse(input []byte) (*schema.Model, error)
       SupportedExtensions() []string
   }
   ```

3. **Register it in the CLI** (`cmd/synthgraph/generate.go` and `inspect.go`):
   ```go
   registry.Register(yourdialect.New())
   ```
   The registry iterates registered parsers and tries each one until one succeeds.

4. **Add test schemas** to `testdata/schemas/`.

No changes needed to the graph, planner, generator, or exporter — they consume only `schema.Model`.

---

## Adding a New Inference Rule

1. **Create a new file** `internal/semantic/infer_<rule>.go`

2. **Implement the `Rule` interface:**
   ```go
   type Rule interface {
       Name() string
       Apply(tableNode *SemanticNode, context *InferenceContext) []Inference
   }
   ```

3. **Register it in `builder.go`:**
   ```go
   func defaultRules() []Rule {
       return []Rule{
           &YourNewRule{},
           &JunctionRule{},
           // ...
       }
   }
   ```

4. **Use the `InferenceContext`** for all graph lookups — it has precomputed indexes:
   - `OutgoingForeignKeyCount` — how many FKs this table has to others
   - `IncomingForeignKeyCount` — how many tables reference this one
   - `ColumnCount` — total columns in the table
   - `ForeignKeyColumnIndex` — which columns are FK source columns
   - `SelfRefCount` — self-referencing FK count
   - `TemporalPattern` — detected created_at/updated_at/deleted_at columns
   - `AuditPattern` — detected created_by/updated_by/deleted_by columns

5. **Add table roles or semantic flags** to `models.go` if your rule introduces new ones.

---

## Adding a New Type Generator

1. **Create a generator struct** in `internal/generator/types.go`:

   ```go
   type macaddrGenerator struct{}
   
   func (g macaddrGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
       // return a random MAC address
       return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", ...), nil
   }
   ```

2. **Register it in `builtInGenerators`:**
   ```go
   var builtInGenerators = map[string]TypeGenerator{
       // ...
       "macaddr": macaddrGenerator{},
       "inet":    inetGenerator{},
   }
   ```

   The key must match the `column.Type` value produced by the parser's type normalization (stored in `internal/parser/postgresql/types.go`).

> **Important:** If the parser normalizes a type to a different name (e.g., `"double precision"` → `"double"`), the generator key must use the normalized name. If the generator key uses the raw name (e.g., `"double precision"`), the lookup will miss and fall through to `unknownTypeGenerator`.

---

## Code Style & Conventions

- **Descriptive naming** — full words, no cryptic abbreviations (`tableBuilder` not `tb`, `columnIndex` not `ci`)
- **Clear receiver names** — spelled out, e.g., `func (translator *SchemaTranslator)` not `func (st *SchemaTranslator)`
- **Single Responsibility** — one thing per method; extract private helpers for nested logic
- **Pure functions** — no globals, no shared state, no side effects, no `time.Now()` or `rand.Int()` (use seeded RNG)
- **Deterministic output** — same input + same seed = same output, every time
- **Error wrapping** — use `fmt.Errorf("context: %w", err)` to annotate errors
- **No `init()` functions** — use explicit wiring in the caller

---

## Debugging Tips

### See what the parser produces

```bash
rtk go run ./cmd/synthgraph/ inspect -i testdata/schemas/ecommerce.sql -v
```
Shows the full pipeline output: schema → graph → semantic inference.

### Print graph structure without semantic layer

```bash
rtk go run ./cmd/synthgraph/ inspect -i testdata/schemas/ecommerce.sql --graph
```

### Visualize the graph in a browser

```bash
rtk CGO_ENABLED=1 go run ./cmd/serveviz/ --schema testdata/schemas/ecommerce.sql
# Then open http://localhost:8080
```

### Run a quick generation to stdout

```bash
rtk go run ./cmd/synthgraph/ generate -i testdata/schemas/users.sql -r 5
```

### Trace test execution

```bash
# Show test file and line for each test
rtk CGO_ENABLED=1 go test ./internal/semantic/ -v -count=1

# Show compiler warnings/errors
rtk CGO_ENABLED=1 go build ./... 2>&1
```

---

## Troubleshooting

### `gcc: error: unrecognized command-line option ...`

You're on Windows with MinGW-w64 and there's a toolchain mismatch. Ensure you installed the **ucrt64** variant and `gcc` is on your PATH:

```bash
where gcc
# → C:\msys64\ucrt64\bin\gcc.exe
```

### `# github.com/pganalyze/pg_query_go/v5/pg_query`
### `fatal error: postgres.h: No such file or directory`

The CGO adapter needs the PostgreSQL headers. On Linux/macOS, install `libpq-dev` / `libpq`. On Windows, ensure MinGW-w64 is properly set up (the headers ship with the Go module).

### `go build` succeeds but `go test` fails on parser tests

Parser tests require CGO. Run without the parser package:

```bash
rtk go test ./internal/graph/ ./internal/planner/ ./internal/generator/ ./internal/validator/ ./internal/exporter/ ./internal/semantic/ -v -count=1
```

### `undefined: pgQuery` or adapter errors

The CGO adapter uses build tags. Ensure `CGO_ENABLED=1` is set. On platforms where CGO is unavailable, parser operations return an error — all other packages work fine.

### `No such layout 'fcose' found` in the visualizer

The web app and `serveviz` HTML pages load `cytoscape-fcose` from unpkg. If the CDN URL is wrong or the file isn't the browser bundle, switch the script tag to:

```html
<script src="https://cdn.jsdelivr.net/npm/cytoscape-fcose/dist/cytoscape-fcose.min.js"></script>
```

### `executable file not found in %PATH%` after `go build`

Go compiles to the current directory by default. Either use `go run` instead, or specify the output path:

```bash
rtk go build -o synthgraph.exe ./cmd/synthgraph/
rtk .\synthgraph.exe generate -i schema.sql
```
