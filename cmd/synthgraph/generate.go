package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"synthgraph/internal/exporter"
	"synthgraph/internal/generator"
	"synthgraph/internal/graph"
	"synthgraph/internal/planner"
	"synthgraph/internal/schema"
	"synthgraph/internal/semantic"
)

type generateConfig struct {
	input      string
	output     string
	format     string
	rows       int
	seed       int64
	schemaName string
}

func runGenerate(args []string) {
	config, err := parseGenerateFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := config.validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	model, parseErr := parseSQLFile(config.input)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "error: parsing schema: %v\n", parseErr)
		os.Exit(1)
	}

	if validationErrors := schema.Validate(model); len(validationErrors) > 0 {
		fmt.Fprintf(os.Stderr, "error: schema validation failed:\n")
		for _, ve := range validationErrors {
			fmt.Fprintf(os.Stderr, "  - %v\n", ve)
		}
		os.Exit(1)
	}

	dataset, genErr := generateData(model, config)
	if genErr != nil {
		fmt.Fprintf(os.Stderr, "error: generation failed: %v\n", genErr)
		os.Exit(1)
	}

	if len(dataset.Tables) == 0 {
		fmt.Fprintf(os.Stderr, "error: no tables were generated\n")
		os.Exit(1)
	}

	if err := exportData(config, dataset, model); err != nil {
		fmt.Fprintf(os.Stderr, "error: export failed: %v\n", err)
		os.Exit(1)
	}
}

func parseGenerateFlags(args []string) (*generateConfig, error) {
	config := &generateConfig{}

	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	flags.StringVar(&config.input, "input", "", "Input SQL schema file (required)")
	flags.StringVar(&config.input, "i", "", "Input SQL schema file (required)")
	flags.StringVar(&config.output, "output", "", "Output file (default: stdout)")
	flags.StringVar(&config.output, "o", "", "Output file (default: stdout)")
	flags.StringVar(&config.format, "format", "sql", "Output format: sql, csv")
	flags.StringVar(&config.format, "f", "sql", "Output format: sql, csv")
	flags.IntVar(&config.rows, "rows", 10, "Number of rows per table")
	flags.IntVar(&config.rows, "r", 10, "Number of rows per table")
	flags.Int64Var(&config.seed, "seed", 42, "Global random seed for determinism")
	flags.Int64Var(&config.seed, "s", 42, "Global random seed for determinism")
	flags.StringVar(&config.schemaName, "schema-name", "", "Schema name for SQL output (optional)")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if flags.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument: %s", strings.Join(flags.Args(), " "))
	}

	return config, nil
}

func (c *generateConfig) validate() error {
	if c.input == "" {
		return fmt.Errorf("--input is required")
	}
	if c.rows < 0 {
		return fmt.Errorf("--rows must be non-negative")
	}
	switch c.format {
	case "sql", "csv":
		// valid
	default:
		return fmt.Errorf("unsupported format %q: use sql or csv", c.format)
	}
	return nil
}

func generateData(model *schema.Model, config *generateConfig) (*generator.Dataset, error) {
	g, err := graph.Build(model)
	if err != nil {
		return nil, fmt.Errorf("building graph: %w", err)
	}

	sg, err := semantic.Build(g)
	if err != nil {
		return nil, fmt.Errorf("building semantic graph: %w", err)
	}

	plan, err := planner.BuildPlan(g, model, config.rows)
	if err != nil {
		return nil, fmt.Errorf("building generation plan: %w", err)
	}

	ctx := &generator.GenerationContext{
		GlobalSeed:    uint64(config.seed),
		Model:         model,
		Graph:         g,
		SemanticGraph: sg,
	}

	dataset, err := generator.Generate(plan, ctx)
	if err != nil {
		return nil, fmt.Errorf("generating data: %w", err)
	}

	return dataset, nil
}

func exportData(config *generateConfig, dataset *generator.Dataset, model *schema.Model) error {
	var writer io.Writer
	if config.output != "" {
		file, err := os.Create(config.output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer file.Close()
		writer = file
	} else {
		writer = os.Stdout
	}

	switch config.format {
	case "sql":
		exportCfg := &exporter.ExportConfig{
			SchemaName: config.schemaName,
		}
		return exporter.ExportSQL(writer, dataset, model, exportCfg)
	case "csv":
		exportCfg := &exporter.ExportConfig{
			SchemaName:   config.schemaName,
			IncludeHeader: true,
		}
		return exporter.ExportCSV(writer, dataset, model, exportCfg)
	default:
		return fmt.Errorf("unsupported format: %s", config.format)
	}
}
