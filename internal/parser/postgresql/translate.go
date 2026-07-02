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
	enumColumns   []int       // column indices that reference enums (populated by normalizer, consumed by linker)
	fkTargetIndex []int       // table index for each FK (populated by linker, -1 = unresolved) — see link.go

	// Intermediate constraint state: assembled from both column-level
	// (e.g. inline PRIMARY KEY) and table-level (e.g. PRIMARY KEY(a,b)) definitions.
	pkCols  []string
	fks     []schema.ForeignKey
	uniques [][]string
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
	st := newTranslator(stmts)
	st.normalize()
	if err := st.link(); err != nil {
		return nil, err
	}
	if err := st.validate(); err != nil {
		return nil, err
	}
	return st.build(), nil
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
	st := &schemaTranslator{
		enums: make(map[string]*schema.EnumType),
	}

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case CreateEnumStmt:
			et := &schema.EnumType{
				Name:   enumKey(s.Schema, s.Name),
				Values: s.Values,
			}
			st.enums[et.Name] = et

		case CreateTableStmt:
			tb := tableBuilder{
				name:       tableName(s.Schema, s.Name),
				rawColumns: s.Columns,
				pkCols:     extractInlinePK(s),
				uniques:    extractInlineUniques(s),
			}
			tb.fks, tb.pkCols, tb.uniques = extractTableConstraints(s, tb.pkCols, tb.uniques)
			st.tables = append(st.tables, tb)
		}
	}

	return st
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
	var pk []string
	for _, col := range stmt.Columns {
		if col.IsPrimaryKey {
			pk = append(pk, col.Name)
		}
	}
	return pk
}

// extractInlineUniques collects column-level UNIQUE flags into unique-constraint lists.
func extractInlineUniques(stmt CreateTableStmt) [][]string {
	var uniques [][]string
	for _, col := range stmt.Columns {
		if col.IsUnique {
			uniques = append(uniques, []string{col.Name})
		}
	}
	return uniques
}

// extractTableConstraints processes table-level constraints and merges them
// into the already-collected column-level constraint lists.
func extractTableConstraints(stmt CreateTableStmt, existingPK []string, existingUniques [][]string) (fks []schema.ForeignKey, pkCols []string, uniques [][]string) {
	pkCols = existingPK
	uniques = existingUniques

	for _, tc := range stmt.TableConstraints {
		switch tc.Type {
		case ConstraintPrimaryKey:
			pkCols = append(pkCols, tc.Columns...)

		case ConstraintForeignKey:
			fks = append(fks, schema.ForeignKey{
				Columns:    tc.Columns,
				RefTable:   tc.RefTable,
				RefColumns: tc.RefColumns,
				OnDelete:   schema.FKAction(tc.OnDelete),
				OnUpdate:   schema.FKAction(tc.OnUpdate),
			})

		case ConstraintUnique:
			uniques = append(uniques, tc.Columns)

		case ConstraintCheck:
			// V1: parsed, available for future validation
		}
	}

	return fks, pkCols, uniques
}

// dedupe removes duplicate strings preserving order.
func dedupe(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// dedupeUniques removes unique constraints already covered by the primary key.
func dedupeUniques(uniques [][]string, pk []string) [][]string {
	pkSet := make(map[string]bool, len(pk))
	for _, v := range pk {
		pkSet[v] = true
	}

	isPkCovered := func(cols []string) bool {
		for _, c := range cols {
			if !pkSet[c] {
				return false
			}
		}
		return len(cols) > 0
	}

	seen := make(map[string]bool)
	result := make([][]string, 0)

	for _, u := range uniques {
		if isPkCovered(u) {
			continue
		}
		key := strings.Join(u, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, u)
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
func (st *schemaTranslator) build() *schema.Model {
	m := &schema.Model{
		TableMap: make(map[string]*schema.Table, len(st.tables)),
	}

	// Collect enums
	for _, et := range st.enums {
		m.Enums = append(m.Enums, *et)
	}

	// Build each table
	for _, tb := range st.tables {
		t := &schema.Table{
			Name:    tb.name,
			Columns: tb.columns,
		}

		// Deduplicate PK columns (same column can appear in both inline and table-level)
		t.PrimaryKey = dedupe(tb.pkCols)

		// Mark PK columns in the Column struct
		pkSet := make(map[string]bool, len(t.PrimaryKey))
		for _, p := range t.PrimaryKey {
			pkSet[p] = true
		}
		for i := range t.Columns {
			if pkSet[t.Columns[i].Name] {
				t.Columns[i].IsPrimaryKey = true
			}
		}

		// Remove unique constraints covered by the PK
		t.Unique = dedupeUniques(tb.uniques, t.PrimaryKey)

		// Copy resolved foreign keys
		t.ForeignKeys = tb.fks

		m.Tables = append(m.Tables, t)
		m.TableMap[t.Name] = t
	}

	return m
}
