package semantic

// TableRole classifies the structural or business role of a table.
//
// A table can hold multiple roles simultaneously. For example, an employee
// table that self-references its manager is both Entity and Hierarchical.
// An order table with temporal columns is both Entity and Transactional.
//
// Roles are never mutually exclusive. Inspect a SemanticNode's Roles slice
// to get the full picture.
type TableRole string

const (
	// TableRoleEntity represents a primary domain object — a thing the system
	// stores and manages (User, Product, Article). This is the default role
	// assigned to any table that does not clearly fit a more specific pattern.
	TableRoleEntity TableRole = "entity"

	// TableRoleJunction represents a many-to-many connector table (also known
	// as an associative or bridge table). The structural signal is a composite
	// primary key where every primary key column is also a foreign key column.
	// Example: product_categories with PK (product_id, category_id).
	TableRoleJunction TableRole = "junction"

	// TableRoleLookup represents a reference or code table — a small, stable
	// collection of known values (statuses, countries, categories, roles).
	// Structural signals: few or no outgoing foreign keys, frequently referenced
	// by many other tables, no temporal tracking columns.
	TableRoleLookup TableRole = "lookup"

	// TableRoleTransactional represents an event or fact record — something that
	// captures that something happened at a point in time (orders, invoices,
	// payments, shipments, audit logs). Structural signals: has temporal columns,
	// has foreign keys to entity tables, is not a junction or lookup table.
	TableRoleTransactional TableRole = "transactional"

	// TableRoleHierarchical represents a self-referencing table — a table where
	// rows form a tree or directed graph among themselves. The structural signal
	// is a foreign key from the table back to itself. Example: employees with
	// a manager_id that references employees.id.
	TableRoleHierarchical TableRole = "hierarchical"
)

// RelationshipKind describes the semantic nature of a foreign key relationship
// using database and domain modelling vocabulary rather than ORM terminology.
//
// These kinds are intentionally language-agnostic and ORM-agnostic. Renderers
// and AI layers are free to translate them into whatever terms their domain
// requires (e.g., RelationshipKindComposition → "belongs_to" in ActiveRecord,
// "Aggregate" in DDD, "composition" in UML).
type RelationshipKind string

const (
	// RelationshipKindComposition describes a "parent owns child" relationship
	// where the child row cannot exist independently of the parent. The child
	// is existentially dependent on the parent.
	//
	// Structural signal: FK is NOT NULL and the ON DELETE action is CASCADE.
	// Example: order_items cannot exist without an order.
	RelationshipKindComposition RelationshipKind = "composition"

	// RelationshipKindAssociation describes a strong reference where the child
	// references the parent but has an independent lifecycle — the parent could
	// be logically replaced or the child could reference a different parent.
	//
	// Structural signal: FK is NOT NULL, ON DELETE is not CASCADE (e.g., RESTRICT,
	// NO ACTION, SET DEFAULT).
	// Example: an order references a user, but an order is not owned by a user
	// in the UML sense — it is associated with one.
	RelationshipKindAssociation RelationshipKind = "association"

	// RelationshipKindAggregation describes a weak or optional reference where
	// the child row can optionally point to a parent row.
	//
	// Structural signal: FK source columns are nullable.
	// Example: a post may optionally reference a category (category_id is nullable).
	RelationshipKindAggregation RelationshipKind = "aggregation"

	// RelationshipKindHierarchy describes a self-referencing relationship where
	// rows of the same table reference each other to form a tree or graph.
	//
	// Structural signal: FK source table name == FK target table name.
	// Example: employees.manager_id → employees.id.
	RelationshipKindHierarchy RelationshipKind = "hierarchy"

	// RelationshipKindManyToMany describes the implicit many-to-many association
	// expressed through a junction table. This kind is assigned to both FK edges
	// of a table whose cardinality is already resolved as many_to_many by the
	// graph layer.
	//
	// Structural signal: graph.CardinalityManyToMany on the FK edge.
	RelationshipKindManyToMany RelationshipKind = "many_to_many"
)

// TemporalPattern records which time-tracking columns were detected on a table.
// Detection is based purely on column name matching (case-insensitive), which
// is a strong cross-language convention used consistently across ecosystems
// (Rails, Django, Laravel, Prisma, GORM, etc.).
type TemporalPattern struct {
	// HasCreatedAt is true if a column named "created_at" (case-insensitive)
	// was found on the table.
	HasCreatedAt bool

	// HasUpdatedAt is true if a column named "updated_at" (case-insensitive)
	// was found on the table.
	HasUpdatedAt bool

	// HasDeletedAt is true if a column named "deleted_at" (case-insensitive)
	// was found on the table. Also implies soft-delete behaviour.
	HasDeletedAt bool
}

// AuditPattern records which accountability-tracking columns were detected on a table.
// These columns typically reference a user who performed an action (created, updated,
// or deleted a row), and are distinct from TemporalPattern which only records *when*.
type AuditPattern struct {
	// HasCreatedBy is true if a column named "created_by" (case-insensitive)
	// was found on the table.
	HasCreatedBy bool

	// HasUpdatedBy is true if a column named "updated_by" (case-insensitive)
	// was found on the table.
	HasUpdatedBy bool

	// HasDeletedBy is true if a column named "deleted_by" (case-insensitive)
	// was found on the table.
	HasDeletedBy bool
}

// Inference carries a single semantic conclusion about a node, together with
// the evidence that supports it and a confidence score.
//
// Inferences are produced by Rules. Each Rule can return zero or more inferences.
// Storing inferences separately from the conclusion allows future AI layers to
// explain their reasoning: "I classified this as a junction table because all
// primary key columns are also foreign key columns (confidence: 0.95)."
type Inference struct {
	// Kind identifies what was inferred. This is a short, machine-readable label
	// matching the corresponding TableRole or RelationshipKind string value, or
	// a signal-specific label like "temporal", "soft_delete", "audit".
	Kind string

	// Confidence is a value in [0.0, 1.0] representing how certain the engine is
	// about this inference. A confidence of 1.0 means the structural evidence is
	// unambiguous. Lower values indicate heuristic or probabilistic conclusions.
	Confidence float64

	// Evidence is an ordered list of human-readable reasons that support this
	// inference. Each item is a self-contained explanation (e.g., "all 2 primary
	// key columns are foreign key columns"). Renderers and AI layers can relay
	// these directly to users.
	Evidence []string
}
