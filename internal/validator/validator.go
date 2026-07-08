// Package validator validates a generated Dataset against its schema Model
// after generation completes, catching any constraint violations that may have
// been introduced by the generator or data pipeline.
//
// This is the post-generation gate: every row is checked against the schema's
// constraints before it reaches the exporter. Violations are reported as a
// list of ValidationError values — a non-empty list means the dataset should
// not be exported.
//
// # Architecture position
//
//	Generator → Dataset → Validator → ValidatedDataset → Exporter
//
// # Checks performed
//
//  1. NOT NULL — every non-nullable column value is non-nil.
//  2. Primary key uniqueness — no duplicate PK values within a table.
//  3. UNIQUE constraint uniqueness — no duplicate values in unique columns.
//  4. Enum validity — string values in enum-type columns match the enum definition.
//  5. Length limits — string values do not exceed column VARCHAR/nvarchar length.
//  6. FK referential integrity — every FK value references a PK in the target table.
package validator

import (
	"fmt"
	"strings"

	"synthgraph/internal/generator"
	"synthgraph/internal/schema"
)

// ValidationError describes a single constraint violation found during
// post-generation validation.
type ValidationError struct {
	// Table is the table containing the violation.
	Table string `json:"table"`

	// Column is the column containing the violation, or empty for table-level issues.
	Column string `json:"column,omitempty"`

	// RowIndex is the 0-based row index of the violation, or -1 if not row-specific.
	RowIndex int `json:"row_index,omitempty"`

	// Value is the actual value that caused the violation, if applicable.
	Value any `json:"value,omitempty"`

	// Rule identifies the constraint that was violated.
	Rule string `json:"rule"`

	// Message is a human-readable description of the violation.
	Message string `json:"message"`
}

// Error implements the error interface for use in error collections.
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validator: table %q%s: [%s] %s",
		ve.Table,
		formatRowCol(ve),
		ve.Rule,
		ve.Message,
	)
}

func formatRowCol(ve *ValidationError) string {
	var parts []string
	if ve.Column != "" {
		parts = append(parts, fmt.Sprintf("column %q", ve.Column))
	}
	if ve.RowIndex >= 0 {
		parts = append(parts, fmt.Sprintf("row %d", ve.RowIndex))
	}
	if len(parts) > 0 {
		return " " + strings.Join(parts, " ")
	}
	return ""
}

