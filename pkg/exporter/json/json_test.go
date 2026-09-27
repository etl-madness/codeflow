package json

import (
	"os"
	"path/filepath"
	"testing"

	"codeflow/pkg/model"
)

func TestJSONExporter(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "model.json")

	pm := model.NewProcessModel("test-1", "Test Pipeline")
	pm.AddSwimlane(model.Swimlane{ID: "lane-1", Name: "Service A"})
	pm.AddStep(model.Step{
		ID:          "step-1",
		SwimlaneID:  "lane-1",
		Name:        "Handler",
		Description: "Handles request",
		Type:        "Endpoint",
		Language:    "go",
		SourceFile:  "main.go",
		LineNumber:  12,
	})

	exporter := New()
	if err := exporter.Export(pm, outputPath); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("expected non-empty json output")
	}
}
