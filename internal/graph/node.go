// Package graph defines the canonical graph data structure for SynthGraph.
//
// The graph is the central data structure of the project. The parser front-end
// produces a schema.Model; this package transforms that model into a traversable
// Graph that every downstream feature consumes — Mermaid rendering, Draw.io
// export, AI documentation, dependency analysis, and future dialects.
//
// # Architecture position
//
//	SQL File → Parser → Translator → schema.Model → graph.Build → Graph
//	                                                                  ├── Mermaid
//	                                                                  ├── Draw.io
//	                                                                  ├── SVG
//	                                                                  ├── JSON
//	                                                                  ├── AI docs
//	                                                                  └── Analysis
//
// This separation of concerns is a hallmark of mature compiler design:
// the parser understands SQL, the translator normalises it into a
// dialect-independent schema, and the graph builder transforms that schema
// into the universal representation that every back-end can consume.
package graph

import "synthgraph/internal/schema"

// NodeKind identifies what kind of database object a node represents.
type NodeKind string

const (
	// NodeKindTable represents a database table.
	NodeKindTable NodeKind = "table"

	// NodeKindColumn represents a single column within a table.
	NodeKindColumn NodeKind = "column"

	// NodeKindEnum represents a named enum type.
	NodeKindEnum NodeKind = "enum"
)

// Node represents a single vertex in the schema graph.
//
// Every node has a globally unique, deterministic ID constructed from
// its kind and name. The Data field carries kind-specific metadata
// that downstream renderers and analyzers can use without re-parsing
// the original schema.
type Node struct {
	// ID is the globally unique, deterministic identifier for this node.
	// Formats:
	//   - Tables:  "table:{name}"
	//   - Columns: "column:{table_name}.{column_name}"
	//   - Enums:   "enum:{name}"
	ID string

	// Kind identifies what database object this node represents.
	Kind NodeKind

	// Label is the human-readable display name (typically just the object name
	// without any schema prefix).
	Label string

	// Data carries kind-specific metadata. Cast to the appropriate concrete type:
	//   - NodeKindTable:  TableData
	//   - NodeKindColumn: ColumnData
	//   - NodeKindEnum:   EnumData
	Data any
}

// TableData carries all table-level metadata needed by downstream renderers.
type TableData struct {
	// Name is the fully-qualified table name (e.g. "public.users" or "users").
	Name string

	// PrimaryKey lists the column names that form the primary key.
	PrimaryKey []string

	// Unique lists each unique constraint as a set of column names.
	Unique [][]string

	// Checks lists CHECK constraint expressions preserved from the schema.
	Checks []schema.CheckConstraint
}

// ColumnData carries all column-level metadata needed by downstream renderers.
type ColumnData struct {
	// Type is the canonical abstract type name (e.g. "int", "varchar", "boolean").
	Type string

	// Length is the declared length for types like VARCHAR(n).
	// Zero means no length was specified.
	Length int

	// Precision is the declared precision for types like DECIMAL(p,s).
	// Zero means no precision was specified.
	Precision int

	// Nullable is true if the column accepts NULL values.
	Nullable bool

	// Default is the raw default expression string, or nil if no default was declared.
	Default *string

	// IsPrimaryKey is true if this column is part of the table's primary key.
	IsPrimaryKey bool
}

// EnumData carries all enum-level metadata needed by downstream renderers.
type EnumData struct {
	// Values lists the valid string values for this enum type, in declaration order.
	Values []string
}

// tableNodeID returns the deterministic node ID for a table node.
// Format: "table:{tableName}"
func tableNodeID(tableName string) string {
	return "table:" + tableName
}

// columnNodeID returns the deterministic node ID for a column node.
// Format: "column:{tableName}.{columnName}"
func columnNodeID(tableName, columnName string) string {
	return "column:" + tableName + "." + columnName
}

// enumNodeID returns the deterministic node ID for an enum node.
// Format: "enum:{enumName}"
func enumNodeID(enumName string) string {
	return "enum:" + enumName
}
