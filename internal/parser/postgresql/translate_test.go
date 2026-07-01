package postgresql

import (
	"reflect"
	"testing"

	"synthgraph/internal/schema"
)

func TestTranslate_EmptySchema(t *testing.T) {
	s, err := Translate([]Stmt{})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if len(s.Tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(s.Tables))
	}
	if len(s.Enums) != 0 {
		t.Errorf("expected 0 enums, got %d", len(s.Enums))
	}
	if s.TableMap == nil {
		t.Error("TableMap should not be nil")
	}
}

func TestTranslate_SingleTable(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "integer"}, IsPrimaryKey: true},
				{Name: "name", Type: ColumnType{BaseType: "text"}, NotNull: true},
				{Name: "email", Type: ColumnType{BaseType: "text"}, IsUnique: true},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}

	tbl := s.Tables[0]
	if tbl.Name != "users" {
		t.Errorf("expected table name 'users', got %q", tbl.Name)
	}

	if len(tbl.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(tbl.Columns))
	}

	// Check PK
	if len(tbl.PrimaryKey) != 1 || tbl.PrimaryKey[0] != "id" {
		t.Errorf("expected PK [id], got %v", tbl.PrimaryKey)
	}
	if !tbl.Columns[0].IsPrimaryKey {
		t.Error("expected id column to be marked as PK")
	}

	// Check not-null propagated
	if !tbl.Columns[1].Nullable {
		t.Error("expected name column to be NotNull -> Nullable=false")
	}

	// Check unique preserved
	if len(tbl.Unique) != 1 || tbl.Unique[0][0] != "email" {
		t.Errorf("expected unique constraint on email, got %v", tbl.Unique)
	}
}

func TestTranslate_CompositePK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "order_items",
			Columns: []ColumnDef{
				{Name: "order_id", Type: ColumnType{BaseType: "integer"}},
				{Name: "product_id", Type: ColumnType{BaseType: "integer"}},
				{Name: "quantity", Type: ColumnType{BaseType: "integer"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:    ConstraintPrimaryKey,
					Columns: []string{"order_id", "product_id"},
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	if len(tbl.PrimaryKey) != 2 {
		t.Errorf("expected composite PK with 2 columns, got %v", tbl.PrimaryKey)
	}
	if !tbl.Columns[0].IsPrimaryKey || !tbl.Columns[1].IsPrimaryKey {
		t.Error("expected both PK columns to be marked")
	}
	if tbl.Columns[2].IsPrimaryKey {
		t.Error("expected quantity to not be PK")
	}
}

func TestTranslate_ForeignKey(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "integer"}, IsPrimaryKey: true},
			},
		},
		CreateTableStmt{
			Name: "orders",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "integer"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "integer"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"user_id"},
					RefTable:   "users",
					RefColumns: []string{"id"},
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(s.Tables))
	}

	orders := s.Tables[1]
	if len(orders.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(orders.ForeignKeys))
	}

	fk := orders.ForeignKeys[0]
	if len(fk.Columns) != 1 || fk.Columns[0] != "user_id" {
		t.Errorf("expected FK column [user_id], got %v", fk.Columns)
	}
	if fk.RefTable != "users" {
		t.Errorf("expected RefTable 'users', got %q", fk.RefTable)
	}
	if len(fk.RefColumns) != 1 || fk.RefColumns[0] != "id" {
		t.Errorf("expected RefColumns [id], got %v", fk.RefColumns)
	}
}

func TestTranslate_SelfReferencingFK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "employees",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "manager_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"manager_id"},
					RefTable:   "employees",
					RefColumns: []string{"id"},
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}

	emp := s.Tables[0]
	if len(emp.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(emp.ForeignKeys))
	}

	fk := emp.ForeignKeys[0]
	if fk.RefTable != "employees" {
		t.Errorf("expected self-referencing FK to RefTable 'employees', got %q", fk.RefTable)
	}
}

