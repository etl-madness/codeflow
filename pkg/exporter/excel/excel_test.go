package excel

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"codeflow/pkg/model"
)

func TestExcelExporter(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "output.xlsx")

	pm := model.NewProcessModel("pm1", "Order Pipeline")
	pm.AddSwimlane(model.Swimlane{ID: "lane-1", Name: "Customer Portal"})
	pm.AddStep(model.Step{
		ID:          "step-1",
		SwimlaneID:  "lane-1",
		Name:        "Submit Order",
		Description: "Customer submits cart",
		Type:        "Task",
	})
	pm.AddStep(model.Step{
		ID:          "step-2",
		SwimlaneID:  "lane-1",
		Name:        "Payment",
		Description: "Charge card",
		Type:        "Endpoint",
	})
	pm.AddLink(model.Link{
		ID:           "l1",
		SourceStepID: "step-1",
		TargetStepID: "step-2",
		Label:        "Proceed",
	})

	exporter := New()
	if err := exporter.Export(pm, outPath); err != nil {
		t.Fatalf("failed to export excel: %v", err)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed to open exported excel: %v", err)
	}
	defer f.Close()

	// Verify headers
	h1, _ := f.GetCellValue("Process Map", "A1")
	h2, _ := f.GetCellValue("Process Map", "B1")
	h3, _ := f.GetCellValue("Process Map", "C1")
	h4, _ := f.GetCellValue("Process Map", "D1")
	h5, _ := f.GetCellValue("Process Map", "E1")
	h6, _ := f.GetCellValue("Process Map", "F1")

	if h1 != "Process Step ID" || h2 != "Step Description" || h3 != "Next Step ID" ||
		h4 != "Connector Label" || h5 != "Step Type" || h6 != "Owner / Function" {
		t.Errorf("headers mismatch: got %v %v %v %v %v %v", h1, h2, h3, h4, h5, h6)
	}

	// Verify row 2 data
	stepID, _ := f.GetCellValue("Process Map", "A2")
	if stepID != "step-1" {
		t.Errorf("expected step-1, got %s", stepID)
	}
	nextID, _ := f.GetCellValue("Process Map", "C2")
	if nextID != "step-2" {
		t.Errorf("expected nextID step-2, got %s", nextID)
	}
	label, _ := f.GetCellValue("Process Map", "D2")
	if label != "Proceed" {
		t.Errorf("expected label Proceed, got %s", label)
	}
}

func TestCustomWorkbookExporter(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "custom_output.xlsx")

	pm := model.NewProcessModel("pm2", "Billing Service")
	pm.AddSwimlane(model.Swimlane{ID: "lane-api", Name: "API Gateway"})
	pm.AddSwimlane(model.Swimlane{ID: "lane-db", Name: "PostgreSQL Database"})

	pm.AddStep(model.Step{
		ID:          "step-checkout",
		SwimlaneID:  "lane-api",
		Name:        "Checkout",
		Description: "Process checkout request",
		Type:        "Endpoint",
		Language:    "go",
		SourceFile:  "checkout.go",
		LineNumber:  45,
	})
	pm.AddStep(model.Step{
		ID:          "step-sql-insert",
		SwimlaneID:  "lane-db",
		Name:        "InsertOrder",
		Description: "INSERT INTO orders",
		Type:        "TableOperation",
		Language:    "sql",
		SourceFile:  "schema.sql",
		LineNumber:  12,
	})

	pm.AddLink(model.Link{
		ID:             "link-api-db",
		SourceStepID:   "step-checkout",
		TargetStepID:   "step-sql-insert",
		Label:          "persists order",
		IsCrossService: true,
	})

	exporter := New()
	if err := exporter.ExportCustomWorkbook(pm, outPath); err != nil {
		t.Fatalf("failed to export custom workbook: %v", err)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed to open exported custom workbook: %v", err)
	}
	defer f.Close()

	// Check sheets exist
	sheetNames := f.GetSheetList()
	expectedSheets := []string{"Dashboard", "Visual Process Map", "Process Steps", "Traceability Matrix", "Services & Swimlanes"}
	for _, expected := range expectedSheets {
		found := false
		for _, s := range sheetNames {
			if s == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected sheet '%s' in sheets: %v", expected, sheetNames)
		}
	}

	// Verify Dashboard KPI values
	kpi1, _ := f.GetCellValue("Dashboard", "A5")
	if kpi1 != "2" {
		t.Errorf("expected 2 steps in KPI, got %s", kpi1)
	}
	kpiCross, _ := f.GetCellValue("Dashboard", "E5")
	if kpiCross != "1" {
		t.Errorf("expected 1 cross-service link in KPI, got %s", kpiCross)
	}

	// Verify Process Steps headers
	h1, _ := f.GetCellValue("Process Steps", "A1")
	h2, _ := f.GetCellValue("Process Steps", "B1")
	if h1 != "Process Step ID" || h2 != "Step Description" {
		t.Errorf("expected standard schema headers, got %s, %s", h1, h2)
	}

	// Verify Traceability Matrix
	traceSrc, _ := f.GetCellValue("Traceability Matrix", "C2")
	traceTgt, _ := f.GetCellValue("Traceability Matrix", "F2")
	traceScope, _ := f.GetCellValue("Traceability Matrix", "I2")
	if traceSrc != "step-checkout" || traceTgt != "step-sql-insert" || traceScope != "CROSS-SERVICE" {
		t.Errorf("unexpected traceability values: %s -> %s (%s)", traceSrc, traceTgt, traceScope)
	}
}
