package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dataset, model, err := generateData(ctx, config)
	if err != nil {
		if errors.Is(err, generator.ErrCancelled) {
			fmt.Fprintf(os.Stderr, "\nwarning: generation cancelled — exporting partial data (%d tables)\n", len(dataset.Tables))
		} else {
			fmt.Fprintf(os.Stderr, "error: generation failed: %v\n", err)
			os.Exit(1)
		}
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

func generateData(ctx context.Context, config *generateConfig) (*generator.Dataset, *schema.Model, error) {
	model, err := parseSQLFile(config.input)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing schema: %w", err)
	}

	g, err := graph.Build(model)
	if err != nil {
		return nil, nil, fmt.Errorf("building graph: %w", err)
	}

	sg, err := semantic.Build(g)
	if err != nil {
		return nil, nil, fmt.Errorf("building semantic graph: %w", err)
	}

	plan, err := planner.BuildPlan(g, model, config.rows)
	if err != nil {
		return nil, nil, fmt.Errorf("building generation plan: %w", err)
	}

	genCtx := &generator.GenerationContext{
		Context:       ctx,
		GlobalSeed:    uint64(config.seed),
		Model:         model,
		Graph:         g,
		SemanticGraph: sg,
	}

	dataset, err := generator.Generate(plan, genCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("generating data: %w", err)
	}

	return dataset, model, nil
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
