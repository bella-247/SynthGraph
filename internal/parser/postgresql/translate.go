package postgresql

import "synthgraph/internal/schema"

// schemaTranslator carries intermediate state through the translation pipeline.
// Each stage (normalize → link → validate → build) transforms this state
// toward the final immutable schema.Model.
type schemaTranslator struct {
	enums  map[string]*schema.EnumType
	tables []tableBuilder
}

// tableBuilder holds the intermediate state for one table during translation.
type tableBuilder struct {
	name          string
	columns       []schema.Column // populated by normalize stage
	rawColumns    []ColumnDef
	fkTargetIndex []int // table index for each FK (populated by linker, -1 = unresolved)
	pkCols        []string
	fks           []schema.ForeignKey
	uniques       [][]string
	checks        []schema.CheckConstraint
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
			table.fks = append(table.fks, extractInlineFKs(typedStmt)...)
			translator.tables = append(translator.tables, table)
		}
	}

	return translator
}

// build produces the final immutable schema.Model from the pipeline state.
// This is the final pipeline stage.
func (translator *schemaTranslator) build() *schema.Model {
	model := &schema.Model{
		TableMap: make(map[string]*schema.Table, len(translator.tables)),
	}

	for _, enumType := range translator.enums {
		model.Enums = append(model.Enums, *enumType)
	}

	for _, tableBuilder := range translator.tables {
		table := &schema.Table{
			Name:    tableBuilder.name,
			Columns: tableBuilder.columns,
		}

		table.PrimaryKey = dedupe(tableBuilder.pkCols)

		primaryKeySet := make(map[string]bool, len(table.PrimaryKey))
		for _, primaryKeyColumn := range table.PrimaryKey {
			primaryKeySet[primaryKeyColumn] = true
		}
		for columnIndex := range table.Columns {
			if primaryKeySet[table.Columns[columnIndex].Name] {
				table.Columns[columnIndex].IsPrimaryKey = true
				table.Columns[columnIndex].Nullable = false
			}
		}

		table.Unique = dedupeUniques(tableBuilder.uniques, table.PrimaryKey)
		table.ForeignKeys = tableBuilder.fks
		table.Checks = tableBuilder.checks

		model.Tables = append(model.Tables, table)
		model.TableMap[table.Name] = table
	}

	return model
}
