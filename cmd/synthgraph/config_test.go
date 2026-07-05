package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractConfigFlag(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantPath      string
		wantRemaining []string
	}{
		{
			name:          "no config flag",
			args:          []string{"--graph", "--semantic"},
			wantPath:      "",
			wantRemaining: []string{"--graph", "--semantic"},
		},
		{
			name:          "--config with space",
			args:          []string{"--config", "myconfig.yaml", "--graph"},
			wantPath:      "myconfig.yaml",
			wantRemaining: []string{"--graph"},
		},
		{
			name:          "--config with equals",
			args:          []string{"--config=myconfig.yaml", "--graph"},
			wantPath:      "myconfig.yaml",
			wantRemaining: []string{"--graph"},
		},
		{
			name:          "--config at end",
			args:          []string{"--graph", "--config", "cfg.yaml"},
			wantPath:      "cfg.yaml",
			wantRemaining: []string{"--graph"},
		},
		{
			name:          "full inspect args",
			args:          []string{"--config", "synthgraph_test.yaml", "--graph", "--semantic"},
			wantPath:      "synthgraph_test.yaml",
			wantRemaining: []string{"--graph", "--semantic"},
		},
		{
			name:          "--config alone",
			args:          []string{"--config", "cfg.yaml"},
			wantPath:      "cfg.yaml",
			wantRemaining: []string{},
		},
		{
			name:          "multiple flags before config",
			args:          []string{"-i", "input.sql", "-f", "csv", "--config", "cfg.yaml"},
			wantPath:      "cfg.yaml",
			wantRemaining: []string{"-i", "input.sql", "-f", "csv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotRemaining := extractConfigFlag(tt.args)
			if gotPath != tt.wantPath {
				t.Errorf("extractConfigFlag() path = %q, want %q", gotPath, tt.wantPath)
			}
			if !reflect.DeepEqual(gotRemaining, tt.wantRemaining) {
				t.Errorf("extractConfigFlag() remaining = %v, want %v", gotRemaining, tt.wantRemaining)
			}
		})
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
verbose: true

generate:
  input: schema.sql
  format: csv
  rows: 50
  seed: 123

inspect:
  input: inspect.sql
  graph: true
  semantic: true
`
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatalf("loadConfigFile() error = %v", err)
	}

	if !cfg.Verbose {
		t.Error("cfg.Verbose = false, want true")
	}
	if cfg.Generate == nil {
		t.Fatal("cfg.Generate = nil")
	}
	if cfg.Generate.Input != "schema.sql" {
		t.Errorf("cfg.Generate.Input = %q, want %q", cfg.Generate.Input, "schema.sql")
	}
	if cfg.Generate.Format != "csv" {
		t.Errorf("cfg.Generate.Format = %q, want %q", cfg.Generate.Format, "csv")
	}
	if cfg.Generate.Rows == nil || *cfg.Generate.Rows != 50 {
		t.Errorf("cfg.Generate.Rows = %v, want 50", cfg.Generate.Rows)
	}
	if cfg.Generate.Seed == nil || *cfg.Generate.Seed != 123 {
		t.Errorf("cfg.Generate.Seed = %v, want 123", cfg.Generate.Seed)
	}

	if cfg.Inspect == nil {
		t.Fatal("cfg.Inspect = nil")
	}
	if cfg.Inspect.Input != "inspect.sql" {
		t.Errorf("cfg.Inspect.Input = %q, want %q", cfg.Inspect.Input, "inspect.sql")
	}
	if cfg.Inspect.Graph == nil || !*cfg.Inspect.Graph {
		t.Error("cfg.Inspect.Graph = false, want true")
	}
	if cfg.Inspect.Semantic == nil || !*cfg.Inspect.Semantic {
		t.Error("cfg.Inspect.Semantic = false, want true")
	}
}

func TestLoadConfigFileErrors(t *testing.T) {
	if _, err := loadConfigFile("nonexistent.yaml"); err == nil {
		t.Error("expected error for missing file")
	}

	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("invalid: [yaml: "), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfigFile(badPath); err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synthgraph.yaml")

	if err := writeDefaultConfig(path); err != nil {
		t.Fatalf("writeDefaultConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("default config is empty")
	}
}

func TestDefaultGenerateConfig(t *testing.T) {
	c := defaultGenerateConfig()
	if c.format != "sql" {
		t.Errorf("default format = %q, want %q", c.format, "sql")
	}
	if c.rows != 10 {
		t.Errorf("default rows = %d, want 10", c.rows)
	}
	if c.seed != 42 {
		t.Errorf("default seed = %d, want 42", c.seed)
	}
}

func TestApplyGenerateFile(t *testing.T) {
	rows := 100
	seed := int64(999)

	fileCfg := &ConfigFile{
		Generate: &GenerateFileConfig{
			Input:      "test.sql",
			Output:     "out.csv",
			Format:     "csv",
			Rows:       &rows,
			Seed:       &seed,
			SchemaName: "public",
		},
	}

	cfg := defaultGenerateConfig()
	cfg.applyGenerateFile(fileCfg)

	if cfg.input != "test.sql" {
		t.Errorf("input = %q, want %q", cfg.input, "test.sql")
	}
	if cfg.output != "out.csv" {
		t.Errorf("output = %q, want %q", cfg.output, "out.csv")
	}
	if cfg.format != "csv" {
		t.Errorf("format = %q, want %q", cfg.format, "csv")
	}
	if cfg.rows != 100 {
		t.Errorf("rows = %d, want 100", cfg.rows)
	}
	if cfg.seed != 999 {
		t.Errorf("seed = %d, want 999", cfg.seed)
	}
	if cfg.schemaName != "public" {
		t.Errorf("schemaName = %q, want %q", cfg.schemaName, "public")
	}
}

func TestDefaultInspectConfig(t *testing.T) {
	c := defaultInspectConfig()
	if c.graph {
		t.Error("default graph should be false")
	}
	if c.semantic {
		t.Error("default semantic should be false")
	}
}

func TestApplyInspectFile(t *testing.T) {
	graph := true
	semantic := true

	fileCfg := &ConfigFile{
		Inspect: &InspectFileConfig{
			Input:    "test.sql",
			Graph:    &graph,
			Semantic: &semantic,
		},
	}

	cfg := defaultInspectConfig()
	cfg.applyInspectFile(fileCfg)

	if cfg.input != "test.sql" {
		t.Errorf("input = %q, want %q", cfg.input, "test.sql")
	}
	if !cfg.graph {
		t.Error("graph should be true")
	}
	if !cfg.semantic {
		t.Error("semantic should be true")
	}
}

func TestApplyGenerateFileNilSection(t *testing.T) {
	fileCfg := &ConfigFile{}
	cfg := defaultGenerateConfig()
	cfg.applyGenerateFile(fileCfg)

	// Should not panic and values should remain default
	if cfg.rows != 10 {
		t.Errorf("rows = %d, want 10", cfg.rows)
	}
}

func TestApplyInspectFileNilSection(t *testing.T) {
	fileCfg := &ConfigFile{}
	cfg := defaultInspectConfig()
	cfg.applyInspectFile(fileCfg)

	// Should not panic
	if cfg.graph {
		t.Error("graph should remain false")
	}
}
