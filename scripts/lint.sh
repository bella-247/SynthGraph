#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "=== go vet ==="
go vet ./...

echo ""
echo "=== gofmt ==="
UNFORMATTED="$(gofmt -l "$ROOT" | grep -v '^vendor/' || true)"
if [ -n "$UNFORMATTED" ]; then
    echo "The following files need formatting:"
    echo "$UNFORMATTED"
    exit 1
fi
echo "All files are formatted correctly."

echo ""
echo "=== Lint passed ==="
