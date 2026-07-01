package postgresql

import (
	"reflect"
	"strings"
	"testing"
)

func TestTranslate_EmptySchema(t *testing.T) {
	s, err := Translate([]Stmt{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(s.Tables))
	}
}

func TestTranslate_SimpleTable(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, NotNull: true, IsPrimaryKey: true},
				{Name: "email", Type: ColumnType{BaseType: "varchar", Length: 255}, NotNull: true},
				{Name: "name", Type: ColumnType{BaseType: "varchar", Length: 100}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}

	u := s.Tables[0]
	if u.Name != "users" {
		t.Errorf("expected name 'users', got %q", u.Name)
	}

	if len(u.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(u.Columns))
	}

	// id: INT, NOT NULL, PK
	if u.Columns[0].Name != "id" || u.Columns[0].Type != "int" || u.Columns[0].Nullable || !u.Columns[0].IsPrimaryKey {
		t.Errorf("unexpected id column: %+v", u.Columns[0])
	}

	// email: VARCHAR, NOT NULL
	if u.Columns[1].Name != "email" || u.Columns[1].Type != "varchar" || u.Columns[1].Nullable {
		t.Errorf("unexpected email column: %+v", u.Columns[1])
	}

	// name: VARCHAR, nullable
	if u.Columns[2].Name != "name" || u.Columns[2].Type != "varchar" || !u.Columns[2].Nullable {
		t.Errorf("unexpected name column: %+v", u.Columns[2])
	}

	// PK
	if !reflect.DeepEqual(u.PrimaryKey, []string{"id"}) {
		t.Errorf("expected PK [id], got %v", u.PrimaryKey)
	}
}

func TestTranslate_NullableDefault(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "name", Type: ColumnType{BaseType: "text"}, Default: "'unknown'"},
				{Name: "score", Type: ColumnType{BaseType: "int"}, Default: "0", NotNull: true},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	// name: nullable with default
	if !tbl.Columns[1].Nullable {
		t.Error("name should be nullable")
	}
	if tbl.Columns[1].Default == nil || *tbl.Columns[1].Default != "'unknown'" {
		t.Errorf("name default: got %v", tbl.Columns[1].Default)
	}

	// score: NOT NULL with default
	if tbl.Columns[2].Nullable {
		t.Error("score should NOT be nullable")
	}
	if tbl.Columns[2].Default == nil || *tbl.Columns[2].Default != "0" {
		t.Errorf("score default: got %v", tbl.Columns[2].Default)
	}
}

func TestTranslate_AllTypes(t *testing.T) {
	types := map[string]string{
		"int":              "int",
		"integer":          "int",
		"int4":             "int",
		"bigint":           "bigint",
		"int8":             "bigint",
		"smallint":         "smallint",
		"int2":             "smallint",
		"text":             "text",
		"varchar":          "varchar",
		"char":             "char",
		"boolean":          "boolean",
		"bool":             "boolean",
		"decimal":          "decimal",
		"numeric":          "decimal",
		"real":             "float",
		"float4":           "float",
		"float":            "float",
		"double precision": "double",
		"float8":           "double",
		"date":             "date",
		"time":             "time",
		"timestamp":        "timestamp",
		"timestamptz":      "timestamp",
		"uuid":             "uuid",
		"json":             "json",
		"jsonb":            "jsonb",
		"bytea":            "bytea",
		"interval":         "interval",
		"inet":             "text",
		"cidr":             "text",
		"macaddr":          "text",
		"citext":           "text",
		"name":             "text",
		"bpchar":           "char",
		"character varying": "varchar",
		"character":        "char",
	}

	var cols []ColumnDef
	for pgType := range types {
		cols = append(cols, ColumnDef{
			Name: "c" + strings.ReplaceAll(pgType, " ", "_"),
			Type: ColumnType{BaseType: pgType},
			NotNull: true,
		})
	}

	stmts := []Stmt{CreateTableStmt{Name: "all_types", Columns: cols}}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}

	tbl := s.Tables[0]
	for _, c := range tbl.Columns {
		pgName := strings.TrimPrefix(c.Name, "c")
		pgName = strings.ReplaceAll(pgName, "_", " ")
		expectedType := types[pgName]

		if c.Type != expectedType {
			t.Errorf("type %q: expected %q, got %q", pgName, expectedType, c.Type)
		}
	}
}

