# SynthGraph

**Paste your SQL schema — get realistic test data. Foreign keys match, emails are unique, nothing breaks.**

```sql
-- You write this (your database schema)
CREATE TABLE users   (id INT PRIMARY KEY, email VARCHAR(255) UNIQUE);
CREATE TABLE orders  (id INT PRIMARY KEY, user_id INT REFERENCES users(id));

-- SynthGraph generates this (realistic test data)
INSERT INTO users  VALUES (1, 'alice@example.com'), (2, 'bob@example.com');
INSERT INTO orders VALUES (1, 1, 'pending'),        (2, 2, 'shipped');
-- Every order.user_id matches a real user. Every email is unique. It just works.
```

No manual seed files. No foreign key violations.

---

## Table of Contents

- [What is SynthGraph?](#what-is-synthgraph)
- [Quick Install](#quick-install)
- [2-Minute Walkthrough](#2-minute-walkthrough)
- [How It Works](#how-it-works)
- [Notable Features](#notable-features)
- [Web App](#web-app)
- [Commands](#commands)
- [Further Reading](#further-reading)
- [License](#license)

---

## What is SynthGraph?

SynthGraph is a CLI tool (with an optional web UI) that generates constraint-compliant test data from SQL DDL schemas.

You give it a `CREATE TABLE` schema. It parses it, understands every primary key, foreign key, unique constraint, enum type, and default value — then generates INSERT statements where every reference is valid, every email is unique, and every NOT NULL column has a meaningful value.

It supports PostgreSQL syntax and generates SQL or CSV output.

---

## Quick Install

### Pre-built binary (easiest — no compiler needed)

Download the latest release for your platform from [github.com/bella-247/SynthGraph/releases](https://github.com/bella-247/SynthGraph/releases):

| Platform | CLI | Web UI |
|----------|-----|--------|
| Linux (amd64) | `synthgraph-linux-amd64` | `synthgraph-web-linux-amd64` |
| Linux (arm64) | `synthgraph-linux-arm64` | `synthgraph-web-linux-arm64` |
| macOS (Intel) | `synthgraph-darwin-amd64` | `synthgraph-web-darwin-amd64` |
| macOS (Apple Silicon) | `synthgraph-darwin-arm64` | `synthgraph-web-darwin-arm64` |
| Windows | `synthgraph-windows-amd64.exe` | `synthgraph-web-windows-amd64.exe` |

Download, extract, run. That's it.

### One-liner scripts

```bash
# Linux / macOS
curl -sSf https://raw.githubusercontent.com/bella-247/SynthGraph/main/scripts/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/bella-247/SynthGraph/main/scripts/install.ps1 | iex
```

### Go users (build from source)

```bash
CGO_ENABLED=1 go install github.com/bella-247/SynthGraph/cmd/synthgraph@latest
```

> **Why CGO?** SynthGraph uses the real PostgreSQL parser (written in C) to understand SQL schemas. Go needs CGO to link against it. The pre-built binaries include the C library — you only need CGO if you're building from source.

---

## 2-Minute Walkthrough

### 1. Create a schema file

Save this as `shop.sql`:

```sql
CREATE TABLE users (
    id         INT PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    email      VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE products (
    id    INT PRIMARY KEY,
    name  VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL
);

CREATE TABLE orders (
    id      INT PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    product_id INT NOT NULL REFERENCES products(id),
    status  VARCHAR(50) DEFAULT 'pending'
);
```

### 2. Generate data

```bash
synthgraph generate -i shop.sql -o seed.sql
```

### 3. Load into your database

```bash
psql -d mydb -f seed.sql
```

You now have 10 users, 10 products, and 10 orders — all referencing real rows.

### More options

```bash
synthgraph generate -i shop.sql -r 100          # 100 rows per table
synthgraph generate -i shop.sql -f csv           # CSV output
synthgraph generate -i shop.sql -s 12345         # fixed seed for repeatable data
synthgraph generate -i shop.sql --schema-name public   # schema-qualified names
```

---

## How It Works

SynthGraph processes your schema through a pipeline of six stages, each a pure function:

```text
SQL → Parse → Graph → Plan → Generate → Validate → SQL / CSV
```

| Stage | What happens |
|-------|-------------|
| **Parse** | Reads your SQL DDL using the real PostgreSQL parser (via CGO). Extracts tables, columns, types, constraints, enums. |
| **Graph** | Builds a dependency graph: which tables reference which other tables through foreign keys. Detects cycles (e.g., A → B → A). |
| **Plan** | Topologically sorts tables so parents generate before children. Breaks cycles with a null-first row strategy. |
| **Generate** | Walks every table in order, producing rows with realistic values — names, emails, phone numbers, timestamps, addresses — determined by column name and type. Deterministic seeding means the same schema + seed always produces the same data. |
| **Validate** | Re-checks every constraint (FK, unique, NOT NULL) against the generated output. Catches any edge cases before they reach your database. |
| **Export** | Formats the result as SQL INSERT statements or CSV. |

For a deep dive into each stage, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Notable Features

- **Constraint-aware generation** — foreign keys always reference real rows, emails are unique, NOT NULL columns always have values, VARCHAR lengths are respected
- **Cycle-safe** — even circular FK dependencies (e.g., `employees.manager_id REFERENCES employees(id)`) are handled gracefully using null-first rows
- **Deterministic by default** — same schema + same seed = same output every time. No surprises between runs
- **Realistic data** — column names drive value selection (columns named `email` get emails, `phone` gets phone numbers, `city` gets city names). No random garbage
- **Schema inference** — detects audit columns (`created_at`, `updated_at`), naming patterns, and role-like columns (`is_admin`, `status`) to generate appropriate values
- **PostgreSQL-native parsing** — uses the actual PostgreSQL parser via CGO. Handles `CREATE TYPE ... AS ENUM`, `DEFAULT` expressions, composite keys, and more
- **Web UI included** — visual schema diagram, step-by-step generation, job history, one-click downloads
- **Dual output** — SQL or CSV. Pipe directly into your migration scripts

---

## Web App

SynthGraph includes a browser-based UI at `localhost:8080`:

```bash
synthgraph-web
```

The web app guides you through 4 steps:

| Step | What you do | What you see |
|------|-------------|--------------|
| **Schema** | Paste SQL or pick a template | Parsed tables, columns, types, enums |
| **Graph** | Explore the diagram | Tables as boxes with FK arrows |
| **Generate** | Set row count, click Generate | Live pipeline progress |
| **History** | Browse past jobs | Download any result again |

---

## Commands

### `synthgraph generate`

Generate synthetic data from a SQL schema.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--input` | `-i` | — | Path to your `.sql` schema file (required) |
| `--output` | `-o` | stdout | File to write to (omit to print to terminal) |
| `--rows` | `-r` | `10` | Rows per table (max: 100,000) |
| `--format` | `-f` | `sql` | Output format: `sql` or `csv` |
| `--seed` | `-s` | `42` | Random seed — same seed = same data every time |
| `--verbose` | `-v` | — | Show detailed progress |
| `--schema-name` | — | `""` | Schema name for SQL output (e.g., `public`) |
| `--config` | `-c` | — | Path to YAML config file |
| `--init-config` | — | — | Write a default YAML config template and exit |

**Examples:**

```bash
synthgraph generate -i schema.sql                          # 10 rows, SQL output
synthgraph generate -i schema.sql -o data.sql              # save to file
synthgraph generate -i schema.sql -r 1000 -f csv -o data.csv  # 1000 rows as CSV
synthgraph generate -i schema.sql --config synthgraph.yaml     # use YAML config
```

### `synthgraph inspect`

Analyze a schema and print its structure — tables, columns, types, enums, and inferred semantics.

```bash
synthgraph inspect -i schema.sql
synthgraph inspect -i schema.sql -v    # with graph and semantic details
```

### `synthgraph version`

```bash
synthgraph version
# → synthgraph version 1.0.0
```

---

## Further Reading

| Document | What it covers |
|----------|---------------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Full pipeline breakdown, design decisions, file layout |
| [docs/cli_reference.md](docs/cli_reference.md) | Complete CLI reference with all flags and examples |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Building from source, running tests, dev workflow |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | How to contribute, commit conventions, PR workflow |
| [docs/Future-Plan.md](docs/Future-Plan.md) | Upcoming features, roadmap ideas |
| [docs/DESIGN.md](docs/DESIGN.md) | Design philosophy, trade-offs, why certain choices were made |
| [docs/graph_model.md](docs/graph_model.md) | How the dependency graph works internally |
| [docs/constraint_system.md](docs/constraint_system.md) | How constraints are tracked and enforced |

---

## License

MIT — see [LICENSE](LICENSE).
