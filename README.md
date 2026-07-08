# SynthGraph

Generate mathematically valid, constraint-compliant synthetic data from your SQL schema.

No foreign key violations. No duplicate uniques. No manually ordered seed files. SynthGraph reads your schema, builds it as a dependency graph, and generates data that respects every constraint — automatically.

```sql
-- Your schema
CREATE TABLE users   (id INT PRIMARY KEY);
CREATE TABLE orders  (id INT PRIMARY KEY, user_id INT REFERENCES users(id));
```

```sql
-- What most generators give you
INSERT INTO orders (user_id) VALUES (9999);
-- ERROR: foreign key constraint violation. user 9999 does not exist.
```

```sql
-- What SynthGraph gives you
INSERT INTO orders (user_id) VALUES (23);
-- user_id 23 is guaranteed to exist in users
```

---

## Try it: the web app

The easiest way to use SynthGraph is the built-in web app — a full visual pipeline from schema to downloadable seed file.

```bash
CGO_ENABLED=1 go run ./cmd/synthgraph-web/
```

Open **http://localhost:8080**. You'll move through five steps:

| Step | What you do |
|---|---|
| **1. Schema** | Paste or upload your SQL DDL |
| **2. Graph** | Explore your schema as an interactive graph (Cytoscape.js) |
| **3. Semantic** | See how each table was classified (entity, junction, lookup, transactional) |
| **4. Generate** | Set row count, seed, and output format |
| **5. Download** | Get your seed file as SQL or CSV |

This is the recommended starting point for most users — no flags to remember, and you can see your schema's structure before generating anything.

---

## How it works

SynthGraph treats your schema as a directed graph: tables are nodes, foreign keys are edges.

```
users ──────────► orders ──────────► payments
                              │
                              └──────► refunds
```

From there, it:

1. **Sorts tables topologically**, so dependencies are always generated before the tables that reference them
2. **Detects circular dependencies** (e.g. `users → organizations → users`) using Tarjan's algorithm, and resolves them by inserting nullable placeholder rows first, then backfilling once both sides exist
3. **Generates every value with full awareness of every constraint** — primary keys are unique, foreign keys always reference real rows, unique constraints hold, enums only use declared values
4. **Infers meaning from column names** — a column named `email` gets a real email, `country` gets a real country, `created_at` gets a realistic timestamp
5. **Produces identical output for the same schema and seed**, so your test environments are reproducible

---

## What it supports

| Constraint | Status |
|---|---|
| Primary Key (single + composite) | ✅ |
| Foreign Key (single + composite) | ✅ |
| Unique (single + composite) | ✅ |
| NOT NULL | ✅ |
| Enum values | ✅ |
| VARCHAR length | ✅ |
| DECIMAL precision/scale | ✅ |
| Circular FK dependencies | ✅ |

---

## Using the CLI

If you prefer scripting, CI pipelines, or just the terminal, the same engine is available as a CLI.

**Install:**

```bash
CGO_ENABLED=1 go install ./cmd/synthgraph@latest
```

**Generate a seed file:**

```bash
synthgraph generate --input schema.sql --rows 100 --output seed.sql
```

**Inspect your schema before generating anything:**

```bash
synthgraph inspect --input schema.sql -v
```

```
SynthGraph Schema Inspection
─────────────────────────────────────────
Tables:             8
Relationships:      11
Cycles Detected:    1
Deepest Chain:      4

Cycle Details:
  Cycle 1: users → organizations → users
  Breakpoint: organizations.owner_id (nullable) ✓

Generation Order:
  1. roles
  2. users                [deferred FK: organization_id]
  3. organizations
  4. products
  5. orders
  6. order_items
  7. payments
  8. refunds
─────────────────────────────────────────
```

### CLI reference

**`generate`**

| Flag | Description | Default |
|---|---|---|
| `--input, -i` | Path to SQL schema file (required) | — |
| `--output, -o` | Output file path | stdout |
| `--format, -f` | `sql` or `csv` | `sql` |
| `--rows, -r` | Rows per table | `10` |
| `--seed, -s` | RNG seed, for reproducibility | `42` |
| `--schema-name` | Schema name for SQL output | — |

**`inspect`**

| Flag | Description |
|---|---|
| `--input, -i` | Path to SQL schema file (required) |
| `--graph` | Show graph structure summary |
| `--semantic` | Show semantic inference summary |
| `-v` | Verbose (`--graph` + `--semantic`) |

**`version`** — prints the installed SynthGraph version.

There's also `serveviz`, a lightweight, read-only version of the graph explorer for local development (no generation UI):

```bash
CGO_ENABLED=1 go run ./cmd/serveviz/ --schema schema.sql
```

---

## Full example

**Input** (`ecommerce.sql`):

```sql
CREATE TABLE users (
    id         SERIAL PRIMARY KEY,
    email      VARCHAR(255) NOT NULL UNIQUE,
    first_name VARCHAR(100) NOT NULL,
    last_name  VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE products (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    price       DECIMAL(10,2) NOT NULL,
    description TEXT
);

CREATE TABLE orders (
    id         SERIAL PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id),
    total      DECIMAL(10,2) NOT NULL,
    status     VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Command:**

```bash
synthgraph generate --schema ecommerce.sql --rows 50
```

**Output:**

```sql
-- Generated by SynthGraph v1.0.0
-- Schema: ecommerce.sql
-- Rows per table: 50
-- Seed: 42

BEGIN;

-- Table: users (50 rows)
INSERT INTO users (id, email, first_name, last_name, created_at)
VALUES
  (1, 'alice.johnson@example.com', 'Alice', 'Johnson', '2024-03-15 09:22:11'),
  (2, 'bob.smith@example.net', 'Bob', 'Smith', '2024-07-02 14:55:30'),
  ...
;

-- Table: products (50 rows)
INSERT INTO products (id, name, price, description)
VALUES
  (1, 'Wireless Headphones', 89.99, 'Lorem ipsum dolor sit amet.'),
  ...
;

-- Table: orders (50 rows)
INSERT INTO orders (id, user_id, total, status, created_at)
VALUES
  (1, 23, 142.50, 'pending', '2024-09-10 11:30:00'),
  -- user_id 23 is guaranteed to exist in users
  ...
;

COMMIT;
```

Every FK value exists. Every email is unique. The file runs clean, first try.

---

## Architecture

SynthGraph is a strict linear pipeline. Every stage consumes one typed artifact and produces another — and every parser (Postgres today, others later) feeds into the same canonical model, so the graph engine, planner, generator, and exporter never need to know where the schema came from.

```
SQL File → Parser → AST → Translator → schema.Model → Graph Builder
→ SchemaGraph → Planner → GenerationPlan → Generator → Dataset → Exporter → SQL / CSV
```

Full details: [`docs/architecture.md`](docs/architecture.md)

---

## Future Plans

See [`docs/Future-Plan.md`](docs/Future-Plan.md) for planned features including additional parsers, export formats, streaming generation, and more.

---

## Development

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) for setup, building (with/without CGO), testing, and debugging.

Future enhancements: [`docs/Future-Plan.md`](docs/Future-Plan.md)

## Contributing

Contributions welcome — read [`CONTRIBUTING.md`](CONTRIBUTING.md) first. Good first issues: new semantic field generators, translator test coverage for Postgres edge cases, new golden test schemas, better error messages.

## License

MIT — see [`LICENSE`](LICENSE)