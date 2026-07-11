package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"synthgraph/internal/graph"
	"synthgraph/internal/parser"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

type inspectConfig struct {
	input    string
	graph    bool
	semantic bool
	verbose  bool
}

func runInspect(args []string) {
	config, err := parseInspectFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		globalLogger.Error("%v", err)
		os.Exit(1)
	}

	if config.input == "" {
		globalLogger.Error("--input is required")
		os.Exit(1)
	}

	if config.verbose {
		globalLogger = NewLogger(LevelDebug)
	}

	globalLogger.Debug("inspecting schema: %s", config.input)
	model, err := parseSQLFile(config.input)
	if err != nil {
		if pe := (*parser.ParseError)(nil); errors.As(err, &pe) {
			globalLogger.Error("parsing schema: %s", pe.Error())
		} else {
			globalLogger.Error("parsing schema: %v", err)
		}
		os.Exit(1)
	}

	// Always print schema overview.
	printSchemaOverview(model)

	showGraph := config.graph || config.verbose
	showSemantic := config.semantic || config.verbose

	if showGraph || showSemantic {
		g, graphError := graph.Build(model)
		if graphError != nil {
			globalLogger.Error("building graph: %v", graphError)
			os.Exit(1)
		}

		if showSemantic {
			semantic.ResolveColumns(model)
		}

		if showGraph {
			printGraphSummary(g)
		}

		if showSemantic {
			sg, semanticError := semantic.Build(g)
			if semanticError != nil {
				globalLogger.Error("building semantic graph: %v", semanticError)
				os.Exit(1)
			}
			printSemanticSummary(sg)
		}
	}
}

func parseInspectFlags(args []string) (*inspectConfig, error) {
	configPath, args := extractConfigFlag(args)

	config := defaultInspectConfig()

	if configPath != "" {
		fileCfg, err := loadConfigFile(configPath)
		if err != nil {
			return nil, err
		}
		config.applyInspectFile(fileCfg)
		if fileCfg.Verbose {
			config.verbose = true
		}
	}

	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	flags.StringVar(&config.input, "input", config.input, "Input SQL schema file (required)")
	flags.StringVar(&config.input, "i", config.input, "Input SQL schema file (required)")
	flags.BoolVar(&config.graph, "graph", config.graph, "Show graph structure summary")
	flags.BoolVar(&config.semantic, "semantic", config.semantic, "Show semantic inference summary")
	flags.BoolVar(&config.verbose, "v", config.verbose, "Verbose: enable debug logging and show all summaries")
	flags.BoolVar(&config.verbose, "verbose", config.verbose, "Verbose: enable debug logging and show all summaries")
	flags.StringVar(&configPath, "config", configPath, "Path to YAML config file")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if flags.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument: %s", strings.Join(flags.Args(), " "))
	}

	return config, nil
}

func printSchemaOverview(model *schema.Model) {
	fmt.Println("Schema Overview")
	fmt.Println("===============")
	fmt.Printf("Tables: %d\n", len(model.Tables))
	fmt.Printf("Enums:  %d\n", len(model.Enums))
	fmt.Println()

	for _, table := range model.Tables {
		fmt.Printf("Table: %s\n", table.Name)
		fmt.Printf("  Columns: %d\n", len(table.Columns))
		if len(table.PrimaryKey) > 0 {
			fmt.Printf("  Primary Key: %s\n", strings.Join(table.PrimaryKey, ", "))
		}
		for _, col := range table.Columns {
			nullable := ""
			if col.Nullable {
				nullable = " NULL"
			}
			pk := ""
			if col.IsPrimaryKey {
				pk = " PK"
			}
			fmt.Printf("    %s %s%s%s\n", col.Name, col.Type, nullable, pk)
		}
		if len(table.ForeignKeys) > 0 {
			fmt.Println("  Foreign Keys:")
			for _, fk := range table.ForeignKeys {
				fmt.Printf("    %s → %s(%s)\n",
					strings.Join(fk.Columns, ", "),
					fk.RefTable,
					strings.Join(fk.RefColumns, ", "))
			}
		}
		if len(table.Unique) > 0 {
			fmt.Println("  Unique Constraints:")
			for _, u := range table.Unique {
				fmt.Printf("    (%s)\n", strings.Join(u, ", "))
			}
		}
		if len(table.Checks) > 0 {
			fmt.Println("  Check Constraints:")
			for _, c := range table.Checks {
				fmt.Printf("    %s\n", c.Expression)
			}
		}
		fmt.Println()
	}

	for _, enum := range model.Enums {
		fmt.Printf("Enum: %s (%s)\n", enum.Name, strings.Join(enum.Values, ", "))
	}
}