// Validate checks every table in the dataset against the schema model and
// returns all constraint violations found. Returns nil when the dataset is valid.
//
// Validation is purely additive — it never modifies the dataset or model.
// Callers should inspect the returned slice length: len == 0 means valid.
func Validate(dataset *generator.Dataset, model *schema.Model) []ValidationError {
	var errs []ValidationError

	if dataset == nil {
		errs = append(errs, ValidationError{
			Rule:    "INTERNAL",
			Message: "dataset is nil",
		})
		return errs
	}

	if model == nil {
		errs = append(errs, ValidationError{
			Rule:    "INTERNAL",
			Message: "model is nil",
		})
		return errs
	}

	// Build a lookup of referenced table PK values for FK validation.
	refPKs := buildRefPKMap(dataset, model)

	for _, table := range dataset.Tables {
		if table == nil {
			continue
		}

		schemaTable := model.TableMap[table.TableName]
		if schemaTable == nil {
			errs = append(errs, ValidationError{
				Table: table.TableName,
				Rule:  "INTERNAL",
				Message: fmt.Sprintf("table %q not found in schema model", table.TableName),
			})
			continue
		}

		// Build column lookup map for the schema table.
		colMap := make(map[string]*schema.Column, len(schemaTable.Columns))
		for i := range schemaTable.Columns {
			colMap[schemaTable.Columns[i].Name] = &schemaTable.Columns[i]
		}

		// ── NOT NULL checks ────────────────────────────────────────────────
		errs = append(errs, checkNotNull(table, schemaTable, colMap)...)

		// ── Primary key uniqueness ─────────────────────────────────────────
		errs = append(errs, checkPKUnique(table, schemaTable)...)

		// ── UNIQUE constraint uniqueness ───────────────────────────────────
		errs = append(errs, checkUniqueConstraints(table, schemaTable)...)

		// ── Enum validity ──────────────────────────────────────────────────
		errs = append(errs, checkEnumValues(table, schemaTable, colMap, model)...)

		// ── Length constraints ─────────────────────────────────────────────
		errs = append(errs, checkLengthConstraints(table, schemaTable, colMap)...)

		// ── FK referential integrity ───────────────────────────────────────
		errs = append(errs, checkFKRefs(table, schemaTable, refPKs)...)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// ── Individual checks ─────────────────────────────────────────────────────────

// checkNotNull verifies that non-nullable columns do not contain nil values.
func checkNotNull(table *generator.GeneratedTable, schemaTable *schema.Table, colMap map[string]*schema.Column) []ValidationError {
	var errs []ValidationError

	for rowIdx, row := range table.Rows {
		for _, col := range schemaTable.Columns {
			if col.Nullable {
				continue
			}
			val, ok := row[col.Name]
			if !ok {
				errs = append(errs, ValidationError{
					Table:    table.TableName,
					Column:   col.Name,
					RowIndex: rowIdx,
					Rule:     "NOT_NULL",
					Message:  fmt.Sprintf("column %q is NOT NULL but row %d has no value", col.Name, rowIdx),
				})
				continue
			}
			if val == nil {
				errs = append(errs, ValidationError{
					Table:    table.TableName,
					Column:   col.Name,
					RowIndex: rowIdx,
					Value:    val,
					Rule:     "NOT_NULL",
					Message:  fmt.Sprintf("column %q is NOT NULL but row %d has nil value", col.Name, rowIdx),
				})
			}
		}
	}
	return errs
}

// checkPKUnique verifies that primary key values are unique across all rows.
func checkPKUnique(table *generator.GeneratedTable, schemaTable *schema.Table) []ValidationError {
	if len(schemaTable.PrimaryKey) == 0 {
		return nil
	}

	var errs []ValidationError
	seen := make(map[string]int) // serialized key -> first row index

	for rowIdx, row := range table.Rows {
		pkKey := serializePK(row, schemaTable.PrimaryKey)
		if firstRow, dup := seen[pkKey]; dup {
			errs = append(errs, ValidationError{
				Table:    table.TableName,
				RowIndex: rowIdx,
				Rule:     "PK_UNIQUE",
				Value:    pkKey,
				Message:  fmt.Sprintf("duplicate primary key %q (first seen at row %d)", pkKey, firstRow),
			})
		} else {
			seen[pkKey] = rowIdx
		}
	}
	return errs
}

// checkUniqueConstraints verifies that UNIQUE-constrained columns have no duplicates.
func checkUniqueConstraints(table *generator.GeneratedTable, schemaTable *schema.Table) []ValidationError {
	var errs []ValidationError

	for _, uniqueGroup := range schemaTable.Unique {
		seen := make(map[string]int) // serialized value -> first row index
		for rowIdx, row := range table.Rows {
			key := serializeUniqueGroup(row, uniqueGroup)
			if firstRow, dup := seen[key]; dup {
				errs = append(errs, ValidationError{
					Table:    table.TableName,
					RowIndex: rowIdx,
					Rule:     "UNIQUE",
					Value:    key,
					Message:  fmt.Sprintf("duplicate unique value %q (first seen at row %d)", key, firstRow),
				})
			} else {
				seen[key] = rowIdx
			}
		}
	}
	return errs
}

// checkEnumValues verifies that values in enum-type columns are valid.
func checkEnumValues(table *generator.GeneratedTable, schemaTable *schema.Table, colMap map[string]*schema.Column, model *schema.Model) []ValidationError {
	var errs []ValidationError

	// Build enum type -> valid values lookup.
	enumDefs := make(map[string]map[string]bool, len(model.Enums))
	for _, enumType := range model.Enums {
		vals := make(map[string]bool, len(enumType.Values))
		for _, v := range enumType.Values {
			vals[v] = true
		}
		enumDefs[enumType.Name] = vals
	}

	for rowIdx, row := range table.Rows {
		for _, col := range schemaTable.Columns {
			// Check if this column's type references an enum.
			validVals, isEnum := enumDefs[col.Type]
			if !isEnum {
				continue
			}
			val, ok := row[col.Name]
			if !ok || val == nil {
				continue
			}
			strVal, ok := val.(string)
			if !ok {
				errs = append(errs, ValidationError{
					Table:    table.TableName,
					Column:   col.Name,
					RowIndex: rowIdx,
					Value:    val,
					Rule:     "ENUM",
					Message:  fmt.Sprintf("expected string value for enum column %q, got %T", col.Name, val),
				})
				continue
			}
			if !validVals[strVal] {
				errs = append(errs, ValidationError{
					Table:    table.TableName,
					Column:   col.Name,
					RowIndex: rowIdx,
					Value:    strVal,
					Rule:     "ENUM",
					Message:  fmt.Sprintf("value %q is not a valid enum value for type %q", strVal, col.Type),
				})
			}
		}
	}
	return errs
}

// checkLengthConstraints verifies that string values don't exceed column length.
func checkLengthConstraints(table *generator.GeneratedTable, schemaTable *schema.Table, colMap map[string]*schema.Column) []ValidationError {
	var errs []ValidationError

	for rowIdx, row := range table.Rows {
		for _, col := range schemaTable.Columns {
			if col.Length <= 0 {
				continue
			}
			// Only check string types with a length limit.
			if col.Type == "decimal" || col.Type == "numeric" {
				continue // Length on these is digit count, not string length
			}
			val, ok := row[col.Name]
			if !ok || val == nil {
				continue
			}
			strVal, ok := val.(string)
			if !ok {
				continue
			}
			if len(strVal) > col.Length {
				errs = append(errs, ValidationError{
					Table:    table.TableName,
					Column:   col.Name,
					RowIndex: rowIdx,
					Value:    strVal,
					Rule:     "LENGTH",
					Message:  fmt.Sprintf("string length %d exceeds column limit %d", len(strVal), col.Length),
				})
			}
		}
	}
	return errs
}

// checkFKRefs verifies that FK values reference existing PK values.
func checkFKRefs(table *generator.GeneratedTable, schemaTable *schema.Table, refPKs map[string]map[string]bool) []ValidationError {
	var errs []ValidationError

	for _, fk := range schemaTable.ForeignKeys {
		targetKey := fk.RefTable + "." + strings.Join(fk.RefColumns, ",")
		pkSet, ok := refPKs[targetKey]
		if !ok {
			continue
		}

		for rowIdx, row := range table.Rows {
			// Composite FK: serialize all FK columns together and check as one key.
			if len(fk.Columns) > 1 {
				key := serializeFKGroup(row, fk.Columns)
				if !pkSet[key] {
					errs = append(errs, ValidationError{
						Table:    table.TableName,
						RowIndex: rowIdx,
						Rule:     "FK",
						Value:    key,
						Message:  fmt.Sprintf("foreign key composite value %q not found in referenced table %q PK values", key, fk.RefTable),
					})
				}
				continue
			}
			// Single FK column: check each nullable column individually.
			for _, fkCol := range fk.Columns {
				val, ok := row[fkCol]
				if !ok || val == nil {
					continue
				}
				serialized := fmt.Sprintf("%v", val)
				if !pkSet[serialized] {
					errs = append(errs, ValidationError{
						Table:    table.TableName,
						Column:   fkCol,
						RowIndex: rowIdx,
						Value:    val,
						Rule:     "FK",
						Message:  fmt.Sprintf("foreign key value %q not found in referenced table %q PK values", serialized, fk.RefTable),
					})
				}
			}
		}
	}
	return errs
}

// serializeFKGroup produces a string key for a group of FK columns, matching
// the serialization format used for composite primary keys.
func serializeFKGroup(row generator.GeneratedRow, cols []string) string {
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "::")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// serializePK produces a string key for a composite primary key.
func serializePK(row generator.GeneratedRow, pkCols []string) string {
	parts := make([]string, len(pkCols))
	for i, col := range pkCols {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "::")
}

// serializeUniqueGroup produces a string key for a group of unique columns.
func serializeUniqueGroup(row generator.GeneratedRow, group []string) string {
	parts := make([]string, len(group))
	for i, col := range group {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "::")
}

// buildRefPKMap builds a map of "table.column -> set of PK values" for FK validation.
// Only tables present in the dataset are included.
func buildRefPKMap(dataset *generator.Dataset, model *schema.Model) map[string]map[string]bool {
	refPKs := make(map[string]map[string]bool)

	for _, table := range dataset.Tables {
		if table == nil {
			continue
		}
		schemaTable := model.TableMap[table.TableName]
		if schemaTable == nil || len(schemaTable.PrimaryKey) == 0 {
			continue
		}

		key := table.TableName + "." + strings.Join(schemaTable.PrimaryKey, ",")
		pkSet := make(map[string]bool, len(table.Rows))
		for _, row := range table.Rows {
			pkSet[serializePK(row, schemaTable.PrimaryKey)] = true
		}
		refPKs[key] = pkSet
	}
	return refPKs
}
