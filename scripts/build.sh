#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
mkdir -p "$BIN"

echo "=== Building synthgraph (CLI) ==="
CGO_ENABLED=1 go build -o "$BIN/synthgraph"   "$ROOT/cmd/synthgraph/"

echo "=== Building synthgraph-web ==="
CGO_ENABLED=1 go build -o "$BIN/synthgraph-web" "$ROOT/cmd/synthgraph-web/"

echo "=== Building serveviz ==="
CGO_ENABLED=1 go build -o "$BIN/serveviz"     "$ROOT/cmd/serveviz/"

echo ""
echo "Done — binaries in $BIN/"
ls -lh "$BIN/"
