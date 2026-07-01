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
func (translator *schemaTranslator) validate() error {
	if err := translator.validateTableNames(); err != nil {
		return err
	}

	for tableIndex := range translator.tables {
		if err := translator.validateTable(&translator.tables[tableIndex]); err != nil {
			return err
		}
	}

	return nil
}

func (translator *schemaTranslator) validateTableNames() error {
	seenTables := make(map[string]bool, len(translator.tables))
	for index, table := range translator.tables {
		if table.name == "" {
			return fmt.Errorf("table at index %d has empty name", index)
		}
		if seenTables[table.name] {
			return fmt.Errorf("duplicate table name: %q", table.name)
		}
		seenTables[table.name] = true
	}
	return nil
}

func (translator *schemaTranslator) validateTable(table *tableBuilder) error {
	columnIndex, err := translator.buildColumnIndex(table)
	if err != nil {
		return err
	}

	if err := translator.validatePrimaryKeys(table, columnIndex); err != nil {
		return err
	}
	if err := translator.validateUniqueConstraints(table, columnIndex); err != nil {
		return err
	}
	if err := translator.validateForeignKeys(table); err != nil {
		return err
	}
	if err := translator.validateEnumColumns(table); err != nil {
		return err
	}

	return nil
}

func (translator *schemaTranslator) buildColumnIndex(table *tableBuilder) (map[string]int, error) {
	columnIndex := make(map[string]int, len(table.columns))
	for index, column := range table.columns {
		if _, duplicate := columnIndex[column.Name]; duplicate {
			return nil, fmt.Errorf("table %q: duplicate column name %q", table.name, column.Name)
		}
		columnIndex[column.Name] = index
	}
	return columnIndex, nil
}

func (translator *schemaTranslator) validatePrimaryKeys(table *tableBuilder, columnIndex map[string]int) error {
	for _, primaryKey := range table.pkCols {
		if _, exists := columnIndex[primaryKey]; !exists {
			return fmt.Errorf("table %q: primary key column %q does not exist in table", table.name, primaryKey)
		}
	}
	return nil
}

func (translator *schemaTranslator) validateUniqueConstraints(table *tableBuilder, columnIndex map[string]int) error {
	for _, uniqueConstraint := range table.uniques {
		for _, column := range uniqueConstraint {
			if _, exists := columnIndex[column]; !exists {
				return fmt.Errorf("table %q: unique column %q does not exist in table", table.name, column)
			}
		}
	}
	return nil
}

func (translator *schemaTranslator) validateForeignKeys(table *tableBuilder) error {
	for foreignKeyIndex, foreignKey := range table.fks {
		targetIndex := table.fkTargetIndex[foreignKeyIndex]
		if targetIndex == -1 {
			return fmt.Errorf("table %q: foreign key %v references unknown table %q", table.name, foreignKey.Columns, foreignKey.RefTable)
		}
		
		targetTable := &translator.tables[targetIndex]
		targetColumns := make(map[string]bool, len(targetTable.columns))
		for _, column := range targetTable.columns {
			targetColumns[column.Name] = true
		}

		for _, refColumn := range foreignKey.RefColumns {
			if !targetColumns[refColumn] {
				return fmt.Errorf("table %q: foreign key %v references column %q in table %q, which does not exist",
					table.name, foreignKey.Columns, refColumn, foreignKey.RefTable)
			}
		}
	}
	return nil
}

func (translator *schemaTranslator) validateEnumColumns(table *tableBuilder) error {
	for _, column := range table.columns {
		if isEnumRef(column) {
			if _, exists := translator.enums[column.Type]; !exists {
				return fmt.Errorf("table %q: column %q references undefined enum type %q",
					table.name, column.Name, column.Type)
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
