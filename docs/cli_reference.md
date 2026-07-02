# SynthGraph — CLI Reference

Complete reference for all SynthGraph commands and flags.

---

## Global Flags

These flags apply to all commands.

| Flag | Default | Description |
|---|---|---|
| `--verbose` | false | Print debug-level output to stderr (parser events, graph construction, generation progress) |
| `--help` | — | Print help for the current command |

---

## `synthgraph generate`

Generates a synthetic dataset from a schema file.

### Usage

```bash
synthgraph generate --schema <path> [flags]
```

### Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--schema` | Yes | — | Path to the SQL schema file |
| `--rows` | No | `100` | Number of rows to generate per table |
| `--output` | No | stdout | Output file path (SQL) or directory path (CSV) |
| `--format` | No | `sql` | Output format: `sql` or `csv` |
| `--seed` | No | `42` | Integer seed for deterministic generation. Same seed + same schema = identical output |
| `--tables` | No | all | Comma-separated list of tables to include in output. Tables they depend on are always generated even if not listed |

### Examples

**Generate 100 rows per table, output to stdout:**
```bash
synthgraph generate --schema schema.sql
```

**Generate 500 rows per table, write to file:**
```bash
synthgraph generate --schema schema.sql --rows 500 --output seed.sql
```

**Generate CSV files into a directory:**
```bash
synthgraph generate --schema schema.sql --rows 200 --format csv --output ./fixtures/
```

**Reproducible generation with explicit seed:**
```bash
synthgraph generate --schema schema.sql --rows 100 --seed 12345
```

**Generate only specific tables (dependencies auto-included):**
```bash
synthgraph generate --schema schema.sql --tables orders,payments
```

**Debug mode — see what the parser and graph engine are doing:**
```bash
synthgraph generate --schema schema.sql --verbose
```

### Output

On success, the SQL output is written to stdout (or `--output` path) and a summary is printed to stderr:

```
✓ Parsed schema: 8 tables, 11 relationships
✓ Graph built: 1 cycle detected, resolved via nullable deferred insertion
✓ Generated: 800 rows across 8 tables
✓ Validated: all constraints satisfied
✓ Exported: seed.sql (42.3 KB)

Generation time: 0.8s
```

### Error Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Schema parse error |
| 2 | Unsupported feature |
| 3 | Unresolvable cycle |
| 4 | Generation constraint failure |
| 5 | Post-generation validation failure |
| 6 | I/O error |
| 99 | Internal error (bug) |

---

## `synthgraph inspect`

Analyzes a schema file and prints graph statistics. No data is generated.

### Usage

```bash
synthgraph inspect <schema-path>
```

### Example

```bash
synthgraph inspect ./schema.sql
```

### Output

```
SynthGraph Schema Inspection
─────────────────────────────────────────
Schema File:        schema.sql
Tables:             12
Relationships:      18
Cycles Detected:    1
Isolated Tables:    2 (audit_logs, feature_flags)
Deepest Chain:      5 hops

  users → orders → payments → refunds → disputes

Cycle Details:
  Cycle 1: users → organizations → users
    Edge:  organizations.owner_id → users.id
    Status: ✓ Resolvable (owner_id is nullable)
    Breakpoint: organizations.owner_id

Generation Order (if `generate` were run):
  1.  feature_flags          [isolated]
  2.  audit_logs             [isolated]
  3.  roles
  4.  users                  [deferred FK: organization_id]
  5.  organizations
  6.  products
  7.  categories
  8.  order_items
  9.  orders
  10. payments
  11. refunds
  12. disputes

Warnings:
  ⚠ CHECK constraint on orders.amount will not be enforced (V1 limitation)
  ⚠ CHECK constraint on products.price will not be enforced (V1 limitation)

─────────────────────────────────────────
Estimated generation time: ~1.2s (100 rows/table)
```

### Exit Codes

| Code | Meaning |
|---|---|
| 0 | Schema is valid and fully resolvable |
| 1 | Schema parse error |
| 3 | Unresolvable cycle detected |
| 6 | I/O error |

---

## `synthgraph version`

Prints the current SynthGraph version.

### Usage

```bash
synthgraph version
```

### Output

```
SynthGraph v1.0.0
```

---

## Output Format Details

### SQL Format

The SQL output file contains:

1. A header comment block with metadata
2. `BEGIN;` to start a transaction
3. `INSERT INTO` statements for each table, grouped and in generation order
4. `UPDATE` statements for any deferred FK backfills (cycle resolution)
5. `COMMIT;` to close the transaction

The entire output is wrapped in a transaction so it either fully succeeds or fully rolls back when executed against a real database.

### CSV Format

When `--format csv` is used, one `.csv` file is created per table. Files are placed in the directory specified by `--output` (defaults to current directory).

File naming: `{table_name}.csv`

Format:
- First row: column names as header
- Subsequent rows: data values
- All values are quoted
- RFC 4180 compliant
- UTF-8 encoding

Example `users.csv`:
```csv
"id","email","first_name","last_name","created_at"
"1","alice@example.com","Alice","Johnson","2024-03-15 09:22:11"
"2","bob@example.net","Bob","Smith","2024-07-02 14:55:30"
```

---

## Tips

**Testing your schema before generating:**
Always run `inspect` before `generate` on a new schema. It will catch cycles and unsupported constructs before you wait for generation to fail.

**Reproducible test fixtures:**
Always specify `--seed` explicitly in CI environments. The default seed (42) is stable, but being explicit makes intent clear.

**Large schemas:**
Use `--tables` to generate data for a subset of tables when you only need specific fixtures. Dependencies are always included automatically.

**Debugging generation issues:**
Add `--verbose` to see exactly what the parser found, how the graph was built, and what order tables are being generated in.
