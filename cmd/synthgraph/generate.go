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
	"synthgraph/internal/validator"
)

type generateConfig struct {
	input      string
	output     string
	format     string
	rows       int
	seed       int64
	schemaName string
	verbose    bool
}

func runGenerate(args []string) {
	config, err := parseGenerateFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		globalLogger.Error("%v", err)
		os.Exit(1)
	}

	if err := config.validate(); err != nil {
		globalLogger.Error("%v", err)
		os.Exit(1)
	}

	if config.verbose {
		globalLogger = NewLogger(LevelDebug)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	model, parseErr := parseSQLFile(config.input)
	if parseErr != nil {
		globalLogger.Error("parsing schema: %v", parseErr)
		os.Exit(1)
	}

	if validationErrors := schema.Validate(model); len(validationErrors) > 0 {
		globalLogger.Error("schema validation failed:")
		for _, ve := range validationErrors {
			globalLogger.Error("  - %v", ve)
		}
		os.Exit(1)
	}

	dataset, genErr := generateData(ctx, model, config)
	if genErr != nil {
		if errors.Is(genErr, generator.ErrCancelled) {
			globalLogger.Error("generation cancelled — exporting partial data (%d tables)", len(dataset.Tables))
		} else {
			globalLogger.Error("generation failed: %v", genErr)
			os.Exit(1)
		}
	}

	if len(dataset.Errors) > 0 {
		for _, pe := range dataset.Errors {
			globalLogger.Error("table %q skipped: %v", pe.Table, pe.Err)
		}
	}

	if len(dataset.Tables) == 0 {
		globalLogger.Error("no tables were generated")
		os.Exit(1)
	}

	globalLogger.Info("generated %d table(s)", len(dataset.Tables))

	if validationErrors := validator.Validate(dataset, model); len(validationErrors) > 0 {
		globalLogger.Error("post-generation validation failed (%d violation(s)):", len(validationErrors))
		for _, ve := range validationErrors {
			globalLogger.Error("  - %s", ve.Error())
		}
		os.Exit(1)
	}

	if err := exportData(config, dataset, model); err != nil {
		globalLogger.Error("export failed: %v", err)
		os.Exit(1)
	}

	globalLogger.Debug("output format: %s", config.format)
}

func parseGenerateFlags(args []string) (*generateConfig, error) {
	// Extract --config and --init-config before full flag parsing.
	configPath, args := extractConfigFlag(args)

	initConfig := false
	for _, a := range args {
		if a == "--init-config" {
			initConfig = true
			break
		}
	}

	if initConfig {
		path := "synthgraph.yaml"
		for i, a := range args {
			if a == "--init-config" && i+1 < len(args) {
				path = args[i+1]
				break
			}
			if len(a) > 14 && a[:14] == "--init-config=" {
				path = a[14:]
				break
			}
		}
		if err := writeDefaultConfig(path); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}
		fmt.Printf("wrote default config to %s\n", path)
		os.Exit(0)
	}

	config := defaultGenerateConfig()

	// Load config file if specified.
	if configPath != "" {
		fileCfg, err := loadConfigFile(configPath)
		if err != nil {
			return nil, err
		}
		config.applyGenerateFile(fileCfg)
		if fileCfg.Verbose {
			config.verbose = true
		}
	}

	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	flags.StringVar(&config.input, "input", config.input, "Input SQL schema file (required)")
	flags.StringVar(&config.input, "i", config.input, "Input SQL schema file (required)")
	flags.StringVar(&config.output, "output", config.output, "Output file (default: stdout)")
	flags.StringVar(&config.output, "o", config.output, "Output file (default: stdout)")
	flags.StringVar(&config.format, "format", config.format, "Output format: sql, csv")
	flags.StringVar(&config.format, "f", config.format, "Output format: sql, csv")
	flags.IntVar(&config.rows, "rows", config.rows, "Number of rows per table")
	flags.IntVar(&config.rows, "r", config.rows, "Number of rows per table")
	flags.Int64Var(&config.seed, "seed", config.seed, "Global random seed for determinism")
	flags.Int64Var(&config.seed, "s", config.seed, "Global random seed for determinism")
	flags.StringVar(&config.schemaName, "schema-name", config.schemaName, "Schema name for SQL output (optional)")
	flags.BoolVar(&config.verbose, "v", config.verbose, "Enable verbose logging")
	flags.BoolVar(&config.verbose, "verbose", config.verbose, "Enable verbose logging")
	flags.StringVar(&configPath, "config", configPath, "Path to YAML config file")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if flags.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument: %s", strings.Join(flags.Args(), " "))
	}

	return config, nil
}

const (
	maxRows       = 100_000
	maxSchemaSize = 10 << 20 // 10 MB
)

func (c *generateConfig) validate() error {
	if c.input == "" {
		return fmt.Errorf("--input is required")
	}
	if c.rows < 0 {
		return fmt.Errorf("--rows must be non-negative")
	}
	if c.rows > maxRows {
		return fmt.Errorf("--rows exceeds maximum (%d)", maxRows)
	}
	fi, err := os.Stat(c.input)
	if err != nil {
		return fmt.Errorf("reading input file: %w", err)
	}
	if fi.Size() > maxSchemaSize {
		return fmt.Errorf("schema file too large (%d bytes, max %d)", fi.Size(), maxSchemaSize)
	}
	switch c.format {
	case "sql", "csv":
	default:
		return fmt.Errorf("unsupported format %q: use sql or csv", c.format)
	}
	return nil
}

func generateData(ctx context.Context, model *schema.Model, config *generateConfig) (*generator.Dataset, error) {
	g, err := graph.Build(model)
	if err != nil {
		return nil, fmt.Errorf("building graph: %w", err)
	}

	semantic.ResolveColumns(model)

	sg, err := semantic.Build(g)
	if err != nil {
		return nil, fmt.Errorf("building semantic graph: %w", err)
	}

	plan, err := planner.BuildPlan(g, model, config.rows)
	if err != nil {
		return nil, fmt.Errorf("building generation plan: %w", err)
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

func defaultGenerateConfig() *generateConfig {
	return &generateConfig{
		format: "sql",
		rows:   10,
		seed:   42,
	}
}

func (c *generateConfig) applyGenerateFile(fc *ConfigFile) {
	if fc.Generate == nil {
		return
	}
	g := fc.Generate
	if g.Input != "" {
		c.input = g.Input
	}
	if g.Output != "" {
		c.output = g.Output
	}
	if g.Format != "" {
		c.format = g.Format
	}
	if g.Rows != nil {
		c.rows = *g.Rows
	}
	if g.Seed != nil {
		c.seed = *g.Seed
	}
	if g.SchemaName != "" {
		c.schemaName = g.SchemaName
	}
}
