package mermaid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeflow/pkg/model"
)

func TestMermaidExporter(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Order Pipeline")
	pm.AddSwimlane(model.Swimlane{ID: "lane-1", Name: "API Gateway"})
	pm.AddSwimlane(model.Swimlane{ID: "lane-2", Name: "Billing Service"})

	pm.AddStep(model.Step{
		ID:         "step-1",
		SwimlaneID: "lane-1",
		Name:       "POST /checkout",
		Type:       "Endpoint",
	})
	pm.AddStep(model.Step{
		ID:         "step-2",
		SwimlaneID: "lane-2",
		Name:       "Charge Card",
		Type:       "Task",
	})

	pm.AddLink(model.Link{
		ID:             "l1",
		SourceStepID:   "step-1",
		TargetStepID:   "step-2",
		Label:          "Invoke Payment",
		IsCrossService: true,
	})

	exporter := New()
	output := exporter.Generate(pm)

	if !strings.Contains(output, "flowchart TD") {
		t.Errorf("expected 'flowchart TD' header")
	}
	if !strings.Contains(output, "subgraph lane_lane_1 [\"API Gateway\"]") {
		t.Errorf("expected subgraph for lane-1, got:\n%s", output)
	}
	if !strings.Contains(output, "-.->|\"Invoke Payment\"|") {
		t.Errorf("expected cross service arrow, got:\n%s", output)
	}

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "output.mmd")
	if err := exporter.Export(pm, outPath); err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil || len(content) == 0 {
		t.Fatalf("failed to read output file")
	}
}

func TestMermaidFlowchartLR(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Horizontal Pipeline")
	pm.AddSwimlane(model.Swimlane{ID: "l1", Name: "Worker"})
	pm.AddStep(model.Step{ID: "s1", SwimlaneID: "l1", Name: "Process", Type: "Task"})

	exporter := New()
	output := exporter.GenerateFlowchart(pm, "LR")

	if !strings.Contains(output, "flowchart LR") {
		t.Errorf("expected 'flowchart LR', got: %s", output)
	}
}

func TestMermaidJourneyFlow(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Checkout Journey")
	pm.AddSwimlane(model.Swimlane{ID: "l1", Name: "Customer Portal"})
	pm.AddSwimlane(model.Swimlane{ID: "l2", Name: "Payment Service"})

	pm.AddStep(model.Step{
		ID:         "s1",
		SwimlaneID: "l1",
		Name:       "Submit Order: Cart",
		Type:       "Endpoint",
	})
	pm.AddStep(model.Step{
		ID:         "s2",
		SwimlaneID: "l2",
		Name:       "Verify Balance",
		Type:       "Task",
	})

	exporter := New()
	output := exporter.GenerateJourney(pm)

	if !strings.Contains(output, "flowchart LR") {
		t.Errorf("expected 'flowchart LR' header for universal journey, got:\n%s", output)
	}
	if !strings.Contains(output, "Stage 1: Customer Portal") {
		t.Errorf("expected Stage 1: Customer Portal, got:\n%s", output)
	}
	if !strings.Contains(output, "stage_1 ==> stage_2") {
		t.Errorf("expected stage_1 ==> stage_2 milestone arrow, got:\n%s", output)
	}
}

func TestMermaidRawJourney(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Checkout Journey")
	pm.AddSwimlane(model.Swimlane{ID: "l1", Name: "Customer Portal"})
	pm.AddStep(model.Step{
		ID:         "s1",
		SwimlaneID: "l1",
		Name:       "Submit Order: Cart",
		Type:       "Endpoint",
	})

	exporter := New()
	output := exporter.GenerateRawJourney(pm)

	if !strings.Contains(output, "journey") {
		t.Errorf("expected 'journey' header, got:\n%s", output)
	}
	if !strings.Contains(output, "section Customer Portal") {
		t.Errorf("expected section Customer Portal, got:\n%s", output)
	}
}

