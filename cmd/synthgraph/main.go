package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	subArgs := os.Args[2:]

	switch subcommand {
	case "generate":
		runGenerate(subArgs)
	case "inspect":
		runInspect(subArgs)
	case "version":
		runVersion()
	default:
		fmt.Fprintf(os.Stderr, "error: unknown subcommand %q\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `synthgraph — constraint-aware synthetic data generator

Usage:
  synthgraph generate [flags]   Generate synthetic data from a SQL schema
  synthgraph inspect  [flags]   Inspect a SQL schema structure
  synthgraph version            Print version information

Use "synthgraph <subcommand> --help" for more details about each subcommand.
`)
}
