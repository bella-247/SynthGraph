# SynthGraph — CLI Reference

Complete reference for all SynthGraph commands and flags.

---

## Global Flags

These flags apply to all commands.

| Flag | Default | Description |
|---|---|---|
| `--config <path>` | — | Path to YAML config file. CLI flags override config values |
| `--init-config [<path>]` | `synthgraph.yaml` | Write a commented default config template and exit |
| `--help` | — | Print help for the current command |

---

## `synthgraph generate`

Generates a synthetic dataset from a schema file.

### Usage

```bash
synthgraph generate --input <path> [flags]
```

### Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--input`, `-i` | Yes | — | Path to the SQL schema file |
| `--output`, `-o` | No | stdout | Output file path |
| `--format`, `-f` | No | `sql` | Output format: `sql` or `csv` |
| `--rows`, `-r` | No | `10` | Number of rows to generate per table |
| `--seed`, `-s` | No | `42` | Integer seed for deterministic generation |
| `--schema-name` | No | — | Schema name for SQL output (optional) |
| `--verbose`, `-v` | No | false | Enable debug-level logging to stderr |
| `--config` | No | — | Path to YAML config file |

### Examples

**Generate 100 rows per table, output to stdout:**
```bash
synthgraph generate --input schema.sql
```

**Generate 500 rows as CSV, save to file:**
```bash
synthgraph generate --input schema.sql --format csv --rows 500 --output data.csv
```

**Use config file:**
```bash
synthgraph generate --config synthgraph.yaml
```

---

## `synthgraph inspect`

Analyzes a schema file and prints its structure, graph, and semantic roles.

### Usage

```bash
synthgraph inspect --input <path> [flags]
```

### Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--input`, `-i` | Yes | — | Path to the SQL schema file |
| `--graph` | No | false | Show graph structure summary (nodes, edges, dependencies) |
| `--semantic` | No | false | Show semantic role inference per table |
| `--verbose`, `-v` | No | false | Enable debug logging and show all summaries |
| `--config` | No | — | Path to YAML config file |

### Examples

**Quick schema overview:**
```bash
synthgraph inspect --input schema.sql
```

**Full analysis with graph and semantics:**
```bash
synthgraph inspect --input schema.sql --graph --semantic
```

**Verbose mode (all summaries + debug logs):**
```bash
synthgraph inspect --input schema.sql -v
```

---

## `synthgraph version`

Prints version and repository information.

### Usage

```bash
synthgraph version
```

---

## Configuration File

All flags can be specified in a YAML config file and passed with `--config`.

```yaml
# synthgraph.yaml
verbose: false

generate:
  input: schema.sql
  output: data.csv
  format: csv
  rows: 100
  seed: 42
  schema-name: public

inspect:
  input: schema.sql
  graph: true
  semantic: true
```

CLI flags **always take precedence** over config file values. Use `--init-config` to generate a template:

```bash
synthgraph generate --init-config
```

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Error (parsing failure, generation failure, invalid flags) |
