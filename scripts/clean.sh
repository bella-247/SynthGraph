#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Removing bin/ ==="
rm -rf "$ROOT/bin"

echo "=== Removing coverage.out ==="
rm -f "$ROOT/coverage.out"

echo "=== Go clean ==="
cd "$ROOT"
go clean -cache ./...

echo "=== Clean ==="
