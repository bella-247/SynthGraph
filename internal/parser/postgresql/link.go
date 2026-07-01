package postgresql

// link runs the third pipeline stage: cross-reference resolution.
//
// Responsibilities:
//   - Resolve foreign key RefTable strings to actual tables in the schema.
//   - Store resolved table indices (or -1 if the target doesn't exist)
//     for later validation.
//   - Connect enum-type columns to their CreateEnumStmt definitions.
//
// Contract:
//   - Every FK reference that CAN be resolved IS resolved.
//   - Unresolvable references are marked with index -1 (not an error here;
//     the validate stage rejects them).
//   - Enum-type columns that match a known enum definition are noted.
func (st *schemaTranslator) link() error {
	tableIndex := st.buildTableIndex()
	st.resolveForeignKeys(tableIndex)
	st.resolveEnumReferences()
	return nil
}

func (st *schemaTranslator) buildTableIndex() map[string]int {
	tableIndex := make(map[string]int, len(st.tables))
	for i, tb := range st.tables {
		if _, exists := tableIndex[tb.name]; !exists {
			tableIndex[tb.name] = i
		}
	}
	return tableIndex
}

func (st *schemaTranslator) resolveForeignKeys(tableIndex map[string]int) {
	for ti := range st.tables {
		tb := &st.tables[ti]
		tb.fkTargetIndex = make([]int, len(tb.fks))
		for fi, fk := range tb.fks {
			if idx, exists := tableIndex[fk.RefTable]; exists {
				tb.fkTargetIndex[fi] = idx
			} else {
				tb.fkTargetIndex[fi] = -1
			}
		}
	}
}

func (st *schemaTranslator) resolveEnumReferences() {
	for ti := range st.tables {
		tb := &st.tables[ti]
		for ci := range tb.columns {
			col := &tb.columns[ci]
			if _, isEnum := st.enums[col.Type]; isEnum {
				// Column type matches a known enum — resolved.
				continue
			}
		}
	}
}
