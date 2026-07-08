# Contributing to SynthGraph

> **Who this is for:** Anyone who wants to contribute code, report bugs, or suggest features. Covers PR workflow, code style, testing rules, and architectural guidelines.
>
> **New to open source?** Don't worry — we welcome contributors of all levels. Check the `good first issue` labels on GitHub for beginner-friendly tasks.

Thank you for your interest in contributing. SynthGraph is built to be a long-term, widely adopted developer tool — and that only happens through a strong, thoughtful community.

This document explains how to contribute effectively.

---

## Before You Start

Read the [Architecture document](architecture.md) before making any significant contribution. SynthGraph has strict pipeline boundaries and architectural rules. Understanding them upfront will save you from rework.

---

## Development Setup

**Requirements:**
- Go 1.21 or higher
- `golangci-lint` (for linting)
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
go build ./...
go test ./...
```

---

## Core Design Principles

These rules exist to keep the codebase maintainable, readable, and approachable for developers of all skill levels.

### 1. Descriptive, Explicit Naming

- **No cryptic abbreviations**: Variable names like `tb`, `st`, `ci`, or `idx` are forbidden outside the tightest loops (and even then, descriptive names are strongly preferred). Use full words: `tableBuilder`, `schemaTranslator`, `columnIndex`, `targetIndex`.
- **Clear receiver names**: Unlike traditional Go conventions that favor 1-2 letter receivers (`func (s *Server)`), use fully spelled out receiver names: `func (translator *schemaTranslator)`.
- Variables should describe *what* they hold, not their underlying type.

### 2. Single Responsibility Principle (SRP)

- Methods should do exactly one thing. If a method needs paragraph comments like `// Validate Primary Keys` followed by `// Validate Unique Constraints`, those paragraphs belong in their own helper methods.
- Large validation logic or pipeline steps should be split into smaller, composable functions.

### 3. Avoid "Ugly" Conditionals

- Do not add hacky `if/else` blocks just to make tests pass.
- Use declarative boolean logic where applicable (e.g., `nullable = !raw.NotNull && !raw.IsPrimaryKey`).
- Reduce nesting with early returns to keep the "happy path" un-indented.

### 4. Self-Documenting Code over Comments

- Well-named functions and variables reduce the need for comments.
- Comments should explain *why* something is done (the business logic or an edge case), not *what* is being done (the code itself should explain what).

---

## Code Style

- Run `golangci-lint run ./...` before submitting. Zero warnings expected.
- Follow standard Go conventions (`gofmt`, `goimports`).
- Every exported function must have a doc comment.

---

## Architectural Rules

These are non-negotiable. PRs that violate them will not be merged.

- **No cross-stage imports.** The dependency rules in `architecture.md` are enforced. Parser does not import generator. Generator does not import validator. Etc.
- **No silent failures.** Every error path must return an error. No logging-and-continuing.
- **No global state.** All state flows through function arguments and return values.
- **Determinism is sacred.** No `time.Now()`, `rand.Int()` without seeding, or any other non-deterministic call in the generation pipeline.

---

## Testing Rules

Every PR must include tests. No exceptions.

- New generators: unit test asserting the value format
- New parser features: unit test with a schema that uses the feature
- New exporters: golden test comparing output against expected file
- Bug fixes: a test that would have caught the bug

Run tests before pushing:

```bash
go test ./... -v -count=1
```

---

## Pull Request Process

1. **Open an issue first** for anything non-trivial. Describe what you want to build and why. This prevents wasted effort.

2. **Fork and branch.** Branch naming: `feat/generator-creditcard`, `fix/parser-quoted-names`, `docs/cli-reference`.

3. **Write tests first** where possible.

4. **Run the full suite** before pushing.

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
- [ ] `go test ./...` passes
- [ ] Architectural rules respected (no cross-stage imports)
- [ ] Deterministic (no time.Now() in pipeline)
```

---

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(generator): add credit_card semantic generator
fix(parser): handle quoted table names in CREATE TABLE
test(graph): add three-node cycle detection test
docs: update CLI reference for --tables flag
chore: bump golangci-lint to 1.55
```

---

## Issue Reporting

### Bug Reports

Include:
- SynthGraph version
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
