# SynthGraph — Future Plan

> **Who this is for:** Anyone curious about what's coming next. Features are grouped by theme, not priority — nothing here is guaranteed or scheduled.

Features and enhancements deferred beyond the current release. Items are grouped by theme, not priority or timeline.

---

## Output & Export

- **JSON export** — structured JSON output per table
- **Parquet export** — columnar format for analytics workloads
- **PostgreSQL COPY format** — native bulk-load format
- **Direct database insertion** — `synthgraph generate --db` to write directly to a live database
- **ZIP archive download** — bundle multiple export files into one archive

## Parser Support

- **MySQL parser** — via the `SchemaParser` interface
- **SQLite parser** — via the `SchemaParser` interface
- **Prisma schema parser** — `.prisma` file support
- **Drizzle schema parser** — Drizzle ORM schema support
- **DBML parser** — database markup language support
- **Parser plugin system** — third-party parsers via the `Registry`

## Generation Engine

- **Streaming row writer** — channel-based row generation so the exporter writes rows as they're produced instead of materialising the full dataset in memory
- **Statistical distributions** — replace uniform random selection with configurable distributions (normal, exponential, etc.)
- **Business rule engine** — conditional cross-column logic evaluated after row generation
- **CHECK constraint enforcement** — currently parsed but not enforced during generation
- **Custom column generators** — regex patterns, custom value lists, user-defined generator functions
- **Configurable null probability** — make `aggFKNullProbability` tunable via `GenerationContext`
- **Configurable cache TTL** — currently hardcoded at 60 seconds
- **Data preview before generation** — show sample output before running the full generation

## Schema & Analysis

- **Schema diff tool** — `synthgraph diff` comparing two schema files, reporting structural changes
- **Enhanced `inspect`** — terminal graph rendering, dependency and cycle visualisation, cardinality recommendations
- **Semantic fingerprinting** — content-aware generation producing stable outputs across minor schema changes
- **Data anonymization** — transform production data while preserving constraints

## Developer Experience & Operations

- **Workspace configuration (`.synthgraph`)** — per-project config for row counts, seeds, and generation profiles
- **Artifact caching** — cache `schema.Model`, `SchemaGraph`, and `GenerationPlan`; invalidate on schema change
- **CI with coverage** — add CI pipeline with `go test -coverprofile`; document CGO build requirements
- **Graceful shutdown** — handle SIGINT with partial dataset output
- **Per-table error recovery** — continue generating remaining tables when one fails
- **Incremental generation** — regenerate only tables whose schema or row count changed since the last run
- **Bulk / scheduled generation** — queue and run multiple generation jobs

## Web Application

- **Multi-user / auth** — user accounts and session management
- **Dark/light theme toggle** — user-selectable colour scheme
- **PWA support** — offline-capable progressive web app
- **Generation profiles** — save and load named generation configurations
- **Data preview pagination** — paginated inline table preview for large datasets

## Infrastructure

- **Plugin system** — third-party generator plugins extending the generator registry at startup
- **REST API server mode** — `synthgraph serve` exposing the engine as an HTTP API for CI/CD
- **Cloud / SaaS (optional)** — managed hosting with expanded API
- **AI/LLM-powered generation** — use language models for more realistic data
