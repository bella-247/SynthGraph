package postgresql

import (
	"fmt"

	"synthgraph/internal/schema"
)

// validate runs the fourth pipeline stage: consistency checking.
//
// Responsibilities:
//   - Detect duplicate table names.
//   - Detect duplicate column names within a table.
//   - Verify that every PK column exists in the table's column list.
//   - Verify that every UNIQUE column exists in the table's column list.
//   - Verify that FK target columns exist in the referenced table.
//   - Verify that enum column types resolve to a known enum definition.
//
// Contract:
//   - A passing validate means the schema is internally consistent.
//   - All violations produce a specific, actionable error message.
//   - No further validation is needed after this stage.
func (st *schemaTranslator) validate() error {
	if err := st.validateTableNames(); err != nil {
		return err
	}

	for ti := range st.tables {
		if err := st.validateTable(&st.tables[ti]); err != nil {
			return err
		}
	}

	return nil
}

func (st *schemaTranslator) validateTableNames() error {
	seenTables := make(map[string]bool, len(st.tables))
	for i, tb := range st.tables {
		if tb.name == "" {
			return fmt.Errorf("table at index %d has empty name", i)
		}
		if seenTables[tb.name] {
			return fmt.Errorf("duplicate table name: %q", tb.name)
		}
		seenTables[tb.name] = true
	}
	return nil
}

func (st *schemaTranslator) validateTable(tb *tableBuilder) error {
	colIndex, err := st.buildColumnIndex(tb)
	if err != nil {
		return err
	}

	if err := st.validatePrimaryKeys(tb, colIndex); err != nil {
		return err
	}
	if err := st.validateUniqueConstraints(tb, colIndex); err != nil {
		return err
	}
	if err := st.validateForeignKeys(tb); err != nil {
		return err
	}
	if err := st.validateEnumColumns(tb); err != nil {
		return err
	}

	return nil
}

func (st *schemaTranslator) buildColumnIndex(tb *tableBuilder) (map[string]int, error) {
	colIndex := make(map[string]int, len(tb.columns))
	for ci, col := range tb.columns {
		if _, dup := colIndex[col.Name]; dup {
			return nil, fmt.Errorf("table %q: duplicate column name %q", tb.name, col.Name)
		}
		colIndex[col.Name] = ci
	}
	return colIndex, nil
}

func (st *schemaTranslator) validatePrimaryKeys(tb *tableBuilder, colIndex map[string]int) error {
	for _, pk := range tb.pkCols {
		if _, exists := colIndex[pk]; !exists {
			return fmt.Errorf("table %q: primary key column %q does not exist in table", tb.name, pk)
		}
	}
	return nil
}

func (st *schemaTranslator) validateUniqueConstraints(tb *tableBuilder, colIndex map[string]int) error {
	for _, u := range tb.uniques {
		for _, col := range u {
			if _, exists := colIndex[col]; !exists {
				return fmt.Errorf("table %q: unique column %q does not exist in table", tb.name, col)
			}
		}
	}
	return nil
}

func (st *schemaTranslator) validateForeignKeys(tb *tableBuilder) error {
	for fi, fk := range tb.fks {
		targetIdx := tb.fkTargetIndex[fi]
		if targetIdx == -1 {
			return fmt.Errorf("table %q: foreign key %v references unknown table %q", tb.name, fk.Columns, fk.RefTable)
		}
		
		target := &st.tables[targetIdx]
		targetCols := make(map[string]bool, len(target.columns))
		for _, c := range target.columns {
			targetCols[c.Name] = true
		}

		for _, rc := range fk.RefColumns {
			if !targetCols[rc] {
				return fmt.Errorf("table %q: foreign key %v references column %q in table %q, which does not exist",
					tb.name, fk.Columns, rc, fk.RefTable)
			}
		}
	}
	return nil
}

func (st *schemaTranslator) validateEnumColumns(tb *tableBuilder) error {
	for _, col := range tb.columns {
		if isEnumRef(col) {
			if _, exists := st.enums[col.Type]; !exists {
				return fmt.Errorf("table %q: column %q references undefined enum type %q",
					tb.name, col.Name, col.Type)
			}
		}
	}
	return nil
}

// isEnumRef returns true if the column type looks like an enum reference
// (not a built-in type, not empty).
func isEnumRef(col schema.Column) bool {
	if col.Type == "" {
		return false
	}
	return NormalizeType(col.Type) == TypeUnknown
}
