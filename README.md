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

No manual seed files. No foreign key violations. No frustration.

---

## Quick Install

### Linux / macOS (one line)

```bash
curl -sSf https://raw.githubusercontent.com/bella-247/SynthGraph/main/scripts/install.sh | sh
```

### Windows (one line)

```powershell
irm https://raw.githubusercontent.com/bella-247/SynthGraph/main/scripts/install.ps1 | iex
```

### Go users (build from source)

```bash
CGO_ENABLED=1 go install github.com/bella-247/SynthGraph/cmd/synthgraph@latest
```

> **What is CGO?** SynthGraph uses the real PostgreSQL parser to read SQL schemas. That parser is written in C, so Go needs CGO to talk to it. On Windows, you'll need [MinGW-w64](https://www.msys2.org/) (GCC for Windows). On macOS, run `xcode-select --install`. On Linux, `sudo apt install gcc libpq-dev`.

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

That's it. You now have 10 users, 10 products, and 10 orders — all with valid foreign keys.

### Try different options

```bash
synthgraph generate -i shop.sql -r 100     # 100 rows per table
synthgraph generate -i shop.sql -f csv      # CSV instead of SQL
synthgraph generate -i shop.sql -s 12345    # fixed seed (repeatable output)
```

---

## Web App (graphical interface)

SynthGraph also ships with a browser UI that shows your schema as an interactive diagram:

```bash
synthgraph-web
# → Open http://localhost:8080
```

The web app walks you through 4 steps:

| Step | What you do | What you see |
|------|------------|--------------|
| **Schema** | Paste SQL or pick a template | Parsed tables, columns, types |
| **Graph** | Look at the diagram | Tables as boxes, FK relationships as arrows |
| **Generate** | Set row count, click Generate | Live progress as data is created |
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

**Examples:**

```bash
synthgraph generate -i schema.sql                          # 10 rows, SQL output, to terminal
synthgraph generate -i schema.sql -o data.sql              # save to file
synthgraph generate -i schema.sql -r 1000 -f csv -o data.csv  # 1000 rows, CSV
```

### `synthgraph inspect`

Analyze a schema and print its structure.

```bash
synthgraph inspect -i schema.sql                           # tables, columns, enums
synthgraph inspect -i schema.sql -v                        # + graph + semantic details
```

### `synthgraph version`

```bash
synthgraph version
# → synthgraph version 0.1.0
```

---

## What makes SynthGraph different?

Other generators create random values in isolation — they don't know that `orders.user_id` must match a real user. SynthGraph reads your **entire schema**, builds a dependency graph, and generates in the right order:

```sql
CREATE TABLE users   (id INT PRIMARY KEY);
CREATE TABLE orders  (id INT PRIMARY KEY, user_id INT REFERENCES users(id));

-- Other generators:
-- orders.user_id might be 9999 — no user with that ID. Violation. ❌

-- SynthGraph:
-- Generates users FIRST, then orders referencing those users. Guaranteed valid. ✅
```

### Supported constraints

| Constraint | Supported? |
|-----------|:----------:|
| Primary keys (single + composite) | ✅ |
| Foreign keys (single + composite) | ✅ |
| Unique constraints | ✅ |
| NOT NULL | ✅ |
| Enum types (`CREATE TYPE ... AS ENUM`) | ✅ |
| VARCHAR / DECIMAL length | ✅ |
| Circular FK dependencies (e.g. A → B → A) | ✅ |
| `DEFAULT` expressions | ✅ |

---

## Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for building from source, running tests, and extending SynthGraph.

```bash
# Quick test (no CGO needed — skips parser tests)
make test-quick          # Linux/macOS
.\dev.ps1 test quick     # Windows

# Full test suite (requires CGO)
make test                # Linux/macOS
.\dev.ps1 test all       # Windows
```

## Architecture

```
SQL → Parser → Graph → Planner → Generator → Validator → SQL / CSV
```

Each stage is a pure function. Details in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## License

MIT — see [LICENSE](LICENSE).
