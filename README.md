# SynthGraph

**Turn your SQL schema into realistic test data — automatically.**

SynthGraph reads your database schema, figures out how tables relate to each other, and generates sample data that respects every rule: foreign keys match up, emails are unique, enum values are valid, and nothing breaks.

```sql
-- Given this schema
CREATE TABLE users   (id INT PRIMARY KEY, email VARCHAR(255) UNIQUE);
CREATE TABLE orders  (id INT PRIMARY KEY, user_id INT REFERENCES users(id));

-- SynthGraph generates data where every order.user_id
-- actually exists in users.id — guaranteed.
```

No manual seed files. No foreign key violations. No frustration.

---

## Two ways to use it

### 1. Web app (easiest — recommended for beginners)

A full visual tool that takes you from schema to download in 5 steps.

```bash
# macOS / Linux (bash/zsh):
CGO_ENABLED=1 go run ./cmd/synthgraph-web/

# Windows (PowerShell):
$env:CGO_ENABLED='1'; go run ./cmd/synthgraph-web/
```

> **Why `CGO_ENABLED=1`?** SynthGraph embeds PostgreSQL's own C parser (`pg_query_go`) to read SQL reliably. CGO bridges Go and C — it's required.

Open **http://localhost:8080** and follow the pipeline:

| Step | What happens |
|------|-------------|
| **Schema** | Paste your SQL or pick a template to start |
| **Graph** | See your schema as an interactive node diagram |
| **Generate** | Set row count and format, click Generate |
| **Download** | Get your seed file as SQL or CSV |
| **History** | Browse and re-download past jobs |

> **Tip:** Don't have a schema handy? Pick the "E-Commerce" template to see how it works.

### 2. CLI (for scripts, CI, and power users)

Same engine, but from the terminal. Perfect for automation.

```bash
synthgraph generate --input schema.sql --rows 100 --output seed.sql
```

See the [CLI Reference](docs/cli_reference.md) for all commands and flags.

---

## What makes SynthGraph different?

Other generators create values in isolation — they don't know that `orders.user_id` must match a real user. SynthGraph treats your schema as a **dependency graph**:

1. **Reads your schema** — understands tables, columns, and every constraint
2. **Builds a dependency map** — figures out which tables need to exist before others
3. **Generates in the right order** — parents before children, referenced before referencing
4. **Resolves circular dependencies** — handles tricky cases like `users → organizations → users` automatically

The result: data that **just works**, every time.

### Constraint support

| Constraint | Handled? |
|-----------|----------|
| Primary keys (single + composite) | ✅ |
| Foreign keys (single + composite) | ✅ |
| Unique constraints | ✅ |
| NOT NULL | ✅ |
| Enum values | ✅ |
| VARCHAR / DECIMAL length | ✅ |
| Circular FK dependencies | ✅ |

---

## Quick start: Web app

```bash
# 1. Make sure Go is installed (1.21+)
go version

# 2. Clone and start the web app
git clone https://github.com/bella-247/SynthGraph.git
cd SynthGraph

# macOS/Linux:
CGO_ENABLED=1 go run ./cmd/synthgraph-web/
# Windows PowerShell:
# $env:CGO_ENABLED='1'; go run ./cmd/synthgraph-web/

# 3. Open http://localhost:8080
```

Then:
1. On the **Schema** page, pick a template or paste your SQL
2. Click "Parse Schema" — the app analyzes your tables
3. Go to **Graph** to see relationships visually
4. Go to **Generate**, set rows (try 10), click Generate
5. Download your seed file

---

## Quick start: CLI

```bash
# Build the CLI (macOS/Linux):
CGO_ENABLED=1 go build -o synthgraph.exe ./cmd/synthgraph/
# Windows PowerShell:
# $env:CGO_ENABLED='1'; go build -o synthgraph.exe ./cmd/synthgraph/

# Generate 50 rows per table
./synthgraph.exe generate --input testdata/schemas/ecommerce.sql --rows 50

# Output to a file
./synthgraph.exe generate --input testdata/schemas/ecommerce.sql --rows 50 --output seed.sql

# See what your schema looks like
./synthgraph.exe inspect --input testdata/schemas/ecommerce.sql --graph --semantic
```

---

## Example walkthrough

**Input** (`ecommerce.sql`):
```sql
CREATE TABLE users (
    id         SERIAL PRIMARY KEY,
    email      VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE products (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL
);

CREATE TABLE orders (
    id      SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    total   DECIMAL(10,2) NOT NULL,
    status  VARCHAR(50) DEFAULT 'pending'
);
```

**Output** (`seed.sql`):
```sql
BEGIN;
INSERT INTO users (id, email, first_name, created_at) VALUES
  (1, 'alice@example.com', 'Alice', '2024-03-15 09:22:11'), ...
INSERT INTO products (id, name, price) VALUES
  (1, 'Wireless Headphones', 89.99), ...
INSERT INTO orders (id, user_id, total, status) VALUES
  (1, 23, 142.50, 'pending'), ...
COMMIT;
```

Every FK value exists. Every email is unique. The file runs clean, first try.

---

## Architecture in one sentence

```
SQL → Parser → Graph → Planner → Generator → Validator → SQL/CSV
```

Each stage is a pure function that produces one artifact and feeds it to the next. Full details in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for building, testing, and extending.

## Contributing

Contributions welcome — see [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). Good first issues: new semantic field generators, test coverage, better error messages.

## License

MIT — see [LICENSE](LICENSE).
