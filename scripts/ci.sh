#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==================================="
echo "  SynthGraph CI Pipeline"
echo "==================================="

echo ""
echo "=== Step 1: go vet ==="
go vet ./...

echo ""
echo "=== Step 2: Build all binaries ==="
"$ROOT/scripts/build.sh"

echo ""
echo "=== Step 3: Full test suite ==="
CGO_ENABLED=1 go test ./... -race -timeout=180s -count=1

echo ""
echo "==================================="
echo "  All checks passed!"
echo "==================================="
