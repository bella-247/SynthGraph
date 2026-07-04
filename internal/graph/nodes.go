package graph

import "synthgraph/internal/schema"

// addTableNodes creates one NodeKindTable node for every table in the schema.
// Nodes are added in the same order as model.Tables, ensuring determinism.
func addTableNodes(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		node := &Node{
			ID:    tableNodeID(table.Name),
			Kind:  NodeKindTable,
			Label: table.Name,
			Data: TableData{
				Name:       table.Name,
				PrimaryKey: table.PrimaryKey,
				Unique:     table.Unique,
				Checks:     table.Checks,
			},
		}
		schemaGraph.addNode(node)
	}
}

// addEnumNodes creates one NodeKindEnum node for every enum type in the schema.
// Nodes are added in the same order as model.Enums, ensuring determinism.
func addEnumNodes(schemaGraph *Graph, model *schema.Model) {
	for _, enumType := range model.Enums {
		node := &Node{
			ID:    enumNodeID(enumType.Name),
			Kind:  NodeKindEnum,
			Label: enumType.Name,
			Data: EnumData{
				Values: enumType.Values,
			},
		}
		schemaGraph.addNode(node)
	}
}

// addColumnNodes creates one NodeKindColumn node for every column in every table.
// Nodes are added in table order, and within each table in column declaration order,
// ensuring determinism.
func addColumnNodes(schemaGraph *Graph, model *schema.Model) {
	for _, table := range model.Tables {
		for _, column := range table.Columns {
			node := &Node{
				ID:    columnNodeID(table.Name, column.Name),
				Kind:  NodeKindColumn,
				Label: column.Name,
				Data: ColumnData{
					Type:         column.Type,
					Length:       column.Length,
					Precision:    column.Precision,
					Nullable:     column.Nullable,
					Default:      column.Default,
					IsPrimaryKey: column.IsPrimaryKey,
				},
			}
			schemaGraph.addNode(node)
		}
	}
}
