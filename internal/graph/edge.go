package graph

import "synthgraph/internal/schema"

// EdgeKind identifies the semantic relationship between two nodes.
type EdgeKind string

const (
	// EdgeKindContains connects a table node to each of its column nodes.
	// Direction: table → column.
	EdgeKindContains EdgeKind = "contains"

	// EdgeKindReferences connects a referencing table to the table it references
	// via a foreign key constraint. Column-level mapping is stored in FKMetadata.
	// Direction: child table → parent table.
	EdgeKindReferences EdgeKind = "references"

	// EdgeKindReferencedBy connects a referenced table to the tables that reference it.
	// This is the reverse of EdgeKindReferences and carries the same FKMetadata.
	// Direction: parent table → child table.
	EdgeKindReferencedBy EdgeKind = "referenced_by"

	// EdgeKindDependsOn connects a table to another table whose modification or removal
	// would affect it. This is the same direction as EdgeKindReferences (child → parent)
	// but is explicitly intended for impact analysis traversal.
	// Direction: child table → parent table.
	EdgeKindDependsOn EdgeKind = "depends_on"

	// EdgeKindUsesEnum connects a column node to the enum type it uses.
	// Direction: column → enum.
	EdgeKindUsesEnum EdgeKind = "uses_enum"
)

// Cardinality describes the multiplicity of a foreign key relationship
// between two tables, from the perspective of the referenced (parent) table.
type Cardinality string

const (
	// CardinalityOneToOne means each parent row is referenced by at most one child row.
	// Inferred when the FK columns are the complete primary key of the child table.
	CardinalityOneToOne Cardinality = "one_to_one"

	// CardinalityOneToMany means each parent row can be referenced by many child rows.
	// This is the default for most foreign key relationships.
	CardinalityOneToMany Cardinality = "one_to_many"

	// CardinalityManyToMany means two tables are connected through a junction
	// (associative) table whose composite primary key consists entirely of
	// foreign keys referencing both tables.
	CardinalityManyToMany Cardinality = "many_to_many"
)

// Edge represents a directed relationship between two nodes in the schema graph.
type Edge struct {
	// From is the node ID of the source node.
	From string

	// To is the node ID of the destination node.
	To string

	// Kind is the semantic relationship this edge represents.
	Kind EdgeKind

	// Metadata carries edge-kind-specific data.
	// For EdgeKindReferences edges: *FKMetadata.
	// For all other edge kinds: nil.
	Metadata any
}

// FKMetadata carries the full details of a foreign key constraint, stored on
// EdgeKindReferences and EdgeKindReferencedBy edges.
//
// FK edges connect tables (not individual columns) to match the ER diagram
// convention. The column mapping is stored here as metadata so that renderers
// can display it without re-parsing the schema.
type FKMetadata struct {
	// LocalColumns lists the column names in the referencing (child) table.
	LocalColumns []string

	// ForeignColumns lists the corresponding column names in the referenced (parent) table.
	// Position i in LocalColumns maps to position i in ForeignColumns.
	ForeignColumns []string

	// OnDelete describes the action taken on child rows when the referenced parent row is deleted.
	OnDelete schema.FKAction

	// OnUpdate describes the action taken on child rows when the referenced parent row is updated.
	OnUpdate schema.FKAction

	// Cardinality describes the multiplicity of this FK relationship from the
	// perspective of the referenced (parent) table. Inferred automatically from
	// the child table's primary key and unique constraints.
	//   - one_to_one:   FK columns are the complete primary key of the child.
	//   - one_to_many:  FK columns are not unique in the child (the default).
	//   - many_to_many: the child is a junction table whose composite PK consists
	//                   entirely of foreign keys.
	Cardinality Cardinality `json:"cardinality,omitempty"`
}
