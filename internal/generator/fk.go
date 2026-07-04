package generator

import (
	"fmt"
	"strings"

	"synthgraph/internal/graph"
	"synthgraph/internal/schema"
)

// FKRefInfo describes a single FK column and its referenced table+column.
type FKRefInfo struct {
	Column     string
	RefTable   string
	RefColumns []string
}

// buildFKColumnMap builds a map from table name → list of FK ref info.
// Uses the graph's EdgeKindReferences edges for metadata.
func buildFKColumnMap(schemaGraph *graph.Graph) map[string][]FKRefInfo {
	fkMap := make(map[string][]FKRefInfo)

	for _, edge := range schemaGraph.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}
		fromNode, exists := schemaGraph.Nodes[edge.From]
		if !exists {
			continue
		}
		tableData, ok := fromNode.Data.(graph.TableData)
		if !ok {
			continue
		}
		toNode, exists := schemaGraph.Nodes[edge.To]
		if !exists {
			continue
		}
		toData, ok := toNode.Data.(graph.TableData)
		if !ok {
			continue
		}
		fkMeta, ok := edge.Metadata.(*graph.FKMetadata)
		if !ok {
			continue
		}
		for _, col := range fkMeta.LocalColumns {
			fkMap[tableData.Name] = append(fkMap[tableData.Name], FKRefInfo{
				Column:     col,
				RefTable:   toData.Name,
				RefColumns: fkMeta.ForeignColumns,
			})
		}
	}

	return fkMap
}

// isFKColumn checks if the given column name is an FK column and returns its ref info.
func isFKColumn(fkCols []FKRefInfo, columnName string) (FKRefInfo, bool) {
	for _, info := range fkCols {
		if info.Column == columnName {
			return info, true
		}
	}
	return FKRefInfo{}, false
}

// extractPKValues extracts the primary key values from a generated table.
// Supports single-column PKs and composite PKs (returned as string keys).
func extractPKValues(generatedTable *GeneratedTable, table *schema.Table) []any {
	if len(table.PrimaryKey) == 0 {
		return nil
	}
	if len(table.PrimaryKey) == 1 {
		pkCol := table.PrimaryKey[0]
		values := make([]any, len(generatedTable.Rows))
		for i, row := range generatedTable.Rows {
			values[i] = row[pkCol]
		}
		return values
	}
	// Composite PK: return string-keyed map values.
	values := make([]any, len(generatedTable.Rows))
	for i, row := range generatedTable.Rows {
		var parts []string
		for _, pkCol := range table.PrimaryKey {
			parts = append(parts, fmt.Sprintf("%v", row[pkCol]))
		}
		values[i] = strings.Join(parts, "::")
	}
	return values
}
