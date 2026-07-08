# SynthGraph — CLI Reference

Complete guide to using SynthGraph from the command line.

---

## Before you start

### What you need

- **Go 1.21+** installed (`go version` to check)
- **GCC** installed (for CGO — [Windows setup help](#windows-setup))

### Install the CLI

```bash
CGO_ENABLED=1 go install ./cmd/synthgraph@latest
```

Or build locally:
```bash
CGO_ENABLED=1 go build -o synthgraph.exe ./cmd/synthgraph/
```

### Windows setup

Install GCC via MSYS2, then add it to your PATH:
```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
# Add C:\msys64\ucrt64\bin to your PATH
```

---

## `synthgraph generate` — Generate synthetic data

This is the main command. Feed it a SQL schema, get back a seed file.

### Basic usage

```bash
# Minimal — 10 rows per table, output to screen
synthgraph generate --input schema.sql

# 100 rows per table, save to file
synthgraph generate --input schema.sql --rows 100 --output seed.sql

# CSV instead of SQL
synthgraph generate --input schema.sql --format csv --output data.csv

# Reproducible output (same seed = same data every time)
synthgraph generate --input schema.sql --seed 42
```

### All flags

| Flag | Short | Required | Default | What it does |
|------|-------|----------|---------|-------------|
| `--input` | `-i` | Yes | — | Path to your SQL schema file |
| `--output` | `-o` | No | stdout | Where to save the output file |
| `--format` | `-f` | No | `sql` | `sql` or `csv` |
| `--rows` | `-r` | No | `10` | Number of rows per table |
| `--seed` | `-s` | No | `42` | Random seed (same seed = same data) |
| `--schema-name` | — | No | — | Schema name for SQL output (e.g., `public`) |
| `--verbose` | `-v` | No | — | Show detailed progress |

### Examples by use case

**Small test dataset (5 rows, quick check):**
```bash
synthgraph generate --input schema.sql --rows 5 --seed 123
```

**Large dataset for performance testing:**
```bash
synthgraph generate --input schema.sql --rows 10000 --seed 42 --format csv --output ./fixtures/
```

**Using a YAML config file:**
```bash
synthgraph generate --config synthgraph.yaml
```

```yaml
# synthgraph.yaml
generate:
  input: schema.sql
  format: csv
  rows: 500
  seed: 2024
```

---

## `synthgraph inspect` — Analyze your schema

Shows you what SynthGraph sees in your schema before generating anything.

### Basic usage

```bash
# Quick overview (tables, columns, enums)
synthgraph inspect --input schema.sql

# Show graph structure (how tables relate)
synthgraph inspect --input schema.sql --graph

# Show semantic analysis (what each table "means")
synthgraph inspect --input schema.sql --semantic

# Full analysis
synthgraph inspect --input schema.sql -v
```

### Example output

```
Schema Overview
===============
Tables: 3
Enums:  1

Table: users
  Columns: 6
  Primary Key: id
    id          integer PK
    email       text NOT NULL
    created_at  timestamp DEFAULT NOW()
    ...
```

### All flags

| Flag | Short | Default | What it does |
|------|-------|---------|-------------|
| `--input` | `-i` | — | Path to SQL schema file (required) |
| `--graph` | — | false | Show table dependency graph |
| `--semantic` | — | false | Show semantic table roles |
| `--verbose` | `-v` | false | Show everything |

---

## `synthgraph version`

Prints the installed version.

```bash
synthgraph version
# → synthgraph version 0.1.0
```

---

## FAQ

### "gcc: error: unrecognized command-line option"

You're on Windows with a GCC version mismatch. Install the **ucrt64** variant of MinGW-w64:
```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
```
Make sure `where gcc` points to `C:\msys64\ucrt64\bin\gcc.exe`.

### "postgres.h: No such file or directory"

The parser needs PostgreSQL headers during compilation. On Linux: `sudo apt-get install libpq-dev`. On macOS: `brew install libpq`. On Windows: MinGW-w64 includes everything needed.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (bad input, generation failure, etc.) |
| `3` | Unresolvable circular dependency |
| `4` | Can't satisfy a constraint (e.g., too many unique values) |

### How do I get the same data twice?

Use the same `--seed` value. SynthGraph is fully deterministic — same schema + same seed = identical output every time.
