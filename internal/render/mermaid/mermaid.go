// Package mermaid renders a schema Graph as a Mermaid ER diagram.
//
// The output uses Mermaid's erDiagram syntax, mapping tables to entities,
// columns to entity attributes, and foreign keys to relationships.
// The result can be pasted directly into any Mermaid-compatible renderer
// (GitHub markdown, VS Code, mermaid.live, etc.).
//
// # Example output
//
//	erDiagram
//	    users {
//	        bigint id PK
//	        varchar email
//	        boolean is_active
//	    }
//	    orders {
//	        bigint id PK
//	        bigint user_id FK
//	    }
//	    users ||--o{ orders : "FK user_id"
//
// # Cardinality notation
//
// Every FK relationship is rendered from the parent (referenced) table
// to the child (referencing) table. The cardinality reflects the
// per-FK perspective:
//
//	||--||   one to one        — FK columns are the child's full PK.
//	||--o{   one to many       — default for most FKs.
package mermaid

import (
	"fmt"
	"strings"

	"synthgraph/internal/graph"
)

// RenderOption configures the ER diagram output.
type RenderOption func(*renderConfig)

// renderConfig holds all configurable options for the renderer.
type renderConfig struct {
	showEnums      bool
	showColumnNull bool
	showColumnDef  bool
}

// WithEnums controls whether enum types are rendered as standalone entities.
// Disabled by default because enums are not database tables.
func WithEnums(show bool) RenderOption {
	return func(cfg *renderConfig) {
		cfg.showEnums = show
	}
}

// WithColumnNull controls whether nullable columns show a "NULL" annotation.
// Disabled by default (only NOT NULL is implied).
func WithColumnNull(show bool) RenderOption {
	return func(cfg *renderConfig) {
		cfg.showColumnNull = show
	}
}

// WithColumnDefault controls whether DEFAULT expressions are shown as annotations.
// Disabled by default.
func WithColumnDefault(show bool) RenderOption {
	return func(cfg *renderConfig) {
		cfg.showColumnDef = show
	}
}

// Render produces a Mermaid ER diagram string from the given schema graph.
//
// The output is deterministic: the same graph always produces the same string.
// Tables appear in NodeList order, columns within each table in their
// Contains-edge order, and FK relationships after all entity blocks.
func Render(g *graph.Graph, opts ...RenderOption) (string, error) {
	cfg := &renderConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Pre-compute FK column sets per table — which columns are foreign keys.
	fkColumnSets := buildFKColumnSets(g)

	var b strings.Builder
	b.WriteString("erDiagram\n")

	// Phase 1: entity blocks for each table (in NodeList order).
	for _, node := range g.NodeList {
		if node.Kind != graph.NodeKindTable {
			continue
		}
		writeTableEntity(&b, node, g, fkColumnSets[node.ID], cfg)
	}

	// Phase 2: entity blocks for each enum (in NodeList order).
	// Enums are rendered as entities only when explicitly requested.
	if cfg.showEnums {
		for _, node := range g.NodeList {
			if node.Kind != graph.NodeKindEnum {
				continue
			}
			writeEnumEntity(&b, node)
		}
	}

	// Phase 3: FK relationship lines (in Edge order).
	// Mermaid puts the "one" (parent) side first, then cardinality,
	// then the "many" (child) side — the reverse of our References edges.
	for _, edge := range g.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}
		writeRelationLine(&b, edge, g)
	}

	return b.String(), nil
}

// ── FK column detection ────────────────────────────────────────────────────

// buildFKColumnSets returns a map from table node ID to the set of column
// names that are part of at least one foreign key on that table.
func buildFKColumnSets(g *graph.Graph) map[string]map[string]bool {
	sets := make(map[string]map[string]bool)
	for _, edge := range g.Edges {
		if edge.Kind != graph.EdgeKindReferences {
			continue
		}
		fkMeta, ok := edge.Metadata.(*graph.FKMetadata)
		if !ok {
			continue
		}
		set := sets[edge.From]
		if set == nil {
			set = make(map[string]bool)
			sets[edge.From] = set
		}
		for _, col := range fkMeta.LocalColumns {
			set[col] = true
		}
	}
	return sets
}

// ── Table entities ──────────────────────────────────────────────────────────

// writeTableEntity writes a Mermaid entity block for one table node.
func writeTableEntity(b *strings.Builder, node *graph.Node, g *graph.Graph, fkColumns map[string]bool, cfg *renderConfig) {
	tableData := node.Data.(graph.TableData)
	entityName := sanitizeName(tableData.Name)

	fmt.Fprintf(b, "    %s {\n", entityName)

	// Collect columns from Contains edges, preserving edge order (which is
	// deterministic — same as column declaration order).
	for _, edge := range g.Edges {
		if edge.From != node.ID || edge.Kind != graph.EdgeKindContains {
			continue
		}
		columnNode, ok := g.Nodes[edge.To]
		if !ok {
			continue
		}
		colData := columnNode.Data.(graph.ColumnData)

		isFK := fkColumns[columnNode.Label]
		writeColumnLine(b, &colData, columnNode.Label, isFK, cfg)
	}

	b.WriteString("    }\n")
}