func TestMermaidSequenceDiagram(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Auth Sequence")
	pm.AddSwimlane(model.Swimlane{ID: "l1", Name: "Client"})
	pm.AddSwimlane(model.Swimlane{ID: "l2", Name: "Auth Server"})

	pm.AddStep(model.Step{ID: "s1", SwimlaneID: "l1", Name: "Login Request"})
	pm.AddStep(model.Step{ID: "s2", SwimlaneID: "l2", Name: "Generate JWT"})

	pm.AddLink(model.Link{
		ID:           "l1",
		SourceStepID: "s1",
		TargetStepID: "s2",
		Label:        "POST /auth/token",
	})

	exporter := New()
	output := exporter.GenerateSequenceDiagram(pm)

	if !strings.Contains(output, "sequenceDiagram") {
		t.Errorf("expected 'sequenceDiagram' header, got:\n%s", output)
	}
	if !strings.Contains(output, "participant P1 as Client") {
		t.Errorf("expected participant P1 as Client, got:\n%s", output)
	}
	if !strings.Contains(output, "P1->>P2: POST /auth/token") {
		t.Errorf("expected sequence arrow P1->>P2, got:\n%s", output)
	}
}

func TestMermaidStateDiagram(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Pipeline States")
	pm.AddSwimlane(model.Swimlane{ID: "l1", Name: "Ingestion"})
	pm.AddSwimlane(model.Swimlane{ID: "l2", Name: "Transformation"})

	exporter := New()
	output := exporter.GenerateStateDiagram(pm)

	if !strings.Contains(output, "stateDiagram-v2") {
		t.Errorf("expected 'stateDiagram-v2' header, got:\n%s", output)
	}
	if !strings.Contains(output, "state \"Ingestion\" as State_1") {
		t.Errorf("expected State_1 for Ingestion, got:\n%s", output)
	}
}

func TestMermaidERDiagram(t *testing.T) {
	pm := model.NewProcessModel("pm1", "Billing ER")
	pm.AddSwimlane(model.Swimlane{ID: "sql_db", Name: "SQL Database"})

	pm.AddStep(model.Step{
		ID:         "s1",
		SwimlaneID: "sql_db",
		Name:       "CREATE TABLE Orders",
		Type:       "TableOperation",
		Metadata: map[string]any{
			"table": "Orders",
			"columns": []map[string]string{
				{"name": "OrderID", "type": "BIGINT", "key": "PK"},
				{"name": "TotalAmount", "type": "DECIMAL_18_2"},
				{"name": "CustomerID", "type": "VARCHAR_32", "key": "FK"},
			},
		},
	})

	pm.AddStep(model.Step{
		ID:         "s2",
		SwimlaneID: "sql_db",
		Name:       "CREATE TABLE Customers",
		Type:       "TableOperation",
		Metadata: map[string]any{
			"table": "Customers",
			"columns": []map[string]string{
				{"name": "CustomerID", "type": "VARCHAR_32", "key": "PK"},
				{"name": "Name", "type": "NVARCHAR_100"},
			},
		},
	})

	pm.AddLink(model.Link{
		ID:           "l1",
		SourceStepID: "s2",
		TargetStepID: "s1",
		Label:        "places",
	})

	exporter := New()
	output := exporter.GenerateERDiagram(pm)

	if !strings.Contains(output, "erDiagram") {
		t.Errorf("expected 'erDiagram' header, got:\n%s", output)
	}
	if !strings.Contains(output, "Orders {") {
		t.Errorf("expected Orders entity block, got:\n%s", output)
	}
	if !strings.Contains(output, "BIGINT OrderID PK") {
		t.Errorf("expected PK column in Orders, got:\n%s", output)
	}
	if !strings.Contains(output, "Customers ||--o{ Orders : \"places\"") {
		t.Errorf("expected relationship between Customers and Orders, got:\n%s", output)
	}
}