func TestTranslate_SerialTypes(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "serials",
			Columns: []ColumnDef{
				{Name: "a", Type: ColumnType{BaseType: "serial"}, IsPrimaryKey: true},
				{Name: "b", Type: ColumnType{BaseType: "bigserial"}},
				{Name: "c", Type: ColumnType{BaseType: "smallserial"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	tests := []struct {
		colIdx   int
		expected string
	}{
		{0, "int"},
		{1, "bigint"},
		{2, "smallint"},
	}
	for _, tt := range tests {
		if tbl.Columns[tt.colIdx].Type != tt.expected {
			t.Errorf("column %d: expected %q, got %q",
				tt.colIdx, tt.expected, tbl.Columns[tt.colIdx].Type)
		}
	}
}

func TestTranslate_CompositePrimaryKey(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "order_items",
			Columns: []ColumnDef{
				{Name: "order_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
				{Name: "product_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
				{Name: "qty", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintPrimaryKey, Columns: []string{"order_id", "product_id"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	expected := []string{"order_id", "product_id"}
	if !reflect.DeepEqual(tbl.PrimaryKey, expected) {
		t.Errorf("expected PK %v, got %v", expected, tbl.PrimaryKey)
	}
}

func TestTranslate_InlineAndTablePK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "a", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "b", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "c", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintPrimaryKey, Columns: []string{"a", "b"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if !reflect.DeepEqual(tbl.PrimaryKey, []string{"a", "b"}) {
		t.Errorf("expected PK [a b], got %v", tbl.PrimaryKey)
	}
}

func TestTranslate_ForeignKey(t *testing.T) {
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
				{Name: "user_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"user_id"},
					RefTable:   "users",
					RefColumns: []string{"id"},
					OnDelete:   "CASCADE",
					OnUpdate:   "NO ACTION",
				},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[1] // orders is second

	if len(tbl.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(tbl.ForeignKeys))
	}

	fk := tbl.ForeignKeys[0]
	if !reflect.DeepEqual(fk.Columns, []string{"user_id"}) {
		t.Errorf("FK columns: got %v", fk.Columns)
	}
	if fk.RefTable != "users" {
		t.Errorf("FK ref table: expected 'users', got %q", fk.RefTable)
	}
	if !reflect.DeepEqual(fk.RefColumns, []string{"id"}) {
		t.Errorf("FK ref columns: got %v", fk.RefColumns)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("FK on delete: expected 'CASCADE', got %q", fk.OnDelete)
	}
	if fk.OnUpdate != "NO ACTION" {
		t.Errorf("FK on update: expected 'NO ACTION', got %q", fk.OnUpdate)
	}
}

func TestTranslate_CompositeForeignKey(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "order_items",
			Columns: []ColumnDef{
				{Name: "order_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
				{Name: "product_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintPrimaryKey, Columns: []string{"order_id", "product_id"}},
			},
		},
		CreateTableStmt{
			Name: "order_details",
			Columns: []ColumnDef{
				{Name: "order_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
				{Name: "product_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintPrimaryKey,
					Columns:    []string{"order_id", "product_id"},
				},
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"order_id", "product_id"},
					RefTable:   "order_items",
					RefColumns: []string{"order_id", "product_id"},
					OnDelete:   "CASCADE",
				},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[1] // order_details is second

	if len(tbl.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(tbl.ForeignKeys))
	}

	fk := tbl.ForeignKeys[0]
	if !reflect.DeepEqual(fk.Columns, []string{"order_id", "product_id"}) {
		t.Errorf("FK columns: got %v", fk.Columns)
	}
	if fk.RefTable != "order_items" {
		t.Errorf("FK ref table: expected 'order_items', got %q", fk.RefTable)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("FK on delete: expected 'CASCADE', got %q", fk.OnDelete)
	}
}

func TestTranslate_UniqueConstraints(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "email", Type: ColumnType{BaseType: "varchar"}, IsUnique: true, NotNull: true},
				{Name: "username", Type: ColumnType{BaseType: "varchar"}, NotNull: true},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintUnique, Columns: []string{"username"}},
				{Type: ConstraintUnique, Columns: []string{"email"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	// email appears twice (inline + table-level), should be deduped
	// username is table-level
	expected := [][]string{{"email"}, {"username"}}
	if !reflect.DeepEqual(tbl.Unique, expected) {
		t.Errorf("expected uniques %v, got %v", expected, tbl.Unique)
	}
}

func TestTranslate_CompositeUnique(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "reviews",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
				{Name: "product_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintUnique, Columns: []string{"user_id", "product_id"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	expected := [][]string{{"user_id", "product_id"}}
	if !reflect.DeepEqual(tbl.Unique, expected) {
		t.Errorf("expected uniques %v, got %v", expected, tbl.Unique)
	}
}

func TestTranslate_UniqueCoveredByPK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintUnique, Columns: []string{"id"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if len(tbl.Unique) != 0 {
		t.Errorf("expected no uniques (covered by PK), got %v", tbl.Unique)
	}
}

func TestTranslate_ForeignKeyActions(t *testing.T) {
	actions := []struct {
		input string
		want  string
	}{
		{"RESTRICT", "RESTRICT"},
		{"CASCADE", "CASCADE"},
		{"SET NULL", "SET NULL"},
		{"SET DEFAULT", "SET DEFAULT"},
		{"NO ACTION", "NO ACTION"},
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
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if len(tbl.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(tbl.ForeignKeys))
	}
	if tbl.ForeignKeys[0].RefTable != "employees" {
		t.Errorf("self-ref FK: expected ref 'employees', got %q",
			tbl.ForeignKeys[0].RefTable)
	}
}

func TestTranslate_MultipleForeignKeys(t *testing.T) {
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
				{Name: "author_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
				{Name: "reviewer_id", Type: ColumnType{BaseType: "int"}},
			},
			TableConstraints: []TableConstraint{
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"author_id"},
					RefTable:   "users",
					RefColumns: []string{"id"},
				},
				{
					Type:       ConstraintForeignKey,
					Columns:    []string{"reviewer_id"},
					RefTable:   "users",
					RefColumns: []string{"id"},
				},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[1] // posts is second

	if len(tbl.ForeignKeys) != 2 {
		t.Fatalf("expected 2 FKs, got %d", len(tbl.ForeignKeys))
	}
}

func TestTranslate_EnumType(t *testing.T) {
	stmts := []Stmt{
		CreateEnumStmt{
			Name:   "mood",
			Values: []string{"happy", "sad", "neutral"},
		},
		CreateTableStmt{
			Name: "entries",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "feeling", Type: ColumnType{BaseType: "mood"}, NotNull: true},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}

	if len(s.Enums) != 1 {
		t.Fatalf("expected 1 enum type, got %d", len(s.Enums))
	}
	if s.Enums[0].Name != "mood" {
		t.Errorf("enum name: expected 'mood', got %q", s.Enums[0].Name)
	}
	if !reflect.DeepEqual(s.Enums[0].Values, []string{"happy", "sad", "neutral"}) {
		t.Errorf("enum values: got %v", s.Enums[0].Values)
	}

	tbl := s.Tables[0]
	if tbl.Columns[1].Type != "mood" {
		t.Errorf("enum column type: expected 'mood', got %q", tbl.Columns[1].Type)
	}
}

func TestTranslate_SchemaQualifiedName(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Schema: "billing",
			Name:   "invoices",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}

	if s.Tables[0].Name != "billing.invoices" {
		t.Errorf("expected 'billing.invoices', got %q", s.Tables[0].Name)
	}
}

func TestTranslate_NoConstraints(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "logs",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}},
				{Name: "message", Type: ColumnType{BaseType: "text"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if len(tbl.PrimaryKey) != 0 {
		t.Errorf("expected no PK, got %v", tbl.PrimaryKey)
	}
	if len(tbl.ForeignKeys) != 0 {
		t.Errorf("expected no FKs, got %v", tbl.ForeignKeys)
	}
	if len(tbl.Unique) != 0 {
		t.Errorf("expected no uniques, got %v", tbl.Unique)
	}
	if len(tbl.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(tbl.Columns))
	}

	if !tbl.Columns[0].Nullable || !tbl.Columns[1].Nullable {
		t.Error("expected both columns to be nullable")
	}
}

func TestTranslate_PreprocessSQL(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "CREATE TABLE t (id INT);",
			want:  []string{"CREATE TABLE t (id INT);"},
		},
		{
			input: "CREATE TABLE a (id INT);\nCREATE TABLE b (id INT);",
			want:  []string{"CREATE TABLE a (id INT);", "CREATE TABLE b (id INT);"},
		},
		{
			input: "-- comment\nCREATE TABLE t (id INT);",
			want:  []string{"CREATE TABLE t (id INT);"},
		},
		{
			input: "CREATE TABLE t (id INT /* inline comment */);",
			want:  []string{"CREATE TABLE t (id INT);"},
		},
		{
			input: "CREATE TABLE t (id INT);\n-- trailing comment",
			want:  []string{"CREATE TABLE t (id INT);"},
		},
	}

	for _, tt := range tests {
		got := preprocessSQL(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("preprocessSQL(%q)\n  got:  %v\n  want: %v", tt.input, got, tt.want)
		}
	}
}

func TestTranslate_TypeNormalizer(t *testing.T) {
	tests := []struct {
		input string
		want  AbstractType
	}{
		{"int", TypeInt},
		{"integer", TypeInt},
		{"bigint", TypeBigInt},
		{"smallint", TypeSmallInt},
		{"text", TypeText},
		{"varchar", TypeVarChar},
		{"boolean", TypeBoolean},
		{"decimal", TypeDecimal},
		{"numeric", TypeDecimal},
		{"real", TypeFloat},
		{"double precision", TypeDouble},
		{"date", TypeDate},
		{"timestamp", TypeTimestamp},
		{"uuid", TypeUUID},
		{"json", TypeJSON},
		{"jsonb", TypeJSONB},
		{"bytea", TypeBytea},
		{"foo", TypeUnknown},
	}

	for _, tt := range tests {
		got := NormalizeType(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTranslate_IsSerialType(t *testing.T) {
	serials := []string{"serial", "serial4", "serial2", "bigserial", "serial8", "smallserial"}
	for _, s := range serials {
		if !IsSerialType(s) {
			t.Errorf("IsSerialType(%q) = false, want true", s)
		}
	}
	if IsSerialType("int") {
		t.Error("IsSerialType('int') = true, want false")
	}
}

func TestTranslate_NamedConstraints(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}},
				{Name: "email", Type: ColumnType{BaseType: "varchar"}},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintPrimaryKey, Name: "pk_t", Columns: []string{"id"}},
				{Type: ConstraintUnique, Name: "uq_email", Columns: []string{"email"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if !reflect.DeepEqual(tbl.PrimaryKey, []string{"id"}) {
		t.Errorf("PK: expected [id], got %v", tbl.PrimaryKey)
	}
	if !reflect.DeepEqual(tbl.Unique, [][]string{{"email"}}) {
		t.Errorf("Unique: expected [[email]], got %v", tbl.Unique)
	}
}

func TestTranslate_DefaultExpressions(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "score", Type: ColumnType{BaseType: "int"}, Default: "0"},
				{Name: "label", Type: ColumnType{BaseType: "varchar"}, Default: "'untitled'"},
				{Name: "active", Type: ColumnType{BaseType: "boolean"}, Default: "false"},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	expectedDefaults := []struct {
		col    int
		hasDef bool
		val    string
	}{
		{0, false, ""},
		{1, true, "0"},
		{2, true, "'untitled'"},
		{3, true, "false"},
	}

	for _, e := range expectedDefaults {
		col := tbl.Columns[e.col]
		if e.hasDef {
			if col.Default == nil {
				t.Errorf("column %d: expected default %q, got nil", e.col, e.val)
			} else if *col.Default != e.val {
				t.Errorf("column %d: expected default %q, got %q", e.col, e.val, *col.Default)
			}
		} else if col.Default != nil {
			t.Errorf("column %d: expected no default, got %q", e.col, *col.Default)
		}
	}
}

func TestTranslate_OverlapUnique(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "version", Type: ColumnType{BaseType: "int"}, NotNull: true},
			},
			TableConstraints: []TableConstraint{
				{Type: ConstraintUnique, Columns: []string{"id", "version"}},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if len(tbl.Unique) != 1 {
		t.Errorf("expected 1 unique, got %d: %v", len(tbl.Unique), tbl.Unique)
	}
}

func TestTranslate_MultipleTablesWithFK(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "users",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "name", Type: ColumnType{BaseType: "varchar"}},
			},
		},
		CreateTableStmt{
			Name: "orders",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true},
				{Name: "user_id", Type: ColumnType{BaseType: "int"}, NotNull: true},
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
		t.Fatal(err)
	}

	if len(s.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(s.Tables))
	}
	if s.Tables[0].Name != "users" || s.Tables[1].Name != "orders" {
		t.Errorf("unexpected table order: %q, %q", s.Tables[0].Name, s.Tables[1].Name)
	}

	if len(s.Tables[1].ForeignKeys) != 1 {
		t.Errorf("orders: expected 1 FK, got %d", len(s.Tables[1].ForeignKeys))
	}
}

func TestTranslate_NotNullAndDefault(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "id", Type: ColumnType{BaseType: "int"}, IsPrimaryKey: true, NotNull: true},
				{Name: "name", Type: ColumnType{BaseType: "text"}},
				{Name: "email", Type: ColumnType{BaseType: "varchar"}, NotNull: true},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if tbl.Columns[0].Nullable {
		t.Error("id should not be nullable")
	}
	if !tbl.Columns[1].Nullable {
		t.Error("name should be nullable")
	}
	if tbl.Columns[2].Nullable {
		t.Error("email should not be nullable")
	}
}

func TestTranslate_UnknownTypeBecomesEnum(t *testing.T) {
	stmts := []Stmt{
		CreateTableStmt{
			Name: "t",
			Columns: []ColumnDef{
				{Name: "status", Type: ColumnType{BaseType: "order_status"}, NotNull: true},
			},
		},
	}
	s, err := Translate(stmts)
	if err != nil {
		t.Fatal(err)
	}
	tbl := s.Tables[0]

	if tbl.Columns[0].Type != "order_status" {
		t.Errorf("expected type 'order_status', got %q", tbl.Columns[0].Type)
	}
}
