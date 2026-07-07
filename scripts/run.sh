#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"

# Build if binary doesn't exist
if [ ! -f "$BIN/synthgraph" ] || [ ! -f "$BIN/synthgraph-web" ]; then
    "$ROOT/scripts/build.sh"
fi

usage() {
    cat <<EOF
Usage: ./scripts/run.sh <command> [args]

Commands:
  web [port]       Start web server (default port 9090)
  cli <args>...    Run synthgraph CLI with sample schema
  viz <schema>     Start serveviz with a schema file
EOF
    exit 1
}

CMD="${1:-}"
shift || true

case "$CMD" in
    web)
        PORT="${1:-9090}"
        echo "==> Starting synthgraph-web on port $PORT"
        "$BIN/synthgraph-web" --port "$PORT"
        ;;
    cli)
        SCHEMA="${1:-$ROOT/testdata/schemas/sakila.sql}"
        shift 2>/dev/null || true
        echo "==> Running synthgraph CLI with schema: $SCHEMA"
        "$BIN/synthgraph" generate --input "$SCHEMA" --rows 50 --output /dev/stdout "$@"
        ;;
    viz)
        SCHEMA="${1:-$ROOT/testdata/schemas/sakila.sql}"
        echo "==> Starting serveviz with schema: $SCHEMA"
        "$BIN/serveviz" --schema "$SCHEMA"
        ;;
    *)
        usage
        ;;
esac
