package postgresql

import (
	"strings"

	"synthgraph/internal/schema"
)

// schemaTranslator carries intermediate state through the translation pipeline.
// Each stage (normalize → link → validate → build) transforms this state
// toward the final immutable schema.Model.
//
// This is a combined Pipeline + Builder pattern:
//   - Pipeline: ordered stages, each with a clear contract
//   - Builder: accumulates state, .build() produces the final object
type schemaTranslator struct {
	enums  map[string]*schema.EnumType
	tables []tableBuilder
}

// tableBuilder holds the intermediate state for one table during translation.
// Constraints start as raw AST lists and are normalized/merged as the pipeline progresses.
type tableBuilder struct {
	name    string
	columns []schema.Column // populated by normalize stage

	// Raw column definitions from the AST — consumed by the normalize stage
	// to resolve types, nullability, defaults, etc.
	rawColumns    []ColumnDef
	fkTargetIndex []int // table index for each FK (populated by linker, -1 = unresolved)

	// Intermediate constraint state: assembled from both column-level
	// (e.g. inline PRIMARY KEY) and table-level (e.g. PRIMARY KEY(a,b)) definitions.
	pkCols  []string
	fks     []schema.ForeignKey
	uniques [][]string
	checks  []schema.CheckConstraint // preserved CHECK expressions
}

// Translate converts our PostgreSQL DDL AST into the canonical schema.Model.
//
// The pipeline executes five stages in order:
//
//	 1. Extract — build initial state from []Stmt
//	 2. Normalize — canonicalize types, merge constraints
//	 3. Link — resolve FK and enum cross-references
//	 4. Validate — check internal consistency
//	 5. Build — deduplicate, mark PK columns, produce final output
//
// Each stage has a clear contract (see individual file docs). If any stage fails,
// the pipeline halts and returns an error describing the violation.
func Translate(stmts []Stmt) (*schema.Model, error) {
	translator := newTranslator(stmts)
	translator.normalize()
	if err := translator.link(); err != nil {
		return nil, err
	}
	if err := translator.validate(); err != nil {
		return nil, err
	}
	return translator.build(), nil
}

// newTranslator initialises a schemaTranslator from the raw DDL AST.
// This is the "Extract" stage — a pure 1:1 copy with no normalisation.
//
// Contract:
//   - Every table from the AST exists in the translator.
//   - Every enum from the AST exists in the translator.
//   - Raw column definitions are preserved exactly as parsed.
//   - Constraint lists are assembled from both column-level and table-level
//     definitions but not yet deduplicated.
//   - No information is intentionally discarded.
func newTranslator(stmts []Stmt) *schemaTranslator {
	translator := &schemaTranslator{
		enums: make(map[string]*schema.EnumType),
	}

	for _, stmt := range stmts {
		switch typedStmt := stmt.(type) {
		case CreateEnumStmt:
			enumType := &schema.EnumType{
				Name:   enumKey(typedStmt.Schema, typedStmt.Name),
				Values: typedStmt.Values,
			}
			translator.enums[enumType.Name] = enumType

		case CreateTableStmt:
			table := tableBuilder{
				name:       tableName(typedStmt.Schema, typedStmt.Name),
				rawColumns: typedStmt.Columns,
				pkCols:     extractInlinePK(typedStmt),
				uniques:    extractInlineUniques(typedStmt),
			}
			table.fks, table.pkCols, table.uniques, table.checks = extractTableConstraints(typedStmt, table.pkCols, table.uniques)
			// Collect inline REFERENCES constraints (column-level FKs)
			table.fks = append(table.fks, extractInlineFKs(typedStmt)...)
			translator.tables = append(translator.tables, table)
		}
	}

	return translator
}

// enumKey builds a canonical enum name, schema-qualified if applicable.
func enumKey(schema, name string) string {
	if schema != "" {
		return schema + "." + name
	}
	return name
}

// tableName builds a canonical table name, schema-qualified if applicable.
func tableName(schema, name string) string {
	if schema != "" {
		return schema + "." + name
	}
	return name
}

// extractInlinePK collects column-level PRIMARY KEY flags into a PK column list.
func extractInlinePK(stmt CreateTableStmt) []string {
	var primaryKeys []string
	for _, column := range stmt.Columns {
		if column.IsPrimaryKey {
			primaryKeys = append(primaryKeys, column.Name)
		}
	}
	return primaryKeys
}

// extractInlineUniques collects column-level UNIQUE flags into unique-constraint lists.
func extractInlineUniques(stmt CreateTableStmt) [][]string {
	var uniques [][]string
	for _, column := range stmt.Columns {
		if column.IsUnique {
			uniques = append(uniques, []string{column.Name})
		}
	}
	return uniques
}

