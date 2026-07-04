// Package validator checks a generated Dataset against all schema constraints
// and produces a ValidationResult listing every violation found.
//
// After the generator produces a Dataset, the validator ensures:
//   - NOT NULL columns contain no nil values
//   - Primary key values are unique and non-nil
//   - UNIQUE constraint columns have no duplicates
//   - Foreign key values reference existing primary keys
//
// # Architecture position
//
//	Parser → schema.Model → Graph → planner.BuildPlan → GenerationPlan
//	                                                          │
//	                                                          ▼
//	                                                    generator.Generate
//	                                                          │
//	                                                          ▼
//	                                                    Dataset
//	                                                          │
//	                                                          ▼
//	                                                    validator.Validate
//	                                                          │
//	                                                          ▼
//	                                                    ValidationResult
//	                                                          │
//	                                                          ▼
//	                                                    exporter.Export
package validator

import (
	"fmt"

	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// ValidationResult holds the outcome of validating a Dataset against
// all schema constraints.
type ValidationResult struct {
	// Valid is true when every constraint is satisfied.
	Valid bool

	// Errors lists every constraint violation found.
	Errors []ValidationError
}

// ValidationError describes a single constraint violation.
type ValidationError struct {
	// Table is the table containing the violation.
	Table string

	// Row is the 0-based row index, or -1 if the error affects the whole table.
	Row int

	// Column is the column containing the violation, or empty if table-scoped.
	Column string

	// Constraint identifies which constraint was violated.
	Constraint string

	// Message describes the exact violation.
	Message string
}

// Error implements the error interface so ValidationError can be used as an error.
func (ve *ValidationError) Error() string {
	msg := fmt.Sprintf("table %q", ve.Table)
	if ve.Row >= 0 {
		msg = fmt.Sprintf("%s row %d", msg, ve.Row)
	}
	if ve.Column != "" {
		msg = fmt.Sprintf("%s column %q", msg, ve.Column)
	}
	return fmt.Sprintf("%s: %s constraint violated: %s", msg, ve.Constraint, ve.Message)
}

// Validate checks every constraint in the schema model against the generated
// dataset. It aggregates all violations into a single ValidationResult.
//
// Checks performed:
//  1. NOT NULL — every column marked non-nullable must have a non-nil value.
//  2. Primary Key — PK columns must be non-nil and all PK value combinations
//     must be unique across rows.
//  3. UNIQUE — every column set declared UNIQUE must have no duplicate values.
//  4. Foreign Key — every FK column value must exist as a PK value in the
//     referenced table.
func Validate(dataset *generator.Dataset, model *schema.Model, schemaGraph *graph.Graph) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}

	// Build a PK index: table name → set of PK string representations.
	pkIndex := buildPKIndex(dataset, model)

	// Check each table.
	for _, tableDef := range model.Tables {
		generatedTable := findTable(dataset, tableDef.Name)
		if generatedTable == nil {
			result.addError(tableDef.Name, -1, "", "SCHEMA",
				fmt.Sprintf("table %q missing from generated dataset", tableDef.Name))
			continue
		}

		// 1. Check NOT NULL constraints.
		checkNotNull(generatedTable, tableDef, result)

		// 2. Check PRIMARY KEY constraints.
		checkPrimaryKey(generatedTable, tableDef, result)

		// 3. Check UNIQUE constraints.
		checkUnique(generatedTable, tableDef, result)

		// 4. Check FOREIGN KEY constraints.
		checkForeignKeys(generatedTable, tableDef, pkIndex, result)
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// addError appends a validation error to the result.
func (result *ValidationResult) addError(table string, row int, column, constraint, message string) {
	result.Errors = append(result.Errors, ValidationError{
		Table:      table,
		Row:        row,
		Column:     column,
		Constraint: constraint,
		Message:    message,
	})
}

// buildPKIndex builds a lookup table of primary key values for every table
// in the dataset. Keys are string representations of the PK; single-column
// PKs use their raw value's fmt.Sprintf, composite PKs use "::" joining.
func buildPKIndex(dataset *generator.Dataset, model *schema.Model) map[string]map[string]bool {
	index := make(map[string]map[string]bool)
	for _, tableDef := range model.Tables {
		if len(tableDef.PrimaryKey) == 0 {
			continue
		}
		generatedTable := findTable(dataset, tableDef.Name)
		if generatedTable == nil {
			continue
		}
		set := make(map[string]bool, len(generatedTable.Rows))
		for _, row := range generatedTable.Rows {
			key := pkString(row, tableDef.PrimaryKey)
			set[key] = true
		}
		index[tableDef.Name] = set
	}
	return index
}

// pkString produces a deterministic string key for a row's PK values.
func pkString(row generator.GeneratedRow, pkColumns []string) string {
	if len(pkColumns) == 1 {
		return fmt.Sprintf("%v", row[pkColumns[0]])
	}
	key := ""
	for i, col := range pkColumns {
		if i > 0 {
			key += "::"
		}
		key += fmt.Sprintf("%v", row[col])
	}
	return key
}

// findTable finds a GeneratedTable by name in the dataset, or nil.
func findTable(dataset *generator.Dataset, tableName string) *generator.GeneratedTable {
	for _, t := range dataset.Tables {
		if t.TableName == tableName {
			return t
		}
	}
	return nil
}
