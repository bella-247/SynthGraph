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

## Quick start

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| **Go** | 1.21+ | Language runtime and compiler |
| **GCC / MinGW-w64** | any | Required by CGO for `pg_query_go` (PostgreSQL parser) |
| **Make** | 4+ | Build/dev automation (Linux/macOS, or Git Bash on Windows) |

> **Why `CGO_ENABLED=1`?** SynthGraph embeds PostgreSQL's own C parser (`pg_query_go`) to read SQL reliably. CGO bridges Go and C — it's required. The `Makefile` and `dev.ps1` set it for you, so you never type it.

> **Windows users:** Use `.\dev.ps1` instead of `make` — same commands, documented below.

### Run the web app (easiest)

```bash
# Linux / macOS
make run-web
# → http://localhost:8080

# Windows (PowerShell)
.\dev.ps1 run web
```

Then in your browser:
1. **Schema** — paste your SQL or pick the E-Commerce template
2. **Graph** — see tables and relationships as an interactive diagram
3. **Generate** — set row count, click Generate
4. **Download** — get your seed file as SQL or CSV
5. **History** — re-download past jobs

### Run the CLI

```bash
# Linux / macOS
make run-cli                                    # default schema, 10 rows
make run-cli SCHEMA=my.sql ROWS=100             # custom schema + row count

# Windows (PowerShell)
.\dev.ps1 run cli
.\dev.ps1 run cli my.sql
```

---

## Commands

All common operations go through `make` (Linux/macOS) or `dev.ps1` (Windows). No more remembering flags, paths, or `CGO_ENABLED`.

| Task | Linux/macOS | Windows |
|------|-------------|---------|
| Build CLI | `make build-cli` | `.\dev.ps1 build` |
| Build web | `make build-web` | `.\dev.ps1 build` |
| Build all | `make build` / `make build-all` | `.\dev.ps1 build` |
| Run web (port 8080) | `make run-web` | `.\dev.ps1 run web` |
| Run web (custom port) | `make run-web PORT=9090` | `.\dev.ps1 run web 9090` |
| Run CLI (default) | `make run-cli` | `.\dev.ps1 run cli` |
| Run CLI (custom) | `make run-cli SCHEMA=my.sql ROWS=100` | `.\dev.ps1 run cli my.sql` |
| Test all | `make test` | `.\dev.ps1 test all` |
| Test (fast, no CGO) | `make test-quick` | `.\dev.ps1 test quick` |
| Lint | `make lint` | `.\dev.ps1 lint` |
| Clean | `make clean` | `.\dev.ps1 clean` |

Binaries are written to `bin/` (e.g. `bin/synthgraph`, `bin/synthgraph-web`).

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

## Example walkthrough

**Input** (`testdata/schemas/ecommerce.sql`):
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
## License

MIT — see [LICENSE](LICENSE).

