package schema

import "testing"

func buildTableMap(model *Model) {
	model.TableMap = make(map[string]*Table, len(model.Tables))
	for _, t := range model.Tables {
		model.TableMap[t.Name] = t
	}
}

func TestValidate_ValidModel(t *testing.T) {
	model := &Model{
		Tables: []*Table{
			{
				Name: "users",
				Columns: []Column{
					{Name: "id", IsPrimaryKey: true},
					{Name: "email"},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	buildTableMap(model)

	if errors := Validate(model); errors != nil {
		t.Fatalf("expected no errors, got %v", errors)
	}
}

func TestValidate_DuplicateColumn(t *testing.T) {
	model := &Model{
		Tables: []*Table{
			{
				Name: "users",
				Columns: []Column{
					{Name: "id"},
					{Name: "id"},
				},
			},
		},
	}
	buildTableMap(model)

	errors := Validate(model)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
}

func TestValidate_MissingPKColumn(t *testing.T) {
	model := &Model{
		Tables: []*Table{
			{
				Name:       "users",
				Columns:    []Column{{Name: "id"}},
				PrimaryKey: []string{"nonexistent"},
			},
		},
	}
	buildTableMap(model)

	errors := Validate(model)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
}

func TestValidate_UnknownFKTable(t *testing.T) {
	model := &Model{
		Tables: []*Table{
			{
				Name:    "orders",
				Columns: []Column{{Name: "user_id"}},
				ForeignKeys: []ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}
	buildTableMap(model)

	errors := Validate(model)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
}

func TestValidate_UnknownFKRefColumn(t *testing.T) {
	model := &Model{
		Tables: []*Table{
			{
				Name:    "users",
				Columns: []Column{{Name: "id"}},
			},
			{
				Name:    "orders",
				Columns: []Column{{Name: "user_id"}},
				ForeignKeys: []ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"nonexistent"}},
				},
			},
		},
	}
	buildTableMap(model)

	errors := Validate(model)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
}

func TestValidate_UnknownFKColumn(t *testing.T) {
	model := &Model{
		Tables: []*Table{
			{
				Name:    "users",
				Columns: []Column{{Name: "id"}},
			},
			{
				Name:    "orders",
				Columns: []Column{{Name: "id"}},
				ForeignKeys: []ForeignKey{
					{Columns: []string{"missing_col"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}
	buildTableMap(model)

	errors := Validate(model)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
}