func printGraphSummary(g *graph.Graph) {
	fmt.Println("Graph Summary")
	fmt.Println("=============")

	tableCount := 0
	enumCount := 0
	columnCount := 0
	for _, node := range g.NodeList {
		switch node.Kind {
		case graph.NodeKindTable:
			tableCount++
		case graph.NodeKindEnum:
			enumCount++
		case graph.NodeKindColumn:
			columnCount++
		}
	}
	fmt.Printf("Nodes: %d (tables=%d, enums=%d, columns=%d)\n",
		len(g.NodeList), tableCount, enumCount, columnCount)

	edgeKindCounts := map[graph.EdgeKind]int{}
	for _, edge := range g.Edges {
		edgeKindCounts[edge.Kind]++
	}
	fmt.Printf("Edges: %d\n", len(g.Edges))
	for kind, count := range edgeKindCounts {
		fmt.Printf("  %s: %d\n", kind, count)
	}
	fmt.Println()
}

func printSemanticSummary(sg *semantic.SemanticGraph) {
	fmt.Println("Semantic Summary")
	fmt.Println("================")

	for _, semNode := range sg.NodeList {
		id := semNode.ID
		node := semNode
		if len(node.Roles) == 0 && node.Temporal == nil && node.Audit == nil && len(node.Inferences) == 0 {
			continue
		}
		fmt.Printf("Node: %s\n", id)
		if len(node.Roles) > 0 {
			roleStrs := make([]string, len(node.Roles))
			for i, r := range node.Roles {
				roleStrs[i] = string(r)
			}
			fmt.Printf("  Roles: %s\n", strings.Join(roleStrs, ", "))
		}
		if node.IsHierarchical {
			fmt.Println("  Hierarchical: true")
		}
		if node.IsSoftDelete {
			fmt.Println("  Soft Delete: true")
		}
		if node.Temporal != nil {
			fmt.Printf("  Temporal: created=%v updated=%v deleted=%v\n",
				node.Temporal.HasCreatedAt, node.Temporal.HasUpdatedAt, node.Temporal.HasDeletedAt)
		}
		if node.Audit != nil {
			fmt.Printf("  Audit: created_by=%v updated_by=%v deleted_by=%v\n",
				node.Audit.HasCreatedBy, node.Audit.HasUpdatedBy, node.Audit.HasDeletedBy)
		}
		if len(node.Inferences) > 0 {
			fmt.Printf("  Inferences: %d\n", len(node.Inferences))
			for _, inf := range node.Inferences {
				fmt.Printf("    %s (confidence=%.2f)\n", inf.Kind, inf.Confidence)
			}
		}
		fmt.Println()
	}

	if len(sg.Relationships) > 0 {
		fmt.Println("Relationships:")
		for _, rel := range sg.Relationships {
			fmt.Printf("  %s -> %s: %s\n", rel.From, rel.To, rel.Kind)
		}
		fmt.Println()
	}
}

func defaultInspectConfig() *inspectConfig {
	return &inspectConfig{}
}

func (c *inspectConfig) applyInspectFile(fc *ConfigFile) {
	if fc.Inspect == nil {
		return
	}
	ic := fc.Inspect
	if ic.Input != "" {
		c.input = ic.Input
	}
	if ic.Graph != nil {
		c.graph = *ic.Graph
	}
	if ic.Semantic != nil {
		c.semantic = *ic.Semantic
	}
}
