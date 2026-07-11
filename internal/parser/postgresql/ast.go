// Package postgresql defines a lightweight, pure-Go intermediate representation
// for PostgreSQL DDL statements.
//
// This package has zero external dependencies. The translator operates on these
// types and produces a *schema.Model. The adapter (adapter.go) converts from
// pg_query_go's protobuf AST to these types, isolating the CGO dependency.
package postgresql

// Stmt is a parsed DDL statement.
type Stmt interface {
	isStmt()
}

// CreateTableStmt implements Stmt.
func (CreateTableStmt) isStmt() {}

// CreateEnumStmt implements Stmt.
func (CreateEnumStmt) isStmt() {}

// CreateTableStmt represents a parsed CREATE TABLE statement.
type CreateTableStmt struct {
	Schema           string
	Name             string
	IfNotExists      bool
	Columns          []ColumnDef
	TableConstraints []TableConstraint
}

// InlineFKRef represents a column-level REFERENCES constraint.
// It is equivalent to a table-level FOREIGN KEY constraint and must
// produce the same schema.ForeignKey during translation.
type InlineFKRef struct {
	RefTable   string
	RefColumns []string
	OnDelete   FKAction
	OnUpdate   FKAction
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

// ColumnDef represents a single column definition inside CREATE TABLE.
type ColumnDef struct {
	Name         string
	Type         ColumnType
	NotNull      bool
	Default      string       // raw default expression, empty if none
	IsPrimaryKey bool         // inline PRIMARY KEY (column-level)
	IsUnique     bool         // inline UNIQUE (column-level)
	References   *InlineFKRef // inline REFERENCES (column-level FK), nil if none
	Comment      string
}

// ColumnType represents a PostgreSQL column type.
type ColumnType struct {
	BaseType  string // "integer", "varchar", "timestamp", etc.
	Length    int    // for varchar(n), decimal(p)
	Precision int    // for decimal(p,s)
	IsSerial  bool   // SERIAL / BIGSERIAL
	IsArray   bool   // type[]
}

// TableConstraint represents a table-level constraint (PK, FK, UNIQUE, CHECK).
type TableConstraint struct {
	Type       ConstraintType
	Name       string   // optional named constraint
	Columns    []string // columns involved
	RefTable   string   // FK only
	RefColumns []string // FK only
	OnDelete   FKAction // FK only
	OnUpdate   FKAction // FK only
	CheckExpr  string   // CHECK only: the raw expression text
}

// ConstraintType enumerates table-level constraint types.
type ConstraintType int

const (
	ConstraintPrimaryKey ConstraintType = iota
	ConstraintForeignKey
	ConstraintUnique
	ConstraintCheck
)

// CreateEnumStmt represents a parsed CREATE TYPE ... AS ENUM statement.
type CreateEnumStmt struct {
	Schema string
	Name   string
	Values []string
}
