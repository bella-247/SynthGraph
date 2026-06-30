# Contributing to SynthGraph

Thank you for your interest in contributing. SynthGraph is built to be a long-term, widely adopted developer tool — and that only happens through a strong, thoughtful community.

This document explains how to contribute effectively.

---

## Before You Start

Read the [Architecture document](docs/architecture.md) and the [SRS](SRS.md) before making any significant contribution. SynthGraph has strict pipeline boundaries and architectural rules. Understanding them upfront will save you from rework.

---

## Development Setup

**Requirements:**
- Go 1.21 or higher
- `golangci-lint` (for linting)
- `make` (for build targets)
- PostgreSQL development headers (required for pg_query_go compilation)

**Setup:**

```bash
git clone https://github.com/your-org/synthgraph.git
cd synthgraph

# Install PostgreSQL dev headers (required for pg_query_go compilation)
# macOS: brew install libpq
# Ubuntu/Debian: sudo apt-get install libpq-dev
# Windows: (included with PostgreSQL installer, or use WSL)

go mod download
make build
make test
```

**Run the CLI locally:**

```bash
go run ./cmd/synthgraph generate --schema testdata/schemas/simple.sql --rows 50
```

**Run all tests:**

```bash
make test
```

**Run linter:**

```bash
make lint
```

---

## Types of Contributions

### Easy — Good First Issues

These require no deep architectural knowledge.

- **Add a new semantic field generator**
  Add a new name pattern in one of the `generators_*.go` files and register it in the registry. Write a unit test for it.

- **Add a new test schema**
  Add a realistic `.sql` schema to `testdata/schemas/` and a corresponding golden file to `testdata/golden/`. Good schemas: SaaS product, healthcare, logistics, inventory.

- **Improve an error message**
  Find an error that isn't clear or actionable and make it better. Follow the format in SRS §12.2.

- **Expand translator test coverage**
  Test an edge case: PostgreSQL quoted identifiers, comments, complex types, etc.
  The pg_query_go parser handles it; add a test case to the translator to verify mapping.

### Medium — Requires Understanding the Pipeline

- **Add a new exporter**
  Implement the `Exporter` interface in `internal/exporter/`. Write unit tests and a golden test.

- **Improve `inspect` output**
  Make the inspection report more useful — better formatting, additional statistics, clearer cycle descriptions.

- **Optimize graph performance**
  Profile the graph engine and improve performance for large schemas (50+ tables).

### Hard — Requires Deep Architecture Knowledge

- **Add a new schema parser (MySQL, SQLite, Prisma)**
  Implement the `SchemaParser` interface by wrapping an existing parser library (e.g., `vitess` for MySQL) and writing a translator from that AST to `schema.Model`. See `internal/parser/postgres/translator.go` as the pattern. The graph engine and rest of SynthGraph require zero changes — they're parser-agnostic.

- **Implement CHECK constraint enforcement**
  This requires an expression evaluator inside the generator. Discuss in an issue before starting.

---

## Contribution Rules

### Code Style

- Run `golangci-lint run ./...` before submitting. Zero warnings expected.
- Follow standard Go conventions (`gofmt`, `goimports`).
- No `interface{}` or `any` in pipeline types except where explicitly justified (generator output).
- Every exported function must have a doc comment.

### Architectural Rules

These are non-negotiable. PRs that violate them will not be merged.

- **No cross-stage imports.** The dependency rules in `docs/architecture.md` are enforced. Parser does not import generator. Generator does not import validator. Etc.
- **No silent failures.** Every error path must return an error. No logging-and-continuing.
- **No global state.** All state flows through function arguments and return values.
- **Determinism is sacred.** No `time.Now()`, `rand.Int()` without seeding, or any other non-deterministic call in the generation pipeline.

### Testing Rules

Every PR must include tests. No exceptions.

- New generators: unit test asserting the value format
- New parser features: unit test with a schema that uses the feature
- New exporters: golden test comparing output against expected file
- Bug fixes: a test that would have caught the bug

**Golden test workflow:**

```bash
# Update golden files after intentional output changes
make test-golden UPDATE=true

# Verify golden files match current output
make test-golden
```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(generator): add credit_card semantic generator
fix(parser): handle quoted table names in CREATE TABLE
test(graph): add three-node cycle detection test
docs: update CLI reference for --tables flag
chore: bump golangci-lint to 1.55
```

---

## Pull Request Process

1. **Open an issue first** for anything non-trivial. Describe what you want to build and why. This prevents wasted effort.

2. **Fork and branch.** Branch naming: `feat/generator-creditcard`, `fix/parser-quoted-names`, `docs/cli-reference`.

3. **Write tests first** where possible.

4. **Run the full suite** before pushing:
   ```bash
   make test && make lint
   ```

5. **Fill out the PR template** completely. Incomplete PRs will be asked to revise.

6. **One concern per PR.** Don't bundle a bug fix with a new feature. Keep PRs reviewable.

---

## PR Template

When opening a PR, answer these questions:

```markdown
## What does this PR do?
[Short description]

## Why is it needed?
[Problem being solved or feature being added]

## How was it tested?
[Describe tests added or modified]

## Does it change any existing behavior?
[Yes/No — if yes, explain what and why]

## Checklist
- [ ] Tests added or updated
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Architectural rules respected (no cross-stage imports)
- [ ] Error messages follow SRS §12.2 format
- [ ] Deterministic (no time.Now() in pipeline)
```

---

## Issue Reporting

### Bug Reports

Include:
- SynthGraph version (`synthgraph version`)
- Your OS and Go version
- The SQL schema that caused the issue (or a minimal reproduction)
- The exact command you ran
- The full output / error message
- What you expected to happen

### Feature Requests

Include:
- The problem you're trying to solve (not just the solution)
- How you'd expect it to behave from a user perspective
- Whether you're willing to implement it

---

## Code of Conduct

- Be respectful in all interactions — issues, PRs, and discussions
- Critique code, not people
- Encourage contributors who are learning
- Label beginner-friendly issues as `good first issue`

This project follows a zero-tolerance policy for harassment of any kind.

---

## Questions?

Open a GitHub Discussion. Issues are for bugs and feature requests. Discussions are for questions and architectural conversations.
