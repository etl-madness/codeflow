package model

import (
	"testing"
)

func TestProcessModelOperations(t *testing.T) {
	pm := NewProcessModel("proc-1", "Test Process")

	swimlane := Swimlane{ID: "lane-1", Name: "Order Service", Description: "Handles orders"}
	pm.AddSwimlane(swimlane)
	// Duplicate swimlane should be ignored
	pm.AddSwimlane(swimlane)
	if len(pm.Swimlanes) != 1 {
		t.Fatalf("expected 1 swimlane, got %d", len(pm.Swimlanes))
	}

	foundLane := pm.FindSwimlane("lane-1")
	if foundLane == nil || foundLane.Name != "Order Service" {
		t.Fatalf("failed to find swimlane")
	}

	step1 := Step{
		ID:          "step-1",
		SwimlaneID:  "lane-1",
		Name:        "CreateOrder",
		Description: "Creates a customer order",
		Type:        "Endpoint",
		Language:    "go",
		SourceFile:  "order.go",
		LineNumber:  25,
	}
	pm.AddStep(step1)

	step2 := Step{
		ID:          "step-2",
		SwimlaneID:  "lane-1",
		Name:        "SaveOrderDB",
		Description: "Inserts into orders table",
		Type:        "DatabaseQuery",
		Language:    "sql",
		SourceFile:  "queries.sql",
		LineNumber:  10,
	}
	pm.AddStep(step2)

	if len(pm.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(pm.Steps))
	}

	foundStep := pm.FindStep("step-1")
	if foundStep == nil || foundStep.Name != "CreateOrder" {
		t.Fatalf("failed to find step")
	}

	link := Link{
		ID:             "link-1",
		SourceStepID:   "step-1",
		TargetStepID:   "step-2",
		Label:          "Persist",
		IsCrossService: false,
	}
	pm.AddLink(link)
	pm.AddLink(link) // Duplicate link check

	if len(pm.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(pm.Links))
	}
}