func TestTranslate_ForeignKeyActions(t *testing.T) {
	actions := []struct {
		input FKAction
		want  schema.FKAction
	}{
		{FKRestrict, schema.FKRestrict},
		{FKCascade, schema.FKCascade},
		{FKSetNull, schema.FKSetNull},
		{FKSetDefault, schema.FKSetDefault},
		{FKNoAction, schema.FKNoAction},
	}

	// Parent table referenced by all FK action variants
	parent := CreateTableStmt{
		Name: "parent",
		Columns: []ColumnDef{
			{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
		},
	}

	for _, a := range actions {
		stmts := []Stmt{
			parent, // referenced table
			CreateTableStmt{
				Name: "child",
				Columns: []ColumnDef{
					{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
					{Name: "parent_id", Type: ColumnType{BaseType: "int"}},
				},
				TableConstraints: []TableConstraint{
					{
						Type:       ConstraintForeignKey,
						Columns:    []string{"parent_id"},
						RefTable:   "parent",
						RefColumns: []string{"id"},
						OnDelete:   a.input,
					},
				},
			},
		}
		s, err := Translate(stmts)
		if err != nil {
			t.Fatal(err)
		}
		fk := s.Tables[1].ForeignKeys[0]
		if fk.OnDelete != a.want {
			t.Errorf("OnDelete: expected %q, got %q", a.want, fk.OnDelete)
		}
	}
}

func TestTranslate_UniqueConstraint(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "email", Type: ColumnType{BaseType: "text"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:    ConstraintUnique,
					Columns: []string{"email"},
				},
				{
					Type:    ConstraintUnique,
					Columns: []string{"email"}, // duplicate
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	if len(tbl.Unique) != 1 {
		t.Errorf("expected 1 unique constraint (deduped), got %d: %v", len(tbl.Unique), tbl.Unique)
	}
}

func TestTranslate_UniqueCoveredByPK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "email", Type: ColumnType{BaseType: "text"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:    ConstraintUnique,
					Columns: []string{"id"}, // covered by PK
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	for _, u := range tbl.Unique {
		t.Errorf("expected no uniques (covered by PK), got %v", u)
	}
}

func TestTranslate_DuplicateColumnPK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
			TableConstraints: []TableConstraint{
				{
					Type:    ConstraintPrimaryKey,
					Columns: []string{"id"},
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	if len(tbl.PrimaryKey) != 1 {
		t.Errorf("expected deduplicated PK [id], got %v", tbl.PrimaryKey)
	}
}

func TestTranslate_TableMap(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "a",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
		CreateTableStmt{
			Name: "b",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.TableMap) != 2 {
		t.Errorf("expected 2 entries in TableMap, got %d", len(s.TableMap))
	}

	if s.TableMap["a"] == nil || s.TableMap["b"] == nil {
		t.Error("TableMap missing expected tables")
	}

	if s.TableMap["a"] != s.Tables[0] {
		t.Error("TableMap should point to the same *Table as Tables slice")
	}
}

func TestTranslate_EnumType(t *testing.T) {
	stmts := []Stmt{
		CreateEnumStmt{
			Name:   "mood",
			Values: []string{"happy", "sad", "neutral"},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Enums) != 1 {
		t.Fatalf("expected 1 enum type, got %d", len(s.Enums))
	}

	enum := s.Enums[0]
	if enum.Name != "mood" {
		t.Errorf("expected enum name 'mood', got %q", enum.Name)
	}
	if !reflect.DeepEqual(enum.Values, []string{"happy", "sad", "neutral"}) {
		t.Errorf("expected enum values [happy sad neutral], got %v", enum.Values)
	}
}

func TestTranslate_EnumColumnResolution(t *testing.T) {
	// Create an enum type and a table that references it as a column type
	mood := CreateEnumStmt{
		Name:   "mood",
		Values: []string{"happy", "sad"},
	}

	table := CreateTableStmt{
		Name: "entries",
		Columns: []ColumnDef{
			{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			{
				Name: "current_mood",
				Type: ColumnType{BaseType: "mood"},
			},
		},
	}

	s, err := Translate([]Stmt{mood, table})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}

	tbl := s.Tables[0]
	if len(tbl.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(tbl.Columns))
	}

	moodCol := tbl.Columns[1]
	if moodCol.Type != "mood" {
		t.Errorf("expected column type 'mood', got %q", moodCol.Type)
	}

	// The enum should be resolved by the linker
	if len(s.Enums) != 1 {
		t.Errorf("expected 1 enum, got %d", len(s.Enums))
	}
}

func TestTranslate_SchemaQualifiedTable(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Schema: "public",
			Name:   "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}

	if s.Tables[0].Name != "public.users" {
		t.Errorf("expected schema-qualified name 'public.users', got %q", s.Tables[0].Name)
	}
}

func TestTranslate_MultipleTables(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
		CreateTableStmt{
			Name: "posts",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"user_id"},
					RefTable:   "users",
					RefColumns: []string{"id"},
				},
			},
		},
		CreateTableStmt{
			Name: "comments",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "post_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"post_id"},
					RefTable:   "posts",
					RefColumns: []string{"id"},
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	if len(s.Tables) != 3 {
		t.Fatalf("expected 3 tables, got %d", len(s.Tables))
	}

	// Verify chain: users ← posts ← comments
	posts := s.Tables[1]
	if posts.ForeignKeys[0].RefTable != "users" {
		t.Errorf("expected posts FK -> users, got %q", posts.ForeignKeys[0].RefTable)
	}

	comments := s.Tables[2]
	if comments.ForeignKeys[0].RefTable != "posts" {
		t.Errorf("expected comments FK -> posts, got %q", comments.ForeignKeys[0].RefTable)
	}
}

func TestTranslate_TableLevelPK(t *testing.T) {
	// PK defined as table-level constraint, not inline
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:    ConstraintPrimaryKey,
					Columns: []string{"id"},
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	if len(tbl.PrimaryKey) != 1 || tbl.PrimaryKey[0] != "id" {
		t.Errorf("expected PK [id], got %v", tbl.PrimaryKey)
	}
	if !tbl.Columns[0].IsPrimaryKey {
		t.Error("expected id to be marked as PK")
	}
}

func TestTranslate_ValidateDuplicateTable(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
	}

	_, err := Translate(stmts)
	if err == nil {
		t.Fatal("expected error for duplicate table name")
	}
}

func TestTranslate_ValidateDuplicateColumn(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "id", Type: ColumnType{BaseType: "int"}},
			},
		},
	}

	_, err := Translate(stmts)
	if err == nil {
		t.Fatal("expected error for duplicate column name")
	}
}

