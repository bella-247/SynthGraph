// Package postgresql defines a lightweight, pure-Go intermediate representation
// for PostgreSQL DDL statements.
//
// This package has zero external dependencies. The translator operates on these
// types and produces a *schema.Schema. The adapter (adapter.go) converts from
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

// ColumnDef represents a single column definition inside CREATE TABLE.
type ColumnDef struct {
	Name         string
	Type         ColumnType
	NotNull      bool
	Default      string // raw default expression, empty if none
	IsPrimaryKey bool   // inline PRIMARY KEY (column-level)
	IsUnique     bool   // inline UNIQUE (column-level)
	Comment      string
}

// ColumnType represents a PostgreSQL column type.
type ColumnType struct {
	BaseType  string // "integer", "varchar", "timestamp", etc.
	Length    int    // for varchar(n), decimal(p)
	Precision int    // for decimal(p,s)
	IsSerial  bool   // SERIAL / BIGSERIAL
	IsArray   bool   // type[]
	EnumName  string // for enum types, the name of the enum
}

// TableConstraint represents a table-level constraint (PK, FK, UNIQUE, CHECK).
type TableConstraint struct {
	Type       ConstraintType
	Name       string   // optional named constraint
	Columns    []string // columns involved
	RefTable   string   // FK only
	RefColumns []string // FK only
	OnDelete   string   // FK only
	OnUpdate   string   // FK only
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
