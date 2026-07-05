package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ConfigFile represents the full YAML config file structure.
type ConfigFile struct {
	Verbose  bool                `yaml:"verbose"`
	Generate *GenerateFileConfig `yaml:"generate"`
	Inspect  *InspectFileConfig  `yaml:"inspect"`
}

// GenerateFileConfig mirrors generate CLI flags.
type GenerateFileConfig struct {
	Input      string `yaml:"input"`
	Output     string `yaml:"output"`
	Format     string `yaml:"format"`
	Rows       *int   `yaml:"rows"`
	Seed       *int64 `yaml:"seed"`
	SchemaName string `yaml:"schema-name"`
}

// InspectFileConfig mirrors inspect CLI flags.
type InspectFileConfig struct {
	Input    string `yaml:"input"`
	Graph    *bool  `yaml:"graph"`
	Semantic *bool  `yaml:"semantic"`
}

func loadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

func writeDefaultConfig(path string) error {
	template := `# synthgraph configuration file
# Uncomment and set values to override defaults.
# CLI flags always take precedence over this file.

# verbose: false

# generate:
#   input: schema.sql
#   output: data.csv
#   format: csv
#   rows: 100
#   seed: 42
#   schema-name: public

# inspect:
#   input: schema.sql
#   graph: true
#   semantic: true
`
	return os.WriteFile(path, []byte(template), 0644)
}

// extractConfigFlag does a minimal pre-parse of --config from raw args
// so the config can be loaded before full flag parsing.
func extractConfigFlag(args []string) (configPath string, remaining []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--config" && i+1 < len(args) {
			p := args[i+1]
			r := append(args[:i], args[i+2:]...)
			return p, r
		}
		if len(a) > 9 && a[:9] == "--config=" {
			p := a[9:]
			r := append(args[:i], args[i+1:]...)
			return p, r
		}
	}
	return "", args
}
