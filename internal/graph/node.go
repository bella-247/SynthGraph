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
	ID    string   `json:"id"`
	Kind  NodeKind `json:"kind"`
	Label string   `json:"label"`
	Data  any      `json:"data"`
}

// TableData carries all table-level metadata needed by downstream renderers.
type TableData struct {
	Name       string                    `json:"name"`
	PrimaryKey []string                  `json:"primary_key"`
	Unique     [][]string                `json:"unique,omitempty"`
	Checks     []schema.CheckConstraint  `json:"checks,omitempty"`
}

// ColumnData carries all column-level metadata needed by downstream renderers.
type ColumnData struct {
	Type         string   `json:"type"`
	Length       int      `json:"length,omitempty"`
	Precision    int      `json:"precision,omitempty"`
	Nullable     bool     `json:"nullable"`
	Default      *string  `json:"default,omitempty"`
	IsPrimaryKey bool     `json:"is_primary_key"`
}

// EnumData carries all enum-level metadata needed by downstream renderers.
type EnumData struct {
	Values []string `json:"values"`
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
