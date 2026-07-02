// Package schema defines the canonical internal representation of a database schema.
//
// This is the single most important data structure in SynthGraph.
// Every stage after the parser consumes only this model.
// Every parser (PostgreSQL, MySQL, Prisma, etc.) must produce this model.
//
// Think of it as SynthGraph's Intermediate Representation (IR).
package schema

// Model is the unified internal representation of any supported schema format.
// All parsers must produce this structure. All downstream stages consume it.
type Model struct {
	Tables   []*Table          `json:"tables"`
	TableMap map[string]*Table `json:"-"` // O(1) lookup by table name, built during construction
	Enums    []EnumType        `json:"enums,omitempty"`
}

// EnumType represents a named enum type.
type EnumType struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Table represents a single database table with its columns and constraints.
type Table struct {
	Name        string        `json:"name"`
	Columns     []Column      `json:"columns"`
	PrimaryKey  []string      `json:"primary_key,omitempty"`
	ForeignKeys []ForeignKey  `json:"foreign_keys,omitempty"`
	Unique      [][]string    `json:"unique,omitempty"`
}

// Column represents a single column in a table.
type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

// FKAction represents a foreign key action (ON DELETE / ON UPDATE).
type FKAction string

const (
	FKNoAction   FKAction = "NO ACTION"
	FKCascade    FKAction = "CASCADE"
	FKRestrict   FKAction = "RESTRICT"
	FKSetNull    FKAction = "SET NULL"
	FKSetDefault FKAction = "SET DEFAULT"
)

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
	OnDelete   FKAction `json:"on_delete,omitempty"`
	OnUpdate   FKAction `json:"on_update,omitempty"`
}
