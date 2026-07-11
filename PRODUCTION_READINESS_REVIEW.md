# SynthGraph v1.0 Readiness Review

**Repository:** `github.com/abel-me/synthgraph`
**Language:** Go 1.26.4
**Codebase:** ~90 source files, 27 test files, 10 packages, ~10,000 lines
**Tool type:** Local CLI + optional web UI for generating relational test data

---

## 1. Executive Summary

SynthGraph generates constraint-compliant relational test data from DDL. The core pipeline (parse → graph → plan → generate → export) is well-architected, modular, and tests pass cleanly — 11/11 packages, all green, including the CGO-dependent parser.

This is a **local developer tool**, not an internet-facing SaaS. The v1.0 bar is different: correctness, trust, documentation, and developer experience matter more than infrastructure patterns designed for high-concurrency services.

**The codebase is close to v1.0.** The must-fix list is short: format all Go files, add integration tests, cap input sizes, fix the CGO failure behavior, improve error messages, and polish docs. Everything else is "nice to have" or "can wait."

---

## 2. Actual Checks Run

| Check | Result |
|-------|--------|
| `go vet ./...` | 0 errors |
| `gofmt -l .` | **102 files unformatted** |
| `go build ./...` | Compiles clean |
| `go test ./...` | 11/11 pass in 46.6s |
| Graceful shutdown | Present (SIGINT/SIGTERM → `Server.Shutdown()`) |
| Context propagation | Present in generator + SSE handler |
| Health endpoint | `/api/health` with version, uptime, goroutines, jobs |
| HTTP timeouts | Configured (Read=30s, Write=0, Idle=120s, Gen=10m) |
| Input size limits | **None** |

---

## 3. Must Fix Before v1.0

### 1. gofmt — 102 files unformatted

```
gofmt -w .
```

This is non-negotiable for a Go project. Every Go developer expects `gofmt`-compliant code. This should never reach a public release.

### 2. Integration tests

Tests cover individual packages well, but no single test exercises the full pipeline end-to-end: `ParseDDL → graph.Build → planner.Plan → generator.Generate → exporter.Export`. Add at least one golden-file integration test that takes SQL DDL and compares output against a known result. A CLI integration test using `os/exec` on `cmd/synthgraph` would catch regressions the unit tests miss.

### 3. Input size limits

No limit on input DDL size, column count, or rows requested. `rows=999999999` will OOM the process. This isn't a security issue — it's protecting users from themselves. Add:

```go
const MaxRows = 100000
const MaxDDLSize = 10 << 20 // 10 MB
```

Enforce at both CLI and HTTP handler level.

### 4. CGO failure behavior — verify and document

Check what happens when someone builds without CGO. If `adapter_nocgo.go` silently produces garbage output, that's a bug — fix it to fail loudly:

```go
func init() {
    fmt.Fprintln(os.Stderr, "SynthGraph requires CGO. Install GCC or Mingw-w64.")
    os.Exit(1)
}
```

If it already fails cleanly, just document the CGO requirement prominently in README and install docs. **Do not spend months writing a pure-Go SQL parser.** `pg_query_go` is battle-tested; use it.

### 5. Error messages

This is the highest-ROI improvement for user trust. Replace "parse failed" with structured errors:

```
Parse error at line 42, column 18:
Expected FOREIGN KEY after REFERENCES, found: PRIMARY
Hint: Did you forget a comma?
```

The parser package already has a `ParseError` type with line/col — make sure it surfaces all the way to the user in both CLI and web UI.

### 6. Documentation

The README exists but needs a v1.0 pass. Prioritize:

- **Quick start** — copy-paste DDL, run, see output (the current walkthrough is good, lead with it)
- **Installation** — include Windows, macOS, Linux; document CGO requirement
- **Examples** — 3 real-world schemas with expected output
- **How deterministic generation works** — explain seed, reproducibility
- **How cycles are resolved** — users will hit this with real schemas
- **FAQ / Troubleshooting** — common errors and what they mean

A week on docs is worth more than a week on infrastructure for a v1.0 CLI tool.

---

## 4. Nice to Have (but not blockers)

These improve quality but shouldn't delay v1.0.

| Item | Why wait |
|------|----------|
| Progress bars / better CLI UX | Adds polish, not correctness. The pipeline already reports per-table progress via SSE. |
| Structured logging (`slog`) | `fmt.Printf` is fine for a local CLI tool. Many successful Go tools use it for years. |
| Request cancellation via context | Partially done. Full chain propagation is nice but unlikely to fail in practice. |
| Health endpoint improvements | Already exists. Can be enhanced later. |
| `sync.RWMutex` in job store | The job store serves one user on localhost. Mutex contention is not real. |
| RNG statistical bias audit | FNV-64a passes uniformity tests at the row counts this tool generates. |
| Parser registry cleanup | Unused abstraction. Delete it or leave it. Neither blocks a release. |

---

## 5. Items Removed (SaaS assumptions)

These came from a "production service" mental model that doesn't fit this tool:

- **Circuit breakers** — SynthGraph has no external service dependencies to protect against. A parser failure returns an error. PostgreSQL unavailability returns an error. There's nothing to circuit-break.
- **Load testing for concurrency** — One developer on localhost. The useful benchmark is "generate 100k rows — what's the memory/time/correctness profile?" not "500 concurrent users."
- **Advanced observability (tracing, OpenTelemetry)** — Valuable for a microservice. Overkill for a CLI binary.
- **Plugin systems / semantic inferrer registry** — Premature abstraction. Refactor when there's a second dialect.

---

## 6. Versioning Recommendation

**Do not call it "beta."** Semantic versioning already communicates instability at 0.x.

```
v0.1.0
```

not `v0.1.0-beta`. The `0` major version tells users APIs may change. Reserve "beta" for the pre-release semver extension (`v0.1.0-beta.1`) if you want test releases.

---

## 7. Scorecard Updated

| Dimension | Score | Notes |
|-----------|-------|-------|
| **Correctness** | 75/100 | Tests pass, pipeline is solid. Missing integration test is the gap. |
| **Code Quality** | 70/100 | 102 unformatted files drags this down. Easy fix. |
| **Developer Experience** | 55/100 | Error messages and docs need a pass. Pipeline UX is good. |
| **Documentation** | 60/100 | README exists and has a walkthrough. Needs install docs, FAQ, troubleshooting. |
| **Reliability** | 70/100 | Graceful shutdown and context propagation present. Input limits missing. |
| **Test Coverage** | 65/100 | Good unit coverage. Missing end-to-end integration test. |
| **Build** | 60/100 | CGO is a friction point. Document clearly and move on. |
| **Composite** | **65/100** | |

---

## 8. Verdict

**✅ Pass — ready for v0.1.0 after fixing 6 items.**

The most valuable thing you can do between now and release:

1. Run `gofmt -w .` (5 seconds)
2. Write 2 integration tests (half a day)
3. Add `MaxRows`/`MaxDDLSize` constants (30 minutes)
4. Verify CGO failure is loud, not silent (15 minutes)
5. Polish error messages to show line:col + hint (one day)
6. Polish README and add install docs (one week)

That's roughly one week of work. Ship as `v0.1.0`. The architecture is sound, the pipeline works, and the value proposition is clear.
