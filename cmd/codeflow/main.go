package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"codeflow/pkg/cli"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "codeflow",
		Short: "CodeFlow: Multi-Language Code & Workflow Analysis Platform",
		Long: `CodeFlow scans multi-language repositories recursively, parses source code ASTs
and workflow definitions into a unified ProcessModel, correlates cross-service dependencies,
and exports to diagrammatic formats (Mermaid, Visio-compatible Excel, JSON, SVG).`,
	}

	rootCmd.AddCommand(cli.NewAnalyzeCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
