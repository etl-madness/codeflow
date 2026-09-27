package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/analyzer/csharp"
	"codeflow/pkg/analyzer/flowxml"
	"codeflow/pkg/analyzer/golang"
	"codeflow/pkg/analyzer/python"
	"codeflow/pkg/analyzer/sql"
	"codeflow/pkg/analyzer/ssis"
	"codeflow/pkg/correlation"
	"codeflow/pkg/discovery"
	"codeflow/pkg/enrichment"
	"codeflow/pkg/exporter/excel"
	"codeflow/pkg/exporter/json"
	"codeflow/pkg/exporter/mermaid"
	"codeflow/pkg/exporter/svg"
	"codeflow/pkg/model"
)

type AnalyzeOptions struct {
	SourceDir      string
	Recursive      bool
	Formats        string
	OutputBase     string
	OutputDir      string
	OutputName     string
	AIEnrichment   bool
	CustomWorkbook bool
	ExcludeDirs    []string
	IgnorePatterns []string
	IgnoreTests    bool
	DiagramType    string
	NoTruncate     bool
	NoGitignore    bool
}

// NewAnalyzeCmd creates the 'analyze' cobra command.
func NewAnalyzeCmd() *cobra.Command {
	opts := &AnalyzeOptions{}

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze multi-language source repositories and workflows",
		Long:  `Scan code repositories recursively, extract ASTs, correlate dependencies across services, and export process diagrams.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAnalyze(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.SourceDir, "source", "s", ".", "Root directory or single source file to scan")
	cmd.Flags().BoolVarP(&opts.Recursive, "recursive", "r", true, "Traverse directories recursively")
	cmd.Flags().StringSliceVarP(&opts.ExcludeDirs, "exclude", "e", nil, "Comma-separated or repeated directory names/paths to exclude (e.g. -e tests,docs)")
	cmd.Flags().StringSliceVar(&opts.ExcludeDirs, "exclude-dir", nil, "Alias for --exclude")
	cmd.Flags().StringSliceVarP(&opts.IgnorePatterns, "ignore", "i", nil, "Comma-separated or repeated file/directory patterns to ignore (e.g. -i '*_test.go', '*.spec.ts')")
	cmd.Flags().StringSliceVar(&opts.IgnorePatterns, "ignore-pattern", nil, "Alias for --ignore")
	cmd.Flags().StringSliceVar(&opts.IgnorePatterns, "exclude-pattern", nil, "Alias for --ignore")
	cmd.Flags().BoolVar(&opts.IgnoreTests, "ignore-tests", false, "Ignore all standard unit test files across languages (*_test.go, test_*.py, *Test*.cs, *.spec.*)")
	cmd.Flags().StringVarP(&opts.DiagramType, "diagram-type", "d", "td", "Mermaid diagram type: td (Flowchart TD), lr (Flowchart LR), journey (User/System Journey), er (Entity-Relationship), all")
	cmd.Flags().StringVar(&opts.DiagramType, "mermaid-type", "td", "Alias for --diagram-type")
	cmd.Flags().StringVarP(&opts.Formats, "format", "f", "mermaid,excel,json", "Comma-separated export formats: mermaid, markdown, journey, er, excel, custom-excel, json, svg")
	cmd.Flags().StringVarP(&opts.OutputBase, "output", "o", "output", "Base path or filename for output files (without extension)")
	cmd.Flags().StringVar(&opts.OutputDir, "output-dir", "", "Directory to save output files (created if it does not exist)")
	cmd.Flags().StringVar(&opts.OutputDir, "out-dir", "", "Alias for --output-dir")
	cmd.Flags().StringVar(&opts.OutputDir, "outdir", "", "Alias for --output-dir")
	cmd.Flags().StringVarP(&opts.OutputName, "name", "n", "", "Base name for output files (e.g. 'myflow' generates myflow_sequence.mmd, myflow.json, etc.)")
	cmd.Flags().StringVar(&opts.OutputName, "output-name", "", "Alias for --name")
	cmd.Flags().StringVar(&opts.OutputName, "filename", "", "Alias for --name")
	cmd.Flags().StringVar(&opts.OutputName, "base-name", "", "Alias for --name")
	cmd.Flags().BoolVar(&opts.AIEnrichment, "ai-enrichment", false, "Enable AI enrichment for business-readable descriptions")
	cmd.Flags().BoolVar(&opts.CustomWorkbook, "custom-workbook", false, "Export Excel Data Visualizer Schema (custom workbook) with interactive dashboard and swimlanes")
	cmd.Flags().BoolVar(&opts.NoTruncate, "no-truncate", false, "Preserve full node title and description length without truncation in diagrams")
	cmd.Flags().BoolVar(&opts.NoTruncate, "full-text", false, "Alias for --no-truncate")
	cmd.Flags().BoolVar(&opts.NoGitignore, "no-gitignore", false, "Do not read or apply root .gitignore file during scanning")

	return cmd
}

// RunAnalyze executes the full CodeFlow analysis pipeline.
func RunAnalyze(opts *AnalyzeOptions) error {
	// Resolve output directory and output base name
	outputName := "output"
	outputDir := ""

	if opts.OutputBase != "" && opts.OutputBase != "output" {
		cleaned := filepath.Clean(opts.OutputBase)
		if strings.HasSuffix(opts.OutputBase, "/") || strings.HasSuffix(opts.OutputBase, "\\") {
			outputDir = cleaned
		} else if fi, err := os.Stat(cleaned); err == nil && fi.IsDir() {
			outputDir = cleaned
		} else {
			dir := filepath.Dir(cleaned)
			base := filepath.Base(cleaned)
			if dir != "" && dir != "." {
				outputDir = dir
			}
			if base != "" && base != "." {
				outputName = base
			}
		}
	}

	// Explicit --output-dir overrides any directory from --output
	if opts.OutputDir != "" {
		outputDir = filepath.Clean(opts.OutputDir)
	}

	// Explicit --name overrides any base name
	if opts.OutputName != "" {
		outputName = opts.OutputName
	}

	// Strip known extensions if user passed e.g. --name myflow.mmd or -o myflow.json
	for _, ext := range []string{".mmd", ".svg", ".json", ".xlsx", ".md", ".xml", ".html"} {
		if strings.HasSuffix(strings.ToLower(outputName), ext) {
			outputName = outputName[:len(outputName)-len(ext)]
			break
		}
	}

	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
		}
		opts.OutputBase = filepath.Join(outputDir, outputName)
	} else {
		opts.OutputBase = outputName
	}

	info, err := os.Stat(opts.SourceDir)
	if err == nil && !info.IsDir() {
		fmt.Printf("[*] Scanning source file: %s\n", opts.SourceDir)
	} else {
		fmt.Printf("[*] Scanning repository at: %s (recursive: %v)\n", opts.SourceDir, opts.Recursive)
	}

	var allExcludes []string
	allExcludes = append(allExcludes, opts.ExcludeDirs...)
	allExcludes = append(allExcludes, opts.IgnorePatterns...)
	if opts.IgnoreTests {
		allExcludes = append(allExcludes, discovery.CommonTestPatterns...)
		fmt.Printf("[*] Ignoring all unit test files (*_test.go, test_*.py, *Test*.cs, *.spec.*)\n")
	}

	if len(opts.ExcludeDirs) > 0 {
		fmt.Printf("[*] Excluding directories: %s\n", strings.Join(opts.ExcludeDirs, ", "))
	}
	if len(opts.IgnorePatterns) > 0 {
		fmt.Printf("[*] Ignoring patterns: %s\n", strings.Join(opts.IgnorePatterns, ", "))
	}

	scanner, err := discovery.NewScannerWithOptions(opts.SourceDir, opts.Recursive, opts.NoGitignore, allExcludes...)
	if err != nil {
		return fmt.Errorf("scanner initialization error: %w", err)
	}

	discoveredFiles, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("file discovery error: %w", err)
	}

	fmt.Printf("[+] Discovered %d relevant source and workflow files\n", len(discoveredFiles))

	// Register Analyzers
	analyzers := map[string]analyzer.Analyzer{
		discovery.LangGo:      golang.New(),
		discovery.LangCSharp:  csharp.New(),
		discovery.LangPython:  python.New(),
		discovery.LangSQL:     sql.New(),
		discovery.LangFlowXML: flowxml.New(),
		discovery.LangSSIS:    ssis.New(),
	}

	var results []*analyzer.FileAnalysisResult

	for _, file := range discoveredFiles {
		az, ok := analyzers[file.Language]
		if !ok {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			fmt.Printf("[-] Warning: Failed to read %s: %v\n", file.Path, err)
			continue
		}

		res, err := az.AnalyzeFile(file.Path, content)
		if err != nil {
			fmt.Printf("[-] Warning: Analyzer error on %s: %v\n", file.Path, err)
			continue
		}

		results = append(results, res)
	}

	// Correlate results into canonical ProcessModel
	corr := correlation.New()
	projName := filepath.Base(opts.SourceDir)
	if projName == "." || projName == "/" || projName == "\\" {
		abs, err := filepath.Abs(opts.SourceDir)
		if err == nil {
			projName = filepath.Base(abs)
		}
	}

	pm := corr.Correlate("process-"+projName, projName+" Workflow", results)

	// If no steps found from empty directory or initial run, include at least a root process step
	if len(pm.Steps) == 0 {
		pm.AddSwimlane(model.Swimlane{ID: "lane-main", Name: "Root Process"})
		pm.AddStep(model.Step{
			ID:          "step-init",
			SwimlaneID:  "lane-main",
			Name:        "Repository Scan",
			Description: fmt.Sprintf("Scanned repository %s", opts.SourceDir),
			Type:        "Task",
			Language:    "system",
		})
	}

	// Phase 5: AI Enrichment
	if opts.AIEnrichment {
		fmt.Println("[*] Running AI Enrichment layer...")
		enricher := enrichment.New()
		if err := enricher.Enrich(context.Background(), pm); err != nil {
			fmt.Printf("[-] AI enrichment notice: %v\n", err)
		} else {
			fmt.Println("[+] AI Enrichment completed successfully")
		}
	}

	fmt.Printf("[+] Process model assembled: %d swimlanes, %d steps, %d links\n",
		len(pm.Swimlanes), len(pm.Steps), len(pm.Links))

	// Exporters
	requestedFormats := strings.Split(opts.Formats, ",")
	for _, fmtName := range requestedFormats {
		cleanFmt := strings.TrimSpace(strings.ToLower(fmtName))
		switch cleanFmt {
		case "json":
			outPath := opts.OutputBase + ".json"
			if err := json.New().Export(pm, outPath); err != nil {
				return fmt.Errorf("json export failed: %w", err)
			}
			fmt.Printf("[+] Generated JSON: %s\n", outPath)

		case "mermaid", "mmd":
			m := mermaid.New()
			m.NoTruncate = opts.NoTruncate
			dType := strings.ToLower(strings.TrimSpace(opts.DiagramType))
			switch dType {
			case "all":
				pTD := opts.OutputBase + ".mmd"
				_ = os.WriteFile(pTD, []byte(m.GenerateFlowchart(pm, "TD")), 0644)
				fmt.Printf("[+] Generated Mermaid Flowchart TD: %s\n", pTD)

				pLR := opts.OutputBase + "_lr.mmd"
				_ = os.WriteFile(pLR, []byte(m.GenerateFlowchart(pm, "LR")), 0644)
				fmt.Printf("[+] Generated Mermaid Flowchart LR: %s\n", pLR)

				pJ := opts.OutputBase + "_journey.mmd"
				_ = os.WriteFile(pJ, []byte(m.GenerateJourney(pm)), 0644)
				fmt.Printf("[+] Generated Mermaid Journey: %s\n", pJ)

				pER := opts.OutputBase + "_er.mmd"
				_ = os.WriteFile(pER, []byte(m.GenerateERDiagram(pm)), 0644)
				fmt.Printf("[+] Generated Mermaid ER Diagram: %s\n", pER)

				pSeq := opts.OutputBase + "_sequence.mmd"
				_ = os.WriteFile(pSeq, []byte(m.GenerateSequenceDiagram(pm)), 0644)
				fmt.Printf("[+] Generated Mermaid Sequence Diagram: %s\n", pSeq)

				pState := opts.OutputBase + "_state.mmd"
				_ = os.WriteFile(pState, []byte(m.GenerateStateDiagram(pm)), 0644)
				fmt.Printf("[+] Generated Mermaid State Diagram: %s\n", pState)

			case "journey":
				content := m.GenerateJourney(pm)
				outPath := opts.OutputBase + ".mmd"
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("mermaid export failed: %w", err)
				}
				fmt.Printf("[+] Generated Mermaid Journey: %s (%d chars)\n", outPath, len(content))

			case "sequence", "seq":
				content := m.GenerateSequenceDiagram(pm)
				outPath := opts.OutputBase + ".mmd"
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("mermaid export failed: %w", err)
				}
				fmt.Printf("[+] Generated Mermaid Sequence Diagram: %s (%d chars)\n", outPath, len(content))

			case "state":
				content := m.GenerateStateDiagram(pm)
				outPath := opts.OutputBase + ".mmd"
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("mermaid export failed: %w", err)
				}
				fmt.Printf("[+] Generated Mermaid State Diagram: %s (%d chars)\n", outPath, len(content))

			case "er", "erdiagram":
				content := m.GenerateERDiagram(pm)
				outPath := opts.OutputBase + ".mmd"
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("mermaid export failed: %w", err)
				}
				fmt.Printf("[+] Generated Mermaid ER Diagram: %s (%d chars)\n", outPath, len(content))

			case "lr", "flowchart-lr":
				content := m.GenerateFlowchart(pm, "LR")
				outPath := opts.OutputBase + ".mmd"
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("mermaid export failed: %w", err)
				}
				fmt.Printf("[+] Generated Mermaid Flowchart LR: %s (%d chars)\n", outPath, len(content))

			default: // "td", "flowchart-td"
				content := m.GenerateFlowchart(pm, "TD")
				outPath := opts.OutputBase + ".mmd"
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("mermaid export failed: %w", err)
				}
				fmt.Printf("[+] Generated Mermaid diagram: %s (%d chars)\n", outPath, len(content))
				if len(content) > 50000 {
					fmt.Printf("    [!] Notice: Diagram exceeds 50,000 characters (GitHub's inline markdown limit). For large repositories, consider filtering with --exclude or viewing %s.svg / %s.xlsx.\n", opts.OutputBase, opts.OutputBase)
				}
			}

		case "journey":
			m := mermaid.New()
			m.NoTruncate = opts.NoTruncate
			content := m.GenerateJourney(pm)
			outPath := opts.OutputBase + "_journey.mmd"
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("journey export failed: %w", err)
			}
			fmt.Printf("[+] Generated Mermaid Journey: %s (%d chars)\n", outPath, len(content))
			pSVG := opts.OutputBase + "_journey.svg"
			s := svg.New()
			s.NoTruncate = opts.NoTruncate
			_ = s.ExportJourney(pm, pSVG)
			fmt.Printf("[+] Generated SVG Journey: %s\n", pSVG)

		case "sequence", "seq":
			m := mermaid.New()
			m.NoTruncate = opts.NoTruncate
			content := m.GenerateSequenceDiagram(pm)
			outPath := opts.OutputBase + "_sequence.mmd"
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("sequence export failed: %w", err)
			}
			fmt.Printf("[+] Generated Mermaid Sequence Diagram: %s (%d chars)\n", outPath, len(content))
			pSVG := opts.OutputBase + "_sequence.svg"
			s := svg.New()
			s.NoTruncate = opts.NoTruncate
			_ = s.ExportSequenceDiagram(pm, pSVG)
			fmt.Printf("[+] Generated SVG Sequence Diagram: %s\n", pSVG)

		case "state":
			m := mermaid.New()
			m.NoTruncate = opts.NoTruncate
			content := m.GenerateStateDiagram(pm)
			outPath := opts.OutputBase + "_state.mmd"
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("state export failed: %w", err)
			}
			fmt.Printf("[+] Generated Mermaid State Diagram: %s (%d chars)\n", outPath, len(content))
			pSVG := opts.OutputBase + "_state.svg"
			s := svg.New()
			s.NoTruncate = opts.NoTruncate
			_ = s.ExportStateDiagram(pm, pSVG)
			fmt.Printf("[+] Generated SVG State Diagram: %s\n", pSVG)

		case "er", "erdiagram":
			m := mermaid.New()
			m.NoTruncate = opts.NoTruncate
			content := m.GenerateERDiagram(pm)
			outPath := opts.OutputBase + "_er.mmd"
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("er diagram export failed: %w", err)
			}
			fmt.Printf("[+] Generated Mermaid ER Diagram: %s (%d chars)\n", outPath, len(content))
			pSVG := opts.OutputBase + "_er.svg"
			s := svg.New()
			s.NoTruncate = opts.NoTruncate
			_ = s.ExportERDiagram(pm, pSVG)
			fmt.Printf("[+] Generated SVG ER Diagram: %s\n", pSVG)

		case "markdown", "md":
			m := mermaid.New()
			m.NoTruncate = opts.NoTruncate
			outPath := opts.OutputBase + ".md"
			dType := mermaid.DiagramType(strings.ToLower(strings.TrimSpace(opts.DiagramType)))
			if err := m.ExportMarkdown(pm, outPath, dType); err != nil {
				return fmt.Errorf("markdown export failed: %w", err)
			}
			fmt.Printf("[+] Generated Markdown document: %s\n", outPath)

		case "excel", "xlsx":
			outPath := opts.OutputBase + ".xlsx"
			if opts.CustomWorkbook {
				if err := excel.New().ExportCustomWorkbook(pm, outPath); err != nil {
					return fmt.Errorf("custom excel export failed: %w", err)
				}
				fmt.Printf("[+] Generated Excel Data Visualizer Schema (custom workbook): %s\n", outPath)
			} else {
				if err := excel.New().Export(pm, outPath); err != nil {
					return fmt.Errorf("excel export failed: %w", err)
				}
				fmt.Printf("[+] Generated Visio Excel workbook: %s\n", outPath)
			}

		case "custom-excel", "excel-custom", "custom-workbook", "visualizer-excel":
			outPath := opts.OutputBase + "_custom.xlsx"
			if opts.Formats == "custom-excel" || opts.Formats == "excel-custom" || opts.Formats == "custom-workbook" {
				outPath = opts.OutputBase + ".xlsx"
			}
			if err := excel.New().ExportCustomWorkbook(pm, outPath); err != nil {
				return fmt.Errorf("custom excel export failed: %w", err)
			}
			fmt.Printf("[+] Generated Excel Data Visualizer Schema (custom workbook): %s\n", outPath)

		case "svg":
			s := svg.New()
			s.NoTruncate = opts.NoTruncate
			dType := strings.ToLower(strings.TrimSpace(opts.DiagramType))
			switch dType {
			case "all":
				pTD := opts.OutputBase + ".svg"
				_ = s.ExportFlowchart(pm, pTD, "TD")
				fmt.Printf("[+] Generated SVG Flowchart TD: %s\n", pTD)

				pLR := opts.OutputBase + "_lr.svg"
				_ = s.ExportFlowchart(pm, pLR, "LR")
				fmt.Printf("[+] Generated SVG Flowchart LR: %s\n", pLR)

				pJ := opts.OutputBase + "_journey.svg"
				_ = s.ExportJourney(pm, pJ)
				fmt.Printf("[+] Generated SVG Journey: %s\n", pJ)

				pER := opts.OutputBase + "_er.svg"
				_ = s.ExportERDiagram(pm, pER)
				fmt.Printf("[+] Generated SVG ER Diagram: %s\n", pER)

				pSeq := opts.OutputBase + "_sequence.svg"
				_ = s.ExportSequenceDiagram(pm, pSeq)
				fmt.Printf("[+] Generated SVG Sequence Diagram: %s\n", pSeq)

				pState := opts.OutputBase + "_state.svg"
				_ = s.ExportStateDiagram(pm, pState)
				fmt.Printf("[+] Generated SVG State Diagram: %s\n", pState)

			case "journey":
				outPath := opts.OutputBase + ".svg"
				if err := s.ExportJourney(pm, outPath); err != nil {
					return fmt.Errorf("journey svg export failed: %w", err)
				}
				fmt.Printf("[+] Generated SVG Journey: %s\n", outPath)

			case "sequence", "seq":
				outPath := opts.OutputBase + ".svg"
				if err := s.ExportSequenceDiagram(pm, outPath); err != nil {
					return fmt.Errorf("sequence svg export failed: %w", err)
				}
				fmt.Printf("[+] Generated SVG Sequence Diagram: %s\n", outPath)

			case "state":
				outPath := opts.OutputBase + ".svg"
				if err := s.ExportStateDiagram(pm, outPath); err != nil {
					return fmt.Errorf("state svg export failed: %w", err)
				}
				fmt.Printf("[+] Generated SVG State Diagram: %s\n", outPath)

			case "er", "erdiagram":
				outPath := opts.OutputBase + ".svg"
				if err := s.ExportERDiagram(pm, outPath); err != nil {
					return fmt.Errorf("er svg export failed: %w", err)
				}
				fmt.Printf("[+] Generated SVG ER Diagram: %s\n", outPath)

			case "lr", "flowchart-lr":
				outPath := opts.OutputBase + ".svg"
				if err := s.ExportFlowchart(pm, outPath, "LR"); err != nil {
					return fmt.Errorf("flowchart lr svg export failed: %w", err)
				}
				fmt.Printf("[+] Generated SVG Flowchart LR: %s\n", outPath)

			default: // "td", "flowchart-td"
				outPath := opts.OutputBase + ".svg"
				if err := s.ExportFlowchart(pm, outPath, "TD"); err != nil {
					return fmt.Errorf("svg export failed: %w", err)
				}
				fmt.Printf("[+] Generated SVG diagram: %s\n", outPath)
			}

		default:
			if cleanFmt != "" {
				fmt.Printf("[-] Warning: Unknown format '%s' (supported: mermaid, excel, custom-excel, json, svg)\n", cleanFmt)
			}
		}
	}

	return nil
}