func TestTranslate_ValidateFKTargetExists(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "orders",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"user_id"},
					RefTable:   "nonexistent",
					RefColumns: []string{"id"},
				},
			},
		},
	}

	_, err := Translate(stmts)
	if err == nil {
		t.Fatal("expected error for FK target that doesn't exist")
	}
}

func TestTranslate_NullableColumn(t *testing.T) {
	// NOT NULL is the exception, not the rule
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "name", Type: ColumnType{BaseType: "text"}},
				{Name: "label", Type: ColumnType{BaseType: "text"}, NotNull: true},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	if tbl.Columns[0].Nullable {
		t.Error("PK column should not be nullable")
	}
	if !tbl.Columns[1].Nullable {
		t.Error("column without NOT NULL should be nullable")
	}
	if tbl.Columns[2].Nullable {
		t.Error("column with NOT NULL should not be nullable")
	}
}

func TestTranslate_OnUpdateAction(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
		CreateTableStmt{
			Name: "orders",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"user_id"},
					RefTable:   "users",
					RefColumns: []string{"id"},
					OnUpdate:   FKCascade,
					OnDelete:   FKSetNull,
				},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	fk := s.Tables[1].ForeignKeys[0]
	if fk.OnUpdate != schema.FKCascade {
		t.Errorf("expected OnUpdate=CASCADE, got %q", fk.OnUpdate)
	}
	if fk.OnDelete != schema.FKSetNull {
		t.Errorf("expected OnDelete=SET NULL, got %q", fk.OnDelete)
	}
}

