# SynthGraph — Software Requirements Specification (SRS)

**Version:** 1.0.0  
**Status:** Pre-Implementation Draft  
**Language:** Go  
**Document Type:** Technical Specification  

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Architectural Philosophy](#2-architectural-philosophy)
3. [Core Pipeline Architecture](#3-core-pipeline-architecture)
4. [Supported Input Formats](#4-supported-input-formats)
5. [Internal Schema Representation Model](#5-internal-schema-representation-model)
6. [Graph Construction Rules](#6-graph-construction-rules)
7. [Dependency Resolution Strategy](#7-dependency-resolution-strategy)
8. [Constraint System](#8-constraint-system)
9. [Data Generation Rules](#9-data-generation-rules)
10. [Translator Validator](#10-translator-validator-schema-validation)
11. [Post-generation Validator](#11-post-generation-validator-dataset-validation)
12. [Output Formats](#12-output-formats)
13. [Error Handling Strategy](#13-error-handling-strategy)
14. [CLI Specification](#14-cli-specification)
15. [Non-Functional Requirements](#15-non-functional-requirements)
16. [V1 Scope Boundaries](#16-v1-scope-boundaries)

---

## 1. System Overview

### 1.1 What SynthGraph Is

SynthGraph is a standalone command-line developer tool that reads a relational database schema, models it as a directed graph, and generates a mathematically valid, constraint-compliant synthetic dataset.

SynthGraph is **not** a library. It is **not** a wrapper around Faker. It is a system that understands the structural relationships within a schema and produces data that respects every constraint — primary keys, foreign keys, unique constraints, nullability, enums, and more — without a single violation.

### 1.2 The Core Problem It Solves

Existing synthetic data tools (Faker, factory_boy, Fishery, etc.) generate values in isolation. They do not understand that:

- `orders.user_id` must reference a value that actually exists in `users.id`
- `users.email` must be unique across all generated rows
- `payments` cannot be generated before `orders` because it depends on them
- Circular foreign key references (`users → organizations → users`) require special handling

SynthGraph solves this by treating the schema as a **Directed Acyclic Graph (DAG)**, resolving the correct generation order via topological sort, and generating every field in a constraint-aware manner.

### 1.3 Primary Use Cases

- Backend engineers seeding local development databases
- QA engineers generating test fixtures for integration tests
- Database engineers validating schema correctness
- Teams replacing production data snapshots with safe synthetic alternatives

### 1.4 Design Mandate

> **Correctness first. Realism second.**

A perfectly valid relational dataset that looks boring is infinitely more useful than realistic-looking data that violates a foreign key constraint and crashes a test suite.

---

## 2. Architectural Philosophy

### 2.1 Core Principles

**Parser Agnosticism**  
The core engine must never be aware of where the schema came from. SQL, Prisma, Drizzle — these are all just input formats. Every parser must transform its input into the same unified Internal Schema Model. The graph engine, generator, validator, and exporter must only ever operate on that internal model.

**Stage Isolation**  
Each pipeline stage is a distinct, independently testable unit. No stage may import logic from a non-adjacent stage. Parsing does not generate. Generation does not validate. Validation does not export.

**Determinism**  
Given the same schema, the same row count, and the same seed value, SynthGraph must produce identical output every time. This is essential for reproducible test environments.

**Fail Loudly**  
SynthGraph never produces partial or potentially invalid output. If a constraint cannot be satisfied or a cycle cannot be resolved, the process terminates with a clear, actionable error message. Silent failures are architectural violations.

**Extensibility Without Over-Engineering**  
V1 must be extensible by design — parser interface, exporter interface, generator registry — but must not build extensibility features that are not yet needed. Interfaces are defined; unnecessary abstractions are deferred.

### 2.2 What SynthGraph Is Not

- Not a database client or migration tool
- Not a production data anonymizer
- Not an ORM or query builder
- Not a statistical simulation engine
- Not a library to be imported into application code

---

## 3. Core Pipeline Architecture

The system is a strict linear pipeline. Data flows in one direction only. No stage may communicate laterally or backward. Every stage consumes one typed artifact and produces another.

```
┌─────────────────────────────────────────────────────────────────┐
│                        SynthGraph Pipeline                       │
│                                                                   │
│  [Schema File]                                                    │
│       │                                                           │
│       ▼                                                           │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  PARSER (dialect-specific)  Internal pipeline:             │  │
│  │                                                             │  │
│  │    AST ← pg_query_go                                        │  │
│  │      ↓                                                      │  │
│  │    Extractor → Raw Table State                              │  │
│  │      ↓                                                      │  │
│  │    Normalizer → Canonical Types                             │  │
│  │      ↓                                                      │  │
│  │    Reference Linker → Resolved References                   │  │
│  │      ↓                                                      │  │
│  │    Validator → Consistency Check                            │  │
│  │      ↓                                                      │  │
│  │    Builder → schema.Model                                   │  │
│  └────────────────────────────────────────────────────────────┘  │
│       │                                                           │
│       ▼                                                           │
│  ┌───────────────┐                                                │
│  │ GRAPH BUILDER │  schema.Model → SchemaGraph                   │
│  └───────────────┘                                                │
│       │                                                           │
│       ▼                                                           │
│  ┌──────────────────┐                                             │
│  │   PLANNER        │  SchemaGraph → GenerationPlan              │
│  │   (topological    │  Detects cycles, resolves ordering,       │
│  │    sort + cycle   │  plans deferred FKs                       │
│  │    detection)    │                                             │
│  └──────────────────┘                                             │
│       │                                                           │
│       ▼                                                           │
│  ┌───────────────┐                                                │
│  │   GENERATOR   │  GenerationPlan → Dataset                     │
│  │                │  Produces rows respecting all constraints     │
│  └───────────────┘                                                │
│       │                                                           │
│       ▼                                                           │
│  ┌──────────────────┐                                             │
│  │ POST-GENERATION  │  Dataset → Validated Dataset               │
│  │ VALIDATOR        │  Verifies entire dataset before export     │
│  └──────────────────┘                                             │
│       │                                                           │
│       ▼                                                           │
│  ┌───────────────┐                                                │
│  │   EXPORTER    │  Validated Dataset → SQL / CSV                │
│  └───────────────┘                                                │
└─────────────────────────────────────────────────────────────────┘
```

### 3.1 Stage Responsibilities

| Stage | Input | Output | Responsibility |
|---|---|---|---|
| Parser | Raw schema file | `schema.Model` | Transform format-specific schema into unified model. Internally: Extract → Normalize → Link → Validate → Build |
| Graph Builder | `schema.Model` | `SchemaGraph` | Build directed graph of table dependencies |
| Planner | `SchemaGraph` | `GenerationPlan` | Topological sort, detect cycles, plan deferred FKs |
| Generator | `GenerationPlan` | `Dataset` | Produce constraint-aware values for every field |
| Post-generation Validator | `Dataset` + `schema.Model` | `ValidatedDataset` or error | Verify all constraints are satisfied |
| Exporter | `ValidatedDataset` | Output File | Write SQL INSERT statements or CSV |

---

## 4. Supported Input Formats

### 4.1 V1: PostgreSQL SQL Schema (DDL)

SynthGraph v1 supports PostgreSQL SQL Data Definition Language (DDL) files via `pg_query_go`, a native Go wrapper around PostgreSQL's own C parser. This ensures rock-solid compatibility with PostgreSQL DDL without the complexity of a hand-written parser.

**Parser Implementation:**
- Uses `pg_query_go` (github.com/pganalyze/pg_query_go) for AST generation
- PostgreSQL parser is embedded — no runtime dependency on PostgreSQL installation
- Supports all PostgreSQL DDL constructs that PostgreSQL 12+ supports natively
- Five-stage internal translation pipeline: Extract → Normalize → Link → Validate → Build
- The parser's public interface is a single `Parse()` call; the internal pipeline is an implementation detail

**Supported DDL constructs in V1:**
All PostgreSQL CREATE TABLE syntax, including:
- Column definitions with all PostgreSQL-supported data types
- Primary key constraints (single and composite)
- Foreign key constraints with ON DELETE/UPDATE actions
- Unique constraints (single and composite)
- NOT NULL and DEFAULT clauses
- CHECK constraints (parsed, not enforced in v1)
- PostgreSQL-specific features: SERIAL/BIGSERIAL, GENERATED, ARRAY types, named ENUM types

Since the parser is PostgreSQL's own, if PostgreSQL parses it, SynthGraph v1 parses it.

**Why PostgreSQL First?**
- PostgreSQL's parser is battle-tested on millions of queries and handles every edge case (quoted identifiers, comments, complex expressions)
- Building a hand-written parser would delay v1 by 4-8 weeks and introduce maintenance burden
- SynthGraph's value is not in parsing — it's in graph-based dependency resolution, constraint planning, and deterministic generation
- Future support for MySQL, SQLite, and Prisma is trivial once the translator pattern is proven

**Data Types Supported:**
All PostgreSQL data types are supported. SynthGraph maps them internally to abstract types (INT, BIGINT, TEXT, DECIMAL, BOOLEAN, DATE, TIMESTAMP, UUID, JSON, ENUM).

### 4.2 Parser Interface Contract

Every parser, regardless of format, must implement the following Go interface:

```go
type SchemaParser interface {
    // Parse reads a schema source file and returns the unified internal schema model.
    // The parser handles all dialect-specific AST transformations internally.
    Parse(source []byte) (*schema.Model, error)

    // Name returns the parser identifier (e.g., "postgresql", "mysql", "prisma")
    Name() string

    // SupportedExtensions returns file extensions this parser handles (e.g., [".sql"])
    SupportedExtensions() []string
}

// V1 Implementation: PostgreSQL Parser
// The PostgreSQL parser wraps pg_query_go's AST and translates to schema.Model.
// Future parsers (MySQL, Prisma) implement the same interface with their own translators.
// The rest of SynthGraph (graph builder, planner, generator, validator, exporter)
// only ever sees schema.Model and never needs to know the source format.
```

The PostgreSQL parser in v1 is the only implementation of this interface. The interface is defined now so that future parsers (MySQL, Prisma, Drizzle) can be added without touching the pipeline.

### 4.3 Translator Internal Pipeline (within the Parser)

Treating the translator as a compiler sub-pipeline maintains stage isolation even within the parser. Each sub-stage transforms the intermediate state and has no knowledge of adjacent stages' internals.

**Stage 1 — Extractor**
Consumes the PostgreSQL AST (from `pg_query_go`) and produces raw table and enum state. Preserves column definitions exactly as parsed. Assembles constraint lists from both column-level and table-level definitions. Pure 1:1 copy — no normalization, no resolution.

**Stage 2 — Normalizer**
Consumes raw table state and produces canonical column definitions. Maps PostgreSQL type names to SynthGraph abstract types (e.g., `INT4`/`INTEGER`/`INT` → `int`, `CHARACTER VARYING` → `varchar`). Resolves `SERIAL` variants to concrete integer types. Sets nullability from `NOT NULL`. Stores `DEFAULT` expressions as raw strings. No cross-references are resolved here.

**Stage 3 — Reference Linker**
Consumes canonical column definitions and resolves cross-references. Maps foreign key target table name strings to actual table indices within the schema. Connects enum-type columns to their `CREATE TYPE` definitions. Unresolvable references are marked (not yet an error — validation rejects them).

**Stage 4 — Translator Validator**
Consumes the linked intermediate state and validates schema consistency. Detects duplicate table names, duplicate column names within a table, primary key columns that do not exist, unique constraint columns that do not exist, and foreign key target columns that do not exist in the referenced table. Passing this stage means the schema is internally consistent. All violations produce specific, actionable error messages. This is **not** the Post-generation Validator — it validates the schema model itself, not the generated dataset.

**Stage 5 — Builder**
Consumes the validated intermediate state and produces the final immutable `schema.Model`. Deduplicates primary key columns (same column can appear in both inline and table-level constraint declarations). Marks PK columns on the `Column` struct. Removes unique constraints that are fully covered by the primary key. Returns the final `schema.Model` to the pipeline.

---

## 5. Internal Schema Representation Model

This is the most critical data structure in the entire system. Every stage after parsing operates exclusively on this model.

### 5.1 Top-Level Model

```go
// schema/model.go

package schema

// Model is the unified internal representation of any supported schema format.
// All parsers must produce this structure. All downstream stages consume it.
type Model struct {
    Tables  []*Table            // All tables, in declaration order
    TableMap map[string]*Table  // Fast lookup by table name
    Enums   []*EnumType         // Named enum types (PostgreSQL-style)
}
```

### 5.2 Table

```go
type Table struct {
    Name        string
    Columns     []*Column
    ColumnMap   map[string]*Column
    PrimaryKey  *PrimaryKey
    ForeignKeys []*ForeignKey
    UniqueConstraints []*UniqueConstraint
    CheckConstraints  []*CheckConstraint  // V1: parsed but not enforced in generator
}
```

### 5.3 Column

```go
type Column struct {
    Name         string
    DataType     DataType
    RawType      string      // original string from schema, e.g. "VARCHAR(255)"
    MaxLength    *int        // populated for VARCHAR(n), CHAR(n)
    Precision    *int        // populated for DECIMAL(p,s)
    Scale        *int
    Nullable     bool
    HasDefault   bool
    DefaultValue *string     // raw default expression as string
    IsSerial     bool        // true for SERIAL / BIGSERIAL / AUTO_INCREMENT
    EnumValues   []string    // populated if DataType == DataTypeEnum
    IsPrimaryKey bool        // true if this column is part of the PK
    IsForeignKey bool        // true if this column is an FK source
    IsUnique     bool        // true if single-column unique constraint exists
}
```

### 5.4 DataType Enum

```go
type DataType int

const (
    DataTypeUnknown DataType = iota
    DataTypeInt
    DataTypeBigInt
    DataTypeSmallInt
    DataTypeDecimal
    DataTypeFloat
    DataTypeText
    DataTypeVarchar
    DataTypeChar
    DataTypeBoolean
    DataTypeDate
    DataTypeTimestamp
    DataTypeTime
    DataTypeUUID
    DataTypeJSON
    DataTypeEnum
)
```

### 5.5 Constraints

```go
type PrimaryKey struct {
    Columns []string  // supports composite PKs
}

type ForeignKey struct {
    Name           string   // constraint name, may be empty
    SourceColumns  []string // columns in this table
    TargetTable    string   // referenced table name
    TargetColumns  []string // referenced columns
    OnDelete       FKAction
    OnUpdate       FKAction
}

type FKAction int
const (
    FKActionNoAction FKAction = iota
    FKActionCascade
    FKActionSetNull
    FKActionRestrict
    FKActionSetDefault
)

type UniqueConstraint struct {
    Name    string
    Columns []string
}

type CheckConstraint struct {
    Name       string
    Expression string  // raw SQL expression, stored for future enforcement
}

type EnumType struct {
    Name   string
    Values []string
}
```

---

## 6. Graph Construction Rules

### 6.1 Graph Definition

```go
// graph/graph.go

type Node struct {
    TableName string
    Table     *schema.Table
    InEdges   []*Edge   // tables that THIS table depends on (sources)
    OutEdges  []*Edge   // tables that depend on THIS table (dependents)
}

type Edge struct {
    From       *Node       // the table with the FK (dependent)
    To         *Node       // the table being referenced
    ForeignKey *schema.ForeignKey
    Nullable   bool        // true if all FK source columns are nullable
}

type SchemaGraph struct {
    Nodes   map[string]*Node
    Edges   []*Edge
}
```

### 6.2 Construction Rules

**Rule 1:** Every table in the Internal Schema Model becomes exactly one node.

**Rule 2:** Every foreign key relationship becomes exactly one directed edge, pointing FROM the referencing table TO the referenced table.

Example: `orders.user_id → users.id` becomes an edge from `orders` node to `users` node. This means "orders depends on users."

**Rule 3:** Self-referencing foreign keys (e.g., `employees.manager_id → employees.id`) become self-loop edges. These are treated as a special cycle case.

**Rule 4:** Every edge records whether all of its source FK columns are nullable. This is used by the cycle resolver to determine if deferred insertion is possible.

**Rule 5:** The graph must be fully constructed before topological sort is attempted. Partial construction is not acceptable.

### 6.3 Reverse Lookup

The graph must support reverse lookups:

```go
// Returns all tables that directly depend on the given table
func (g *SchemaGraph) Dependents(tableName string) []*Node

// Returns all tables that the given table directly depends on
func (g *SchemaGraph) Dependencies(tableName string) []*Node

// Returns full transitive dependency chain for a table
func (g *SchemaGraph) TransitiveDependencies(tableName string) []*Node
```

---

## 7. Dependency Resolution Strategy

### 7.1 Topological Sort

SynthGraph uses Kahn's Algorithm (BFS-based) for topological sorting rather than DFS-based sort, because Kahn's algorithm naturally exposes which nodes are responsible for cycles (those never reaching in-degree zero).

**Algorithm:**

```
1. Compute in-degree for every node (number of tables it depends on)
2. Initialize queue with all nodes of in-degree 0 (no dependencies)
3. While queue is not empty:
   a. Pop node N from queue
   b. Append N to generation order
   c. For each dependent D of N:
      - Decrement D's in-degree by 1
      - If D's in-degree reaches 0, add D to queue
4. If generation order length < total nodes:
   - Remaining nodes form one or more cycles
   - Invoke Cycle Resolution strategy
```

### 7.2 Cycle Detection

After topological sort, any node not included in the output is part of a cycle. SynthGraph uses Tarjan's Algorithm to identify all Strongly Connected Components (SCCs) among the remaining nodes. Each SCC is one cycle group.

### 7.3 Cycle Resolution Strategy

SynthGraph v1 uses **Nullable Deferred Insertion** as the cycle resolution strategy.

**Steps:**

1. For each detected cycle, identify the edge(s) within the cycle whose FK source columns are all nullable. This edge is the "breakpoint."
2. Generate all tables in the cycle with their breakpoint FK columns set to `NULL` initially.
3. After all tables in the cycle are generated, perform an `UPDATE` pass to backfill the nullable FK values.
4. If no nullable edge exists within a cycle, SynthGraph fails with an explicit error describing the unresolvable cycle and listing the tables involved.

**Output representation:**

For deferred cycles, the SQL exporter must output:
- `INSERT` statements with `NULL` for deferred FK columns
- `UPDATE` statements at the end of the file to backfill those values

**Error for unresolvable cycles:**

```
Error: Unresolvable circular dependency detected.

Cycle: users → organizations → users

None of the foreign key columns in this cycle are nullable.
SynthGraph cannot generate data for this cycle without a nullable breakpoint.

To resolve: make at least one FK column in the cycle nullable.
  Suggestion: ALTER TABLE organizations ALTER COLUMN owner_id DROP NOT NULL;
```

### 7.4 Self-Referencing Tables

Self-referencing tables (e.g., `categories.parent_id → categories.id`) are treated as a single-node cycle. The resolution is:

1. Generate root rows first (self-referencing FK set to `NULL`)
2. Generate child rows referencing the root rows
3. This requires the FK column to be nullable — if not, error with suggestion

---

## 8. Constraint System

### 8.1 Primary Key Constraints

- Every PK column must have a unique value across all generated rows for that table
- For `SERIAL` / `BIGSERIAL` / `AUTO_INCREMENT` columns: generate sequential integers starting at 1
- For `UUID` PK columns: generate RFC 4122 v4 UUIDs
- For composite PKs: the combination of all PK column values must be unique

### 8.2 Foreign Key Constraints

- FK column values must be drawn exclusively from the set of already-generated values in the referenced table's referenced column
- FK values are selected randomly from the pool of valid referenced values (uniform distribution in v1)
- If the referenced table has zero rows generated (should not happen after correct topological sort), this is a fatal internal error
- For nullable FKs: approximately 10% of values may be NULL (configurable in v2)

### 8.3 Unique Constraints

- Single-column unique constraints: maintain a per-table, per-column set of generated values; regenerate on collision
- Composite unique constraints: maintain a set of generated value tuples; regenerate on collision
- If a unique constraint cannot be satisfied after `maxRetries` attempts (default: 100), fail with error explaining the constraint and the cardinality problem

### 8.4 Not Null Constraints

- Columns marked NOT NULL must never receive a NULL value
- Columns without NOT NULL (nullable): in v1, always generate a non-null value unless the column is a deferred FK (see cycle resolution). This is conservative and correct.

### 8.5 Enum Constraints

- Enum columns must only contain values declared in the enum definition
- Values are selected randomly from the declared set (uniform distribution in v1)

### 8.6 Length Constraints

- `VARCHAR(n)` columns: generated string must not exceed `n` characters
- `CHAR(n)` columns: generated string must be exactly `n` characters
- `DECIMAL(p, s)` columns: generated value must have at most `p` total digits and `s` decimal places

### 8.7 Check Constraints

- V1: Check constraint expressions are parsed and stored in the Internal Schema Model
- V1: Check constraints are **not** enforced during generation (they are logged as a warning)
- V2: Check constraint evaluation will be implemented

---

## 9. Data Generation Rules

### 9.1 Generator Registry

The generator is driven by a registry that maps a (ColumnName, DataType) pair to a generator function. Name-based matching takes priority over type-based matching.

```go
type GeneratorFunc func(col *schema.Column, ctx *GenerationContext) (any, error)

type GeneratorRegistry struct {
    nameGenerators map[string]GeneratorFunc  // keyed by normalized column name
    typeGenerators map[schema.DataType]GeneratorFunc
}
```

### 9.2 Semantic Name-Based Generators

The following column name patterns trigger semantic generators. Matching is case-insensitive and uses suffix/exact matching.

| Column Name Pattern | Generated Value |
|---|---|
| `id` (exact, non-FK, non-serial) | Sequential integer or UUID depending on type |
| `*email` / `*email_address` | Valid email format `name@domain.tld` |
| `*first_name` / `*firstname` | Realistic first name |
| `*last_name` / `*lastname` / `*surname` | Realistic last name |
| `*full_name` / `*name` (on user-like tables) | Realistic full name |
| `*phone` / `*phone_number` | Valid phone number format |
| `*address` / `*street` | Street address |
| `*city` | City name |
| `*country` / `*country_code` | Country name or ISO 3166-1 alpha-2 code |
| `*zip` / `*postal_code` / `*postcode` | Postal code format |
| `*url` / `*website` / `*link` | Valid URL |
| `*price` / `*amount` / `*cost` / `*fee` | Positive decimal, 2 decimal places |
| `*quantity` / `*qty` / `*count` | Positive integer |
| `*latitude` / `*lat` | Float between -90.0 and 90.0 |
| `*longitude` / `*lng` / `*lon` | Float between -180.0 and 180.0 |
| `*description` / `*bio` / `*summary` | Lorem ipsum paragraph |
| `*title` / `*heading` | Short lorem ipsum phrase |
| `*status` (non-enum) | One of: `active`, `inactive`, `pending` |
| `*created_at` / `*updated_at` | Timestamp within the past 2 years |
| `*deleted_at` | NULL (90%) or timestamp (10%) |
| `*is_*` / `*has_*` / `*can_*` | Boolean, weighted 80% true |
| `*token` / `*api_key` / `*secret` | Random hex string, 32–64 chars |
| `*hash` / `*password_hash` | bcrypt-style hash placeholder string |
| `*ip` / `*ip_address` | Valid IPv4 address |
| `*uuid` (non-PK) | RFC 4122 v4 UUID |
| `*color` / `*colour` | Hex color string `#RRGGBB` |
| `*currency` / `*currency_code` | ISO 4217 code (USD, EUR, GBP, etc.) |
| `*rating` / `*score` | Integer between 1 and 5 |
| `*percentage` / `*percent` / `*rate` | Float between 0.0 and 100.0 |

### 9.3 Type-Based Fallback Generators

When no name-based generator matches, fall back to type-based generation:

| DataType | Generation Strategy |
|---|---|
| `DataTypeInt` | Random integer between 1 and 10,000 |
| `DataTypeBigInt` | Random integer between 1 and 1,000,000 |
| `DataTypeSmallInt` | Random integer between 1 and 1,000 |
| `DataTypeDecimal` / `DataTypeFloat` | Random float between 0.0 and 1,000.0 |
| `DataTypeText` | Random lorem ipsum sentence |
| `DataTypeVarchar` | Random alphanumeric string, capped at MaxLength |
| `DataTypeChar` | Random alphanumeric string, exactly MaxLength |
| `DataTypeBoolean` | Random true/false (50/50) |
| `DataTypeDate` | Random date within the past 3 years |
| `DataTypeTimestamp` | Random timestamp within the past 3 years |
| `DataTypeTime` | Random time string HH:MM:SS |
| `DataTypeUUID` | RFC 4122 v4 UUID |
| `DataTypeJSON` | Empty object `{}` |
| `DataTypeEnum` | Random value from declared enum values |

### 9.4 Generation Context

Every generator function receives a `GenerationContext` to allow contextually aware generation:

```go
type GenerationContext struct {
    TableName    string
    RowIndex     int              // 0-based row index being generated
    TotalRows    int              // total rows requested for this table
    GeneratedPKs map[string][]any // table → generated PK values (for FK selection)
    GlobalSeed   int64            // user-supplied global RNG seed
    TableSeed    int64            // per-table seed: hash(GlobalSeed, TableName)
    RNG          *rand.Rand       // seeded random source, unique per table
}
```

**TableSeed Derivation:**

Each table gets its own deterministic RNG stream derived from the global seed:

```
TableSeed = FNV-64a(GlobalSeed || "::" || TableName)
```

Where `||` denotes string concatenation. This ensures that identical seeds across different tables produce different values, while the same schema + seed always produces identical results.

**Why per-table RNG instead of a single sequential stream?**

1. **Deterministic generation** — Table generation order is an implementation detail that should not affect output values. With per-table RNG, changing generation order (e.g., due to planner improvements) does not change the values produced for any table.

2. **Future parallel generation** — Each table can be generated independently once its dependencies are available. Per-table RNG eliminates cross-table RNG state, making parallel generation trivially safe.

3. **Future caching** — If a table's schema and row count have not changed, its generated rows can be cached and reused without regenerating dependent tables (as long as FK pools remain valid).

4. **Independent table generation** — Adding or removing a table from the schema does not change the generated values for any other table that does not reference it. With a single shared RNG stream, inserting a table would shift the RNG state for every subsequent table.

### 9.5 Determinism

All random number generation must use a seeded `rand.Rand` instance derived from `TableSeed`. The global seed is either user-supplied via `--seed` flag or defaults to a fixed value (`42`) for reproducibility. The same schema, row count, seed, and configuration must always produce identical output.

---

## 10. Translator Validator (Schema Validation)

The Translator Validator runs as Stage 4 inside the parser's internal pipeline (see §4.3). It validates the *schema model itself* — not the generated dataset. It ensures the translated model is internally consistent before it reaches the graph engine.

### 10.1 Validation Checks

**Duplicate Table Names:** Two tables with the same name in the same schema are a fatal error.

**Duplicate Column Names:** Two columns with the same name within a single table are a fatal error.

**Primary Key Column Existence:** Every column listed in a primary key constraint must be a real column in the table.

**Unique Constraint Column Existence:** Every column listed in a unique constraint must be a real column in the table.

**Foreign Key Target Existence:** Every foreign key's referenced table must exist in the schema, and every referenced column must exist in that table.

### 10.2 Validation Failure Behavior

On any validation failure, the translator returns a structured error with an exact description of the violation. The pipeline halts — no graph is built, no data is generated.

---

## 11. Post-generation Validator (Dataset Validation)

The Post-generation Validator runs after generation and before export. It receives the complete generated dataset and `schema.Model`. It must verify every constraint independently.

### 11.1 Validation Checks (V1)

**PK Uniqueness:** For every table, assert that no two rows share the same primary key value (or combination for composite PKs).

**FK Referential Integrity:** For every FK relationship, assert that every non-null FK value in the referencing table exists as a PK value in the referenced table.

**Unique Constraint Integrity:** For every unique constraint (single or composite), assert that no two rows share the same value or value combination.

**Enum Value Integrity:** For every enum column, assert that every generated value is a declared enum value.

**Not Null Integrity:** For every NOT NULL column, assert that no row contains a NULL value.

**Length Integrity:** For every VARCHAR(n) column, assert that no generated value exceeds n characters.

### 11.2 Validation Failure Behavior

On any validation failure:

1. Generation output is **discarded entirely** — no partial output is written
2. A structured error is printed to stderr describing the exact violation
3. Process exits with code `1`

**Example validation error output:**

```
Validation failed: Foreign key constraint violation

  Table:   orders
  Column:  user_id
  Value:   9999
  References: users.id
  Reason:  Value 9999 does not exist in users.id

  This is an internal generation error. Please report it as a bug.
  https://github.com/your-org/synthgraph/issues
```

If the validator fails due to an internal bug (not a schema error), the error must include a prompt to report the issue, since all constraints should have been satisfied by the generator.

---

## 12. Output Formats

### 13.1 Exporter Interface

```go
type Exporter interface {
    Export(dataset *Dataset, model *schema.Model, out io.Writer) error
    Name() string
    FileExtension() string
}
```

### 13.2 SQL INSERT Exporter (V1 — Primary)

Output is a single `.sql` file containing:

1. A header comment block with generation metadata
2. `INSERT INTO` statements grouped by table
3. Tables appear in topological generation order
4. For cyclic tables: `INSERT` statements with NULLs, followed by `UPDATE` statements at the end
5. A transaction wrapper (`BEGIN; ... COMMIT;`) for atomicity

**Example output:**

```sql
-- Generated by SynthGraph v1.0.0
-- Schema: schema.sql
-- Rows per table: 100
-- Generated at: 2025-07-01T12:00:00Z
-- Seed: 42

BEGIN;

-- Table: users (100 rows)
INSERT INTO users (id, email, first_name, last_name, created_at)
VALUES
  (1, 'alice@example.com', 'Alice', 'Johnson', '2024-03-15 09:22:11'),
  (2, 'bob@example.com', 'Bob', 'Smith', '2024-07-02 14:55:30'),
  ...
;

-- Table: orders (100 rows)
INSERT INTO orders (id, user_id, amount, created_at)
VALUES
  (1, 3, 142.50, '2024-08-11 10:30:00'),
  ...
;

COMMIT;
```

### 13.3 CSV Exporter (V1 — Secondary)

One `.csv` file per table. File naming: `{table_name}.csv`. Header row with column names. All values properly escaped per RFC 4180.

### 12.4 Output Destination

By default, output is written to stdout (for SQL) or the current directory (for CSV). The `--output` flag specifies an output file path (SQL) or directory (CSV).

---

## 13. Error Handling Strategy

### 13.1 Error Categories

| Category | Description | Exit Code |
|---|---|---|
| Schema Parse Error | Malformed SQL, unsupported syntax | 1 |
| Unsupported Feature | Feature not implemented in current version | 2 |
| Cycle Error | Unresolvable circular dependency | 3 |
| Generation Error | Cannot satisfy constraint after max retries | 4 |
| Validation Error | Generated dataset fails post-generation validation | 5 |
| I/O Error | File not found, permission denied, write failure | 6 |
| Internal Error | Unexpected state — always prompts bug report | 99 |

### 13.2 Error Message Format

All errors follow a consistent structured format:

```
Error: [Short description of what failed]

  [Contextual detail — table name, column, value, etc.]

  [Why this happened — plain English explanation]

  [How to fix it — actionable suggestion when possible]
```

### 13.3 Warning System

Non-fatal issues are printed as warnings to stderr before generation begins:

```
Warning: CHECK constraint on "orders.amount" will not be enforced in v1.
  Expression: amount > 0
  This constraint is stored but not evaluated during generation.
```

---

## 14. CLI Specification

### 14.1 Commands

#### `synthgraph generate`

Generates a synthetic dataset from a schema file.

```
synthgraph generate --schema <path> [options]
```

**Required flags:**

| Flag | Description |
|---|---|
| `--schema` | Path to the SQL schema file |

**Optional flags:**

| Flag | Default | Description |
|---|---|---|
| `--rows` | `100` | Number of rows to generate per table |
| `--output` | stdout | Output file path (SQL) or directory (CSV) |
| `--format` | `sql` | Output format: `sql` or `csv` |
| `--seed` | `42` | RNG seed for deterministic output |
| `--tables` | all | Comma-separated list of tables to generate (others still generated if depended upon) |

**Example:**
```bash
synthgraph generate --schema ./schema.sql --rows 500 --output ./seed.sql
synthgraph generate --schema ./schema.sql --rows 100 --format csv --output ./fixtures/
```

#### `synthgraph inspect`

Analyzes a schema file and prints graph statistics. Does not generate data.

```
synthgraph inspect <schema-path>
```

**Output:**

```
SynthGraph Schema Inspection
─────────────────────────────────────────
Schema File:        schema.sql
Tables:             12
Relationships:      18
Cycles Detected:    1
Deepest Chain:      5 (users → orders → payments → refunds → disputes)
Isolated Tables:    2 (audit_logs, feature_flags)

Cycle Details:
  Cycle 1: users → organizations → users
  Breakpoint: organizations.owner_id (nullable) ✓
  Resolution: nullable deferred insertion

Generation Order:
  1. feature_flags
  2. audit_logs
  3. users
  4. organizations  [deferred FK: owner_id]
  5. products
  ...

Generation Plan:
  Tables:             12
  Deferred FKs:       1 (organizations.owner_id → users.id)
  Total rows:         1,200
  Estimated order:    shown above

Warnings:
  - CHECK constraint on orders.amount will not be enforced (v1 limitation)
─────────────────────────────────────────
Estimated generation time: ~2s for 100 rows/table
```

#### `synthgraph version`

Prints the current version.

```
synthgraph version
→ SynthGraph v1.0.0
```

### 16.2 Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Schema or input error |
| `2` | Unsupported feature |
| `3` | Unresolvable cycle |
| `4` | Generation constraint failure |
| `5` | Post-generation validation failure |
| `6` | I/O error |
| `99` | Internal error (bug) |

---

## 15. Non-Functional Requirements

### 15.1 Performance

- Must generate 100 rows per table for schemas with up to 50 tables in under 5 seconds on standard developer hardware
- Must generate 1,000 rows per table for schemas with up to 20 tables in under 10 seconds
- Memory usage must not exceed 512MB for any supported workload in v1

### 16.2 Determinism

- Identical inputs (schema + row count + seed) must always produce identical outputs
- The `--seed` flag controls all randomness in the system
- No system time, process ID, or other non-deterministic state may influence output values (timestamps are generated from the seeded RNG, not from `time.Now()`)

### 15.3 Correctness Guarantee

- Every dataset produced by SynthGraph must pass its own internal validator
- If the validator ever fails on internally generated data, it is treated as a critical bug, not a user error

### 15.4 Portability

- SynthGraph must compile and run on Linux, macOS, and Windows
- No external runtime dependencies — single static binary
- No system-level packages or shared libraries required

### 15.5 Testability

- Every pipeline stage must be independently unit-testable
- Golden tests must exist for the complete pipeline: schema file in, SQL file out
- All graph algorithms (topological sort, cycle detection, SCC) must have isolated unit tests with hand-crafted test graphs

---

## 16. V1 Scope Boundaries

### 16.1 Included in V1

- PostgreSQL DDL parser (via pg_query_go + translator)
- Internal Schema Model
- Directed schema graph construction
- Topological sort (Kahn's algorithm)
- Cycle detection (Tarjan's SCC)
- Nullable deferred insertion for cycle resolution
- Constraint-aware generation engine (PK, FK, unique, enum, nullable, length)
- Semantic name-based field generators
- Type-based fallback generators
- Post-generation constraint validator
- SQL INSERT exporter (with transaction wrapper)
- CSV exporter (one file per table)
- `generate` and `inspect` CLI commands
- Deterministic seeded generation
- Structured error messages with actionable suggestions
- Parser interface (defined, one implementation)
- Exporter interface (defined, two implementations)
- Generator registry (defined, v1 generators registered)

### 16.2 Explicitly Excluded from V1

- Prisma schema parser
- Drizzle schema parser
- Statistical distributions (normal, exponential, etc.)
- Business rule engine (conditional logic between fields)
- CHECK constraint enforcement
- Direct database insertion mode
- JSON / PostgreSQL COPY exporters
- Schema diff command
- CI/CD integration or GitHub Actions support
- Web application or API server
- Data anonymization
- AI/LLM-powered generation
- Cloud or SaaS features
- Telemetry or analytics of any kind
- Plugin or extension system
- Configuration file support (flags only in v1)