// writeColumnLine writes a single column attribute line within an entity block.
func writeColumnLine(b *strings.Builder, colData *graph.ColumnData, label string, isFK bool, cfg *renderConfig) {
	typeStr := formatType(colData)

	fmt.Fprintf(b, "        %s %s", typeStr, label)

	annotations := buildAnnotations(colData, isFK, cfg)
	if annotations != "" {
		b.WriteString(" ")
		b.WriteString(annotations)
	}
	b.WriteString("\n")
}

// formatType renders the column's type for display in the ER diagram.
func formatType(colData *graph.ColumnData) string {
	if colData.Length > 0 {
		if colData.Precision > 0 {
			return fmt.Sprintf("%s(%d,%d)", colData.Type, colData.Length, colData.Precision)
		}
		return fmt.Sprintf("%s(%d)", colData.Type, colData.Length)
	}
	return colData.Type
}

// buildAnnotations produces inline annotations (PK, FK, etc.) for a column.
func buildAnnotations(colData *graph.ColumnData, isFK bool, cfg *renderConfig) string {
	var parts []string

	if colData.IsPrimaryKey {
		parts = append(parts, "PK")
	} else if isFK {
		parts = append(parts, "FK")
	}

	if !colData.Nullable {
		if cfg.showColumnNull {
			parts = append(parts, "NOT NULL")
		}
	} else {
		if cfg.showColumnNull {
			parts = append(parts, "NULL")
		}
	}

	if cfg.showColumnDef && colData.Default != nil && *colData.Default != "" {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", *colData.Default))
	}

	return strings.Join(parts, " ")
}

// ── Enum entities ───────────────────────────────────────────────────────────

// writeEnumEntity writes an entity block for one enum type.
//
// Enum types are rendered as entities whose "attributes" are the enum values,
// all of the pseudo-type "string". This is a recognised Mermaid convention
// for representing enum-like types in ER diagrams. Enable with WithEnums(true).
func writeEnumEntity(b *strings.Builder, node *graph.Node) {
	enumData := node.Data.(graph.EnumData)
	entityName := sanitizeName(node.Label)

	fmt.Fprintf(b, "    %s {\n", entityName)
	for _, value := range enumData.Values {
		fmt.Fprintf(b, "        string %s\n", value)
	}
	b.WriteString("    }\n")
}

// ── FK relationships ────────────────────────────────────────────────────────

// writeRelationLine writes a single Mermaid relationship line from a
// References edge.
//
// The edge direction is child → parent, but Mermaid ER notation expects
// the "one" (parent) side first. We reverse the direction here.
func writeRelationLine(b *strings.Builder, edge *graph.Edge, g *graph.Graph) {
	fkMeta, ok := edge.Metadata.(*graph.FKMetadata)
	if !ok {
		return
	}

	fromNode, ok := g.Nodes[edge.From]
	if !ok {
		return
	}
	toNode, ok := g.Nodes[edge.To]
	if !ok {
		return
	}

	fromTable, ok := fromNode.Data.(graph.TableData)
	if !ok {
		return
	}
	toTable, ok := toNode.Data.(graph.TableData)
	if !ok {
		return
	}

	parentName := sanitizeName(toTable.Name)  // referenced (parent)
	childName := sanitizeName(fromTable.Name)  // referencing (child)
	cardSymbol := mermaidCardinality(fkMeta.Cardinality)

	label := buildRelationLabel(fromTable.Name, fkMeta)

	fmt.Fprintf(b, "    %s %s %s : \"%s\"\n", parentName, cardSymbol, childName, label)
}

// mermaidCardinality maps our Cardinality to Mermaid's ER diagram notation.
//
//	one_to_one    ⟶  ||--||    — parent has at most one child
//	one_to_many   ⟶  ||--o{    — parent has many children (default)
//	many_to_many  ⟶  ||--o{    — each individual FK is still a one-to-many
//	                             from the parent's perspective; the overall
//	                             many-to-many is a table-level property
//	                             achieved via a junction table.
func mermaidCardinality(card graph.Cardinality) string {
	if card == graph.CardinalityOneToOne {
		return "||--||"
	}
	return "||--o{"
}

// buildRelationLabel creates the relationship label from FK metadata.
// Format: "FK col1, col2" showing the child table's FK columns.
func buildRelationLabel(childTableName string, fkMeta *graph.FKMetadata) string {
	cols := strings.Join(fkMeta.LocalColumns, ", ")
	return fmt.Sprintf("FK %s", cols)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// sanitizeName replaces characters that are problematic in Mermaid entity names.
//
// Schema-qualified names (e.g. "inventory.categories") use dots that Mermaid
// may interpret as a field reference in some contexts. We replace dots with
// underscores to produce a safe identifier.
func sanitizeName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}