// extractTableConstraints processes table-level constraints and merges them
// into the already-collected column-level constraint lists.
func extractTableConstraints(stmt CreateTableStmt, existingPK []string, existingUniques [][]string) (fks []schema.ForeignKey, pkCols []string, uniques [][]string, checks []schema.CheckConstraint) {
	pkCols = existingPK
	uniques = existingUniques

	for _, tableConstraint := range stmt.TableConstraints {
		switch tableConstraint.Type {
		case ConstraintPrimaryKey:
			pkCols = append(pkCols, tableConstraint.Columns...)

		case ConstraintForeignKey:
			fks = append(fks, schema.ForeignKey{
				Columns:    tableConstraint.Columns,
				RefTable:   tableConstraint.RefTable,
				RefColumns: tableConstraint.RefColumns,
				OnDelete:   schema.FKAction(tableConstraint.OnDelete),
				OnUpdate:   schema.FKAction(tableConstraint.OnUpdate),
			})

		case ConstraintUnique:
			uniques = append(uniques, tableConstraint.Columns)

		case ConstraintCheck:
			checks = append(checks, schema.CheckConstraint{
				Name:       tableConstraint.Name,
				Expression: tableConstraint.CheckExpr,
			})
		}
	}

	return fks, pkCols, uniques, checks
}

// extractInlineFKs collects column-level REFERENCES constraints and returns
// them as schema.ForeignKey entries, which are equivalent to table-level FKs.
func extractInlineFKs(stmt CreateTableStmt) []schema.ForeignKey {
	var foreignKeys []schema.ForeignKey
	for _, column := range stmt.Columns {
		if column.References != nil {
			foreignKeys = append(foreignKeys, schema.ForeignKey{
				Columns:    []string{column.Name},
				RefTable:   column.References.RefTable,
				RefColumns: column.References.RefColumns,
				OnDelete:   schema.FKAction(column.References.OnDelete),
				OnUpdate:   schema.FKAction(column.References.OnUpdate),
			})
		}
	}
	return foreignKeys
}

// dedupe removes duplicate strings while preserving order.
func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	return unique
}

// dedupeUniques removes unique constraints already covered by the primary key.
func dedupeUniques(uniques [][]string, primaryKey []string) [][]string {
	primaryKeySet := make(map[string]bool, len(primaryKey))
	for _, column := range primaryKey {
		primaryKeySet[column] = true
	}

	isCoveredByPrimaryKey := func(columns []string) bool {
		for _, column := range columns {
			if !primaryKeySet[column] {
				return false
			}
		}
		return len(columns) > 0
	}

	seen := make(map[string]bool)
	result := make([][]string, 0)

	for _, constraint := range uniques {
		if isCoveredByPrimaryKey(constraint) {
			continue
		}
		key := strings.Join(constraint, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, constraint)
	}

	return result
}

// build produces the final immutable schema.Model from the pipeline state.
// This is the final pipeline stage.
//
// Contract:
//   - PK columns are deduplicated and marked on the Column struct.
//   - Unique constraints that are fully covered by the PK are removed.
//   - All remaining constraints are in final form.
//   - TableMap provides O(1) lookup by table name.
func (translator *schemaTranslator) build() *schema.Model {
	model := &schema.Model{
		TableMap: make(map[string]*schema.Table, len(translator.tables)),
	}

	// Collect enums
	for _, enumType := range translator.enums {
		model.Enums = append(model.Enums, *enumType)
	}

	// Build each table
	for _, tableBuilder := range translator.tables {
		table := &schema.Table{
			Name:    tableBuilder.name,
			Columns: tableBuilder.columns,
		}

		// Deduplicate PK columns (same column can appear in both inline and table-level)
		table.PrimaryKey = dedupe(tableBuilder.pkCols)

		// Mark PK columns in the Column struct
		primaryKeySet := make(map[string]bool, len(table.PrimaryKey))
		for _, primaryKeyColumn := range table.PrimaryKey {
			primaryKeySet[primaryKeyColumn] = true
		}
		for columnIndex := range table.Columns {
			if primaryKeySet[table.Columns[columnIndex].Name] {
				table.Columns[columnIndex].IsPrimaryKey = true
			}
		}

		// Remove unique constraints covered by the PK
		table.Unique = dedupeUniques(tableBuilder.uniques, table.PrimaryKey)

		// Copy resolved foreign keys
		table.ForeignKeys = tableBuilder.fks

		// Copy preserved CHECK constraints
		table.Checks = tableBuilder.checks

		model.Tables = append(model.Tables, table)
		model.TableMap[table.Name] = table
	}

	return model
}
