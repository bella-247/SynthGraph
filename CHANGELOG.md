# Changelog

## v1.0.0 (2026-07-12)

### Features

- **CLI:** `synthgraph generate` with row count, format (SQL/CSV), seed, verbose, schema-name, and YAML config flags
- **CLI:** `synthgraph inspect` for schema analysis with structure, enums, and semantic details
- **CLI:** per-table progress reporting during generation
- **Web UI:** full browser interface at localhost:8080 with schema editor, interactive graph diagram, step-by-step generation flow, and job history with one-click downloads
- **Web UI:** in-app docs, URL hash routing, keyboard shortcuts, favicon
- **Parser:** PostgreSQL DDL support via real libpg_query C parser (tables, columns, types, PK, FK, UNIQUE, NOT NULL, DEFAULT expressions, composite keys, `CREATE TYPE ... AS ENUM`)
- **Parser:** structured `ParseError` type with line:column position for actionable error messages
- **Generator:** topological sort of FK dependencies; cycle-safe for circular references (e.g., self-referencing `manager_id`)
- **Generator:** semantic value generation — column names drive realistic data (email → email, phone → phone number, city → city name)
- **Generator:** deterministic seeding — same schema + same seed always produces identical output
- **Generator:** schema inference for audit columns (`created_at`, `updated_at`), role columns (`is_admin`, `status`)
- **Validator:** post-generation validation against NOT NULL, PK, UNIQUE, enum, length, and FK constraints
- **Validator:** catches edge cases before output reaches your database

### Infrastructure

- **CI:** automatic build, vet, race-detected tests, and cross-platform compile check
- **Release:** GitHub Actions builds Linux (amd64 + arm64), macOS (amd64 + arm64), Windows (amd64) binaries on every tag push
- **Release:** auto-generated checksums, downloadable from GitHub Releases
- **Docker:** containerized web server for deployment
- **Install scripts:** one-liner `curl | sh` (Linux/macOS) and `irm | iex` (Windows)

### Documentation

- README with TOC, quick install, 2-minute walkthrough, how-it-works pipeline diagram, platform install table
- `docs/ARCHITECTURE.md` — full pipeline breakdown, design decisions, file layout
- `docs/cli_reference.md` — complete CLI reference with examples
- `docs/DEVELOPMENT.md` — build, test, and dev workflow
- `docs/CONTRIBUTING.md` — PR workflow, commit conventions, architectural rules
- `docs/DESIGN.md` — design philosophy, trade-offs, token system
- `docs/graph_model.md` — dependency graph internals
- `docs/constraint_system.md` — constraint tracking and enforcement
- `docs/Future-Plan.md` — roadmap and upcoming features

### Design

- Logo evolution from SG monogram to graph-node icon
- Six-wordmark design for CLI header output
- CSS variables for colors, typography, spacing
- Warm empty states, dark mode, accessibility (`prefers-reduced-motion`, `skip-to-content`, focus indicators, contrast)
- Custom scrollbar styling, tab-based navigation, searchable template picker

### Bug Fixes

- Web UI CSP relaxed for cytoscape CDN scripts
- Frontend JS favicon 404 resolved
- Generator: silent NOT NULL placeholder replaced with proper `GenError`
- Generator: unique-retry exhaustion reported as error instead of silently continuing
- Validator: all constraint types (NOT NULL, PK, UNIQUE, enum, length, FK) checked
- Input size limits enforced in config validation
- Context propagation and SSE cleanup in server
- Multiple font rendering and syntax highlighting fixes

### Refactoring

- Pluggable semantic generator registry
- Split bloated files into modular structure
- Centralized build commands via Makefile
- Removed redundant scripts and stale documentation
- gofmt standard formatting across all Go source files

## v0.1.0 (2026-07-08)

Initial preview release. Core schema parsing, FK-aware generation, basic CLI, and early web prototype.
