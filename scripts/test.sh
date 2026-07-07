#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

usage() {
    cat <<EOF
Usage: ./scripts/test.sh [OPTION]

Options:
  unit        Run unit tests (no CGO) — skips parser tests
  all         Run full test suite (CGO required)
  coverage    Run with coverage report
  quick       Run only non-CGO packages (fastest)
  server      Run server package tests only
  parser      Run parser package tests only
  <pkg>       Run tests in a specific package (e.g. './internal/graph/...')
EOF
    exit 1
}

MODE="${1:-unit}"

case "$MODE" in
    unit)
        echo "=== Unit tests (no CGO) ==="
        go test ./internal/schema/... ./internal/graph/... ./internal/planner/... \
                ./internal/generator/... ./internal/exporter/... ./internal/semantic/... \
                -v -count=1 "$@"
        ;;
    all)
        echo "=== Full test suite (CGO) ==="
        CGO_ENABLED=1 go test ./... -v -count=1 "$@"
        ;;
    coverage)
        echo "=== Coverage report ==="
        CGO_ENABLED=1 go test ./... -coverprofile="$ROOT/coverage.out" -count=1 "$@"
        go tool cover -func="$ROOT/coverage.out" | tail -1
        echo ""
        echo "HTML report: go tool cover -html=$ROOT/coverage.out"
        ;;
    quick)
        echo "=== Quick: non-CGO packages only ==="
        go test ./internal/... ./cmd/synthgraph/... ./cmd/synthgraph-web/server/... \
                -count=1 "$@"
        ;;
    server)
        echo "=== Server tests ==="
        go test ./cmd/synthgraph-web/server/... -v -count=1 "$@"
        ;;
    parser)
        echo "=== Parser tests (CGO required) ==="
        CGO_ENABLED=1 go test ./internal/parser/... -v -count=1 "$@"
        ;;
    *)
        echo "=== Tests: $MODE ==="
        CGO_ENABLED=1 go test "$MODE" -v -count=1 "$@"
        ;;
esac
