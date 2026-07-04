package schema

import "fmt"

// ValidationError describes a single schema-level issue found during
// pre-flight validation.
type ValidationError struct {
	Table   string
	Message string
}

// Error implements the error interface.
func (v *ValidationError) Error() string {
	msg := "schema"
	if v.Table != "" {
		msg = fmt.Sprintf("table %q", v.Table)
	}
	return fmt.Sprintf("%s: %s", msg, v.Message)
}

// Validate checks the model for internal consistency before generation.
// It returns all issues found, or nil if the model is valid.
//
// Checks performed:
//  1. No duplicate column names within a table.
//  2. Every primary key column name references an existing column.
//  3. Every UNIQUE constraint column references an existing column.
//  4. Every FK references an existing table.
//  5. Every FK columns reference existing columns in the referenced table.
func Validate(model *Model) []ValidationError {
	var errors []ValidationError

	for _, table := range model.Tables {
		if table == nil {
			continue
		}

		columnSet := make(map[string]bool, len(table.Columns))
		for _, col := range table.Columns {
			if columnSet[col.Name] {
				errors = append(errors, ValidationError{
					Table:   table.Name,
					Message: fmt.Sprintf("duplicate column name %q", col.Name),
				})
			}
			columnSet[col.Name] = true
		}

		for _, pkCol := range table.PrimaryKey {
			if !columnSet[pkCol] {
				errors = append(errors, ValidationError{
					Table:   table.Name,
					Message: fmt.Sprintf("primary key column %q does not exist", pkCol),
				})
			}
		}

		for _, uniqueGroup := range table.Unique {
			for _, col := range uniqueGroup {
				if !columnSet[col] {
					errors = append(errors, ValidationError{
						Table:   table.Name,
						Message: fmt.Sprintf("UNIQUE constraint references unknown column %q", col),
					})
				}
			}
		}

		for _, fk := range table.ForeignKeys {
			refTable := model.TableMap[fk.RefTable]
			if refTable == nil {
				errors = append(errors, ValidationError{
					Table:   table.Name,
					Message: fmt.Sprintf("foreign key references unknown table %q", fk.RefTable),
				})
				continue
			}

			refColumnSet := make(map[string]bool, len(refTable.Columns))
			for _, col := range refTable.Columns {
				refColumnSet[col.Name] = true
			}

			for _, fkCol := range fk.Columns {
				if !columnSet[fkCol] {
					errors = append(errors, ValidationError{
						Table:   table.Name,
						Message: fmt.Sprintf("foreign key column %q does not exist in table", fkCol),
					})
				}
			}

			for _, refCol := range fk.RefColumns {
				if !refColumnSet[refCol] {
					errors = append(errors, ValidationError{
						Table:   table.Name,
						Message: fmt.Sprintf("foreign key references unknown column %q in table %q", refCol, fk.RefTable),
					})
				}
			}
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}
