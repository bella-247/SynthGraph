# SynthGraph — Constraint System

> **Who this is for:** Developers and advanced users who want to understand exactly how SynthGraph handles each type of database constraint (primary keys, foreign keys, unique, enum, etc.).
>
> **Plain English summary:** SynthGraph never generates data that breaks your schema's rules. This document walks through every constraint type and explains how the generator satisfies it — and what happens when it can't.

This document describes how SynthGraph handles each type of database constraint during generation and validation.

---

## Philosophy

SynthGraph treats constraint satisfaction as a **hard requirement**, not a best-effort goal.

Every generated dataset must pass the post-generation validator. If any constraint is violated, the output is discarded entirely and a clear error is returned. There is no "mostly valid" output.

The generation engine and the validation engine are **intentionally separate**. The generator tries to satisfy constraints during generation. The validator independently verifies the result. If the validator ever catches a violation on data produced by the generator, that is a bug — not a user error.

---

## Primary Key Constraints

### Single-Column PK

- **SERIAL / BIGSERIAL / AUTO_INCREMENT:** Values are generated as sequential integers starting at 1. Row 1 gets PK = 1, Row 2 gets PK = 2, etc. No randomness.
- **UUID PK:** Each row gets a newly generated RFC 4122 v4 UUID. Collision probability is negligible but the generator maintains a set to guarantee uniqueness.
- **Integer PK (non-serial):** Sequential integers starting at 1.

### Composite PK

The combination of all PK column values must be unique across all rows. The generator maintains a set of generated PK tuples and retries if a collision occurs.

---

## Foreign Key Constraints

### Selection Strategy

FK values are selected from the **pool of already-generated PK values** in the referenced table. Because topological sort ensures the referenced table is always generated before the referencing table, this pool is always non-empty when an FK is being generated.

```
Pool for orders.user_id = {1, 2, 3, ..., N}  (all generated user IDs)
Value selected: random element from pool (uniform distribution in V1)
```

### Nullable FK Columns

In V1, nullable FK columns always receive a non-null value unless they are part of a deferred cycle resolution. Future versions will allow configuring the null probability.

### Deferred FK Columns (Cycle Resolution)

FK columns identified as cycle breakpoints are set to `NULL` during initial insertion. They are backfilled via `UPDATE` statements after all tables in the cycle are generated. See `docs/graph_model.md` for full cycle resolution documentation.

### Composite FK Constraints

When a FK spans multiple columns (composite FK), all columns in the FK must together reference the same row in the target table. The generator selects a complete tuple from the set of generated rows in the target table and uses all column values from that tuple.

---

## Unique Constraints

### Single-Column Unique

The generator maintains a per-table, per-column `set` of generated values. Before finalizing a value for a unique column, it checks the set. On collision, it retries with a new value.

**Maximum retries:** 100. If a unique value cannot be found after 100 attempts, SynthGraph fails with an error explaining the constraint and suggesting a higher cardinality generator or fewer rows.

```
Error: Cannot satisfy UNIQUE constraint on users.email after 100 retries.

  Table:   users
  Column:  email
  Rows requested: 50000

  The email generator may not have enough cardinality for this row count.
  Try using --rows with a smaller value, or report this as a limitation.
```

### Composite Unique Constraints

The generator maintains a set of generated value tuples. The full tuple (all columns in the constraint) must be unique. Retries work at the tuple level — the entire group of columns is regenerated on collision.

---

## NOT NULL Constraints

Columns declared NOT NULL never receive a NULL value from any generator. The `Nullable` field on the `schema.Column` struct controls this:

```go
if !col.Nullable {
    // generator must return a non-null value
    // if it returns nil, retry or use type-based fallback
}
```

---

## Enum Constraints

Enum columns only receive values declared in their enum definition. The generator selects uniformly at random from the declared values.

For inline enums:
```sql
status VARCHAR(50) CHECK (status IN ('active', 'inactive', 'pending'))
```

For PostgreSQL-style named enums:
```sql
CREATE TYPE order_status AS ENUM ('draft', 'submitted', 'shipped', 'delivered');
```

Both are parsed into `Column.EnumValues []string` and handled identically by the generator.

---

## Length Constraints

### VARCHAR(n)

The generator checks `col.MaxLength` and ensures the generated string does not exceed `n` characters. Name-based generators that might produce long strings (e.g., full addresses) are capped at MaxLength.

### CHAR(n)

Generated strings are padded or trimmed to exactly `n` characters.

### DECIMAL(p, s)

Generated values have at most `p` total digits and exactly `s` decimal places. Example: `DECIMAL(8, 2)` → values like `12345.67`, maximum `999999.99`.

---

## CHECK Constraints

### V1 Status

CHECK constraints are **parsed and stored** in the Internal Schema Model but are **not enforced** during generation in V1.

When a CHECK constraint is detected, SynthGraph prints a warning before generation begins:

```
Warning: CHECK constraint on "orders.amount" will not be enforced (V1 limitation).
  Expression: amount > 0
  Data may be generated that violates this constraint.
```

### V2 Plan

V2 will implement a simple expression evaluator for common CHECK constraint patterns:

- Numeric range: `col > 0`, `col BETWEEN 0 AND 100`
- String pattern: `col LIKE 'prefix%'`
- Enum-style: `col IN ('a', 'b', 'c')`
- Cross-column: `end_date > start_date`

Complex expressions will remain unenforced with a warning.

---

## Constraint Validation

After generation, before export, the validator re-checks every constraint independently. This is the final safety net.

### Validation Process

```
For each table T in the dataset:
  1. Build index of PK values for T (for FK lookups)
  2. Check PK uniqueness across all rows
  3. Check unique constraint uniqueness
  4. For each row R:
     a. For each NOT NULL column: assert R[col] != NULL
     b. For each FK column: assert R[col] exists in referenced table's PK index
     c. For each enum column: assert R[col] in declared enum values
     d. For each VARCHAR(n) column: assert len(R[col]) <= n
```

### Validation Failure

If any validation check fails:

1. All generated data is discarded
2. A structured error describing the exact violation is printed to stderr
3. Process exits with code 5

If the violation is on data produced internally (not from user input), the error includes a bug report prompt since this represents a generator bug.

---

## Constraint Priority During Generation

When multiple constraints apply to the same column, they are applied in this order:

1. **Type** — determines value domain
2. **Name** — selects semantic generator (overrides type-only generation)
3. **Serial/PK** — overrides name generator for serial PKs
4. **FK** — overrides everything; value must come from referenced pool
5. **Unique** — post-generation check with retry
6. **NOT NULL** — post-generation check; retry if NULL was produced
7. **MaxLength** — applied as cap inside generator

FK constraint always takes highest priority for FK columns. No name-based generator runs on an FK column that has a valid reference pool — the value comes from the pool.
