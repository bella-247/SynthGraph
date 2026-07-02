# SynthGraph — Graph Model

This document explains the graph theory foundations of SynthGraph in detail. It is intended for contributors who want to understand or extend the graph engine.

---

## Parser Independence

SynthGraph parses PostgreSQL DDL using `pg_query_go` (PostgreSQL's own parser). The AST is translated once into a unified `schema.Model`. From that point forward, every stage in SynthGraph — graph building, constraint planning, generation, validation, export — operates purely on the schema model and has zero knowledge of the original SQL format.

This separation of concerns means:
- The graph engine is dialect-agnostic
- Adding MySQL or Prisma support is a localized translator, not a system redesign
- The core IP (constraint-aware generation, cycle resolution) is reusable across all dialects

---

## Why a Graph?

A relational schema is not a flat list of tables. It is a network of dependencies. Table B cannot have data inserted before Table A if B has a foreign key referencing A. This ordering problem is a classic application of directed graph theory.

SynthGraph models the schema as a **Directed Graph** where:

- Every **table** is a **node**
- Every **foreign key relationship** is a **directed edge** pointing from the dependent table to the table it depends on

This framing means edges point in the direction of "I need this to exist before me."

---

## Graph Construction

Given this schema:

```sql
CREATE TABLE users     (id INT PRIMARY KEY);
CREATE TABLE orders    (id INT PRIMARY KEY, user_id INT REFERENCES users(id));
CREATE TABLE payments  (id INT PRIMARY KEY, order_id INT REFERENCES orders(id));
```

The graph is:

```
payments ──► orders ──► users
```

`payments` depends on `orders`. `orders` depends on `users`. So the correct generation order (reversed graph traversal direction) is:

```
1. users
2. orders
3. payments
```

---

## Topological Sort — Kahn's Algorithm

Topological sort is the algorithm that produces a linear ordering of nodes in a directed acyclic graph such that for every edge `(u → v)`, node `u` appears before `v`.

SynthGraph uses **Kahn's Algorithm** (BFS-based) rather than DFS-based topological sort for one important reason: Kahn's algorithm naturally identifies which nodes are part of cycles (those that never reach in-degree zero), making cycle detection a byproduct of the sort rather than a separate pass.

**Algorithm:**

```
1. Compute in-degree for every node
   (in-degree = number of tables this table depends on)

2. Add all nodes with in-degree 0 to a queue
   (these are tables with no dependencies — safe to generate first)

3. While queue is not empty:
   a. Pop node N
   b. Append N to the result ordering
   c. For each table D that depends on N:
      - Decrement D's in-degree by 1
      - If D's in-degree is now 0, add D to queue

4. If result ordering length < total node count:
   - Nodes not in result are in cycles
   - Invoke cycle resolution
```

**Complexity:** O(V + E) where V = tables, E = foreign key relationships.

---

## Cycle Detection — Tarjan's Algorithm

After topological sort, any nodes not included in the ordering are part of one or more cycles. SynthGraph uses **Tarjan's Strongly Connected Components (SCC) algorithm** to identify which nodes belong to which cycle.

An SCC is a maximal set of nodes where every node is reachable from every other node. In a schema graph, an SCC with more than one node represents a circular foreign key dependency.

**Why Tarjan's?**

- Single DFS pass — O(V + E)
- Produces SCCs in reverse topological order of the condensation graph
- Clearly identifies which tables are involved in each cycle as a group

**Algorithm sketch:**

```
For each unvisited node N:
  DFS from N
  Assign discovery index and lowlink value
  Push N onto stack
  
  For each neighbor M of N:
    If M unvisited: recurse, update lowlink
    If M on stack: update lowlink with M's index
  
  If N's lowlink == N's index:
    N is the root of an SCC
    Pop nodes from stack until N is popped
    All popped nodes = one SCC
```

Each SCC with size > 1 is a cycle that needs resolution.

---

## Cycle Resolution

### The Core Problem

```sql
CREATE TABLE users (
    id              INT PRIMARY KEY,
    organization_id INT REFERENCES organizations(id)  -- NOT NULL
);

CREATE TABLE organizations (
    id       INT PRIMARY KEY,
    owner_id INT REFERENCES users(id)                 -- NOT NULL
);
```

`users` depends on `organizations`. `organizations` depends on `users`. Neither can be generated first. Classic cycle.

### The Solution: Nullable Deferred Insertion

SynthGraph resolves cycles by finding a "breakpoint" edge — a foreign key relationship within the cycle where the FK column is **nullable**.

**Steps:**

1. For the cycle `[users, organizations]`, examine all FK edges within the cycle
2. Find an edge where all FK source columns are nullable
3. That edge is the breakpoint — treat it as temporarily removed
4. Generate tables in the now-acyclic order with the breakpoint FK column set to `NULL`
5. After all tables in the cycle are generated, run `UPDATE` statements to backfill the deferred FK values

**Modified schema (resolvable):**

```sql
CREATE TABLE organizations (
    id       INT PRIMARY KEY,
    owner_id INT REFERENCES users(id)   -- nullable! This is the breakpoint
);
```

**Generated SQL output:**

```sql
-- Generate users first (with organization_id as NULL initially)
INSERT INTO users (id, organization_id) VALUES (1, NULL), (2, NULL), ...;

-- Generate organizations (can now reference users)
INSERT INTO organizations (id, owner_id) VALUES (1, 3), (2, 7), ...;

-- Backfill the deferred FK
UPDATE users SET organization_id = 2 WHERE id = 1;
UPDATE users SET organization_id = 1 WHERE id = 2;
```

### Unresolvable Cycles

If every FK edge in a cycle is NOT NULL, SynthGraph cannot resolve it without violating constraints. It fails with a clear error:

```
Error: Unresolvable circular dependency detected.

Cycle: users → organizations → users

  users.organization_id    NOT NULL → references organizations.id
  organizations.owner_id   NOT NULL → references users.id

None of the foreign key columns in this cycle are nullable.
SynthGraph cannot generate data for this cycle.

To resolve, make at least one FK column nullable:
  ALTER TABLE organizations ALTER COLUMN owner_id DROP NOT NULL;
```

---

## Self-Referencing Tables

```sql
CREATE TABLE categories (
    id        INT PRIMARY KEY,
    parent_id INT REFERENCES categories(id)  -- self-reference
);
```

A self-referencing table is a single-node SCC — a cycle of length 1. Resolution:

1. Generate root rows with `parent_id = NULL`
2. Generate child rows with `parent_id` pointing to root rows
3. Requires `parent_id` to be nullable — error if NOT NULL

---

## Graph Inspection Utilities

The graph module exposes these utilities for the `inspect` command and future visualization:

```go
// All tables with no dependencies (safe to generate first)
func (g *SchemaGraph) Roots() []*Node

// All tables that nothing depends on (leaf tables)
func (g *SchemaGraph) Leaves() []*Node

// Longest dependency chain in the graph
func (g *SchemaGraph) LongestChain() []*Node

// Tables with no edges at all (fully isolated)
func (g *SchemaGraph) Isolated() []*Node

// Direct dependencies of a table
func (g *SchemaGraph) Dependencies(tableName string) []*Node

// Tables that directly depend on a given table
func (g *SchemaGraph) Dependents(tableName string) []*Node

// All transitive dependencies (full subtree)
func (g *SchemaGraph) TransitiveDependencies(tableName string) []*Node
```

---

## Graph Complexity Classes

| Schema Type | Characteristics | SynthGraph handling |
|---|---|---|
| Star schema | One central table, many dependents | Simple topological sort |
| Linear chain | A → B → C → D | Simple topological sort |
| Diamond | A → B, A → C, B → D, C → D | Topological sort handles naturally |
| Self-reference | A → A | Single-node SCC, nullable required |
| Two-node cycle | A → B → A | SCC, nullable breakpoint required |
| Multi-node cycle | A → B → C → A | SCC, nullable breakpoint required |
| Nested cycles | Multiple SCCs with edges between them | Each SCC resolved independently |
| Isolated tables | No edges | Generated first in any order |
