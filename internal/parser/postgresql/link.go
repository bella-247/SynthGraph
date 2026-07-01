package postgresql

// link runs the third pipeline stage: cross-reference resolution.
//
// Responsibilities:
//   - Resolve foreign key RefTable strings to actual tables in the schema.
//   - Store resolved table indices (or -1 if the target doesn't exist)
//     for later validation.
//
// Contract:
//   - Every FK reference that CAN be resolved IS resolved.
//   - Unresolvable references are marked with index -1 (not an error here;
//     the validate stage rejects them).
func (translator *schemaTranslator) link() error {
	tableIndex := translator.buildTableIndex()
	translator.resolveForeignKeys(tableIndex)
	return nil
}

func (translator *schemaTranslator) buildTableIndex() map[string]int {
	tableIndex := make(map[string]int, len(translator.tables))
	for index, table := range translator.tables {
		if _, exists := tableIndex[table.name]; !exists {
			tableIndex[table.name] = index
		}
	}
	return tableIndex
}

func (translator *schemaTranslator) resolveForeignKeys(tableIndex map[string]int) {
	for tableIndexPosition := range translator.tables {
		table := &translator.tables[tableIndexPosition]
		table.fkTargetIndex = make([]int, len(table.fks))
		
		for foreignKeyIndex, foreignKey := range table.fks {
			if targetIndex, exists := tableIndex[foreignKey.RefTable]; exists {
				table.fkTargetIndex[foreignKeyIndex] = targetIndex
			} else {
				table.fkTargetIndex[foreignKeyIndex] = -1
			}
		}
	}
}