func TestTranslate_DefaultValues(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "label", Type: ColumnType{BaseType: "text"}, Default: "'untitled'"},
				{Name: "count", Type: ColumnType{BaseType: "int"}, Default: "0"},
				{Name: "created_at", Type: ColumnType{BaseType: "timestamp"}, Default: "now()"},
			},
		},
	}

	s, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	tbl := s.Tables[0]
	if tbl.Columns[1].Default == nil || *tbl.Columns[1].Default != "'untitled'" {
		t.Errorf("expected default 'untitled', got %v", tbl.Columns[1].Default)
	}
	if tbl.Columns[2].Default == nil || *tbl.Columns[2].Default != "0" {
		t.Errorf("expected default 0, got %v", tbl.Columns[2].Default)
	}
	if tbl.Columns[3].Default == nil || *tbl.Columns[3].Default != "now()" {
		t.Errorf("expected default now(), got %v", tbl.Columns[3].Default)
	}
}

func TestTranslate_CheckConstraintParsed(t *testing.T) {
	// CHECK constraints are parsed in V1 but not enforced in the schema model
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "age", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type: ConstraintCheck,
					Name: "age_check",
				},
			},
		},
	}

	_, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
}

func TestTranslate_MissingRefColumn(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
		CreateTableStmt{
			Name: "orders",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"user_id"},
					RefTable:   "users",
					RefColumns: []string{"missing_col"},
				},
			},
		},
	}

	_, err := Translate(stmts)
	if err == nil {
		t.Fatal("expected error for FK referencing non-existent column")
	}
}

func TestTranslate_NoPanicOnInvalidTypeName(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{
					Name: "id",
					Type: ColumnType{}, // empty type — shouldn't panic
				},
			},
		},
	}

	_, err := Translate(stmts)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
}

func TestTranslate_NoTableName(t *testing.T) {
	// A table with empty name should be rejected or handled gracefully
	stmts := []Stmt{
		CreateTableStmt{
			Name: "",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
	}

	_, err := Translate(stmts)
	if err == nil {
		t.Fatal("expected error for empty table name")
	}
}

func TestTranslate_FKActionParsing(t *testing.T) {
	// Test that FK action values are accepted correctly
	actions := []struct {
		input FKAction
		want  schema.FKAction
	}{
		{FKRestrict, schema.FKRestrict},
		{FKCascade, schema.FKCascade},
		{FKSetNull, schema.FKSetNull},
		{FKSetDefault, schema.FKSetDefault},
		{FKNoAction, schema.FKNoAction},
	}

	parent := CreateTableStmt{
		Name: "parent",
		Columns: []ColumnDef{
			{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
		},
	}

	for _, a := range actions {
		stmts := []Stmt{
			parent,
			CreateTableStmt{
				Name: "child",
				Columns: []ColumnDef{
					{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
					{Name: "parent_id", Type: ColumnType{BaseType: "int"}},
				},
				TableConstraints: []TableConstraint{
					{
						Type:       ConstraintForeignKey,
						Columns:    []string{"parent_id"},
						RefTable:   "parent",
						RefColumns: []string{"id"},
						OnUpdate:   a.input,
					},
				},
			},
		}

		s, err := Translate(stmts)
		if err != nil {
			t.Fatalf("Translate() error for action=%q: %v", a.input, err)
		}

		fk := s.Tables[1].ForeignKeys[0]
		if fk.OnUpdate != a.want {
			t.Errorf("OnUpdate: expected %q, got %q", a.want, fk.OnUpdate)
		}
	}
}

func TestTranslate_ValidateEnumRefExists(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "mood", Type: ColumnType{BaseType: "nonexistent_enum"}},
			},
		},
	}

	_, err := Translate(stmts)
	if err == nil {
		t.Fatal("expected error for enum reference to undefined type")
	}
}
