package svg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeflow/pkg/model"
)

func TestSVGExporter(t *testing.T) {
	pm := model.NewProcessModel("pm-svg", "SVG Test Flow")
	pm.AddSwimlane(model.Swimlane{ID: "lane-1", Name: "Frontend"})
	pm.AddSwimlane(model.Swimlane{ID: "lane-2", Name: "Backend"})

	pm.AddStep(model.Step{
		ID:          "step-1",
		SwimlaneID:  "lane-1",
		Name:        "User Click",
		Description: "Clicks submit button",
		Type:        "Task",
	})
	pm.AddStep(model.Step{
		ID:          "step-2",
		SwimlaneID:  "lane-2",
		Name:        "Process API",
		Description: "Handles API request",
		Type:        "Endpoint",
	})
	pm.AddLink(model.Link{
		ID:             "l1",
		SourceStepID:   "step-1",
		TargetStepID:   "step-2",
		Label:          "Submit Request",
		IsCrossService: true,
	})

	exporter := New()
	svgContent := exporter.Generate(pm)

	if !strings.Contains(svgContent, "<svg") || !strings.Contains(svgContent, "</svg>") {
		t.Errorf("expected valid SVG structure")
	}
	if !strings.Contains(svgContent, "Frontend") || !strings.Contains(svgContent, "Backend") {
		t.Errorf("expected swimlane names in SVG")
	}
	if !strings.Contains(svgContent, "User Click") || !strings.Contains(svgContent, "Process API") {
		t.Errorf("expected step names in SVG")
	}

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "diagram.svg")
	if err := exporter.Export(pm, outPath); err != nil {
		t.Fatalf("failed to export SVG: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil || len(data) == 0 {
		t.Fatalf("expected non-empty SVG file on disk")
	}
}

func TestSVGExporterDistinctFlows(t *testing.T) {
	pm := model.NewProcessModel("pm-pipeline", "ETL Pipeline Process")
	pm.AddSwimlane(model.Swimlane{ID: "lane-preflight", Name: "Preflight"})
	pm.AddSwimlane(model.Swimlane{ID: "lane-flow", Name: "Main Flow"})
	pm.AddSwimlane(model.Swimlane{ID: "lane-db", Name: "Database"})

	pm.AddStep(model.Step{
		ID:          "s1",
		SwimlaneID:  "lane-preflight",
		Name:        "CheckDB",
		Description: "SELECT 1",
		Type:        "DatabaseQuery",
	})
	pm.AddStep(model.Step{
		ID:          "s2",
		SwimlaneID:  "lane-flow",
		Name:        "ExtractData",
		Description: "Extract via script",
		Type:        "Script",
	})
	pm.AddStep(model.Step{
		ID:          "s3",
		SwimlaneID:  "lane-db",
		Name:        "dbo.BillingData",
		Description: "Target table",
		Type:        "DatabaseTable",
	})

	pm.AddLink(model.Link{
		ID:           "l1",
		SourceStepID: "s1",
		TargetStepID: "s2",
		Label:        "preflight passed",
	})
	pm.AddLink(model.Link{
		ID:             "l2",
		SourceStepID:   "s2",
		TargetStepID:   "s3",
		Label:          "INSERT INTO",
		IsCrossService: true,
	})

	exporter := New()

	// 1. Flowchart TD
	tdSVG := exporter.GenerateFlowchartTD(pm)
	if !strings.Contains(tdSVG, "Flowchart TD") || !strings.Contains(tdSVG, "<svg") {
		t.Errorf("expected Flowchart TD SVG")
	}

	// 2. Flowchart LR
	lrSVG := exporter.GenerateFlowchartLR(pm)
	if !strings.Contains(lrSVG, "Flowchart LR") || !strings.Contains(lrSVG, "<svg") {
		t.Errorf("expected Flowchart LR SVG")
	}

	// 3. Journey
	journeySVG := exporter.GenerateJourney(pm)
	if !strings.Contains(journeySVG, "User / Data Journey") || !strings.Contains(journeySVG, "STAGE 1") {
		t.Errorf("expected Journey SVG with stages")
	}

	// 4. Sequence Diagram
	seqSVG := exporter.GenerateSequenceDiagram(pm)
	if !strings.Contains(seqSVG, "Sequence Diagram") || !strings.Contains(seqSVG, "stroke-dasharray=\"6,4\"") {
		t.Errorf("expected Sequence Diagram SVG with lifelines")
	}

	// 5. State Diagram
	stateSVG := exporter.GenerateStateDiagram(pm)
	if !strings.Contains(stateSVG, "State Diagram") || !strings.Contains(stateSVG, "[*] Start") {
		t.Errorf("expected State Diagram SVG with start/end states")
	}

	// 6. ER Diagram
	erSVG := exporter.GenerateERDiagram(pm)
	if !strings.Contains(erSVG, "ER Diagram") || !strings.Contains(erSVG, "[TABLE]") {
		t.Errorf("expected ER Diagram SVG with table entities")
	}

	// Ensure all generated SVGs are distinct from each other
	svgMap := map[string]string{
		"TD":       tdSVG,
		"LR":       lrSVG,
		"Journey":  journeySVG,
		"Sequence": seqSVG,
		"State":    stateSVG,
		"ER":       erSVG,
	}

	for k1, v1 := range svgMap {
		for k2, v2 := range svgMap {
			if k1 != k2 && v1 == v2 {
				t.Errorf("expected SVG for %s and %s to be distinct, but they were identical", k1, k2)
			}
		}
	}
}

