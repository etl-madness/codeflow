package enrichment

import (
	"context"
	"strings"
	"testing"

	"codeflow/pkg/model"
)

func TestEnrichmentFallback(t *testing.T) {
	pm := model.NewProcessModel("p1", "Test")
	pm.AddStep(model.Step{
		ID:          "s1",
		Name:        "CreateOrderAsync",
		Type:        "Task",
		Description: "technical code",
	})
	pm.AddStep(model.Step{
		ID:          "s2",
		Name:        "AuthorizePaymentCard",
		Type:        "Endpoint",
		Description: "technical code",
	})

	enricher := New()
	// No API keys set, should run semantic fallback
	err := enricher.Enrich(context.Background(), pm)
	if err != nil {
		t.Fatalf("enrichment failed: %v", err)
	}

	step1 := pm.FindStep("s1")
	if !strings.Contains(step1.Description, "customer order") {
		t.Errorf("expected customer order in fallback description, got: %s", step1.Description)
	}

	step2 := pm.FindStep("s2")
	if !strings.Contains(step2.Description, "payment") {
		t.Errorf("expected payment in fallback description, got: %s", step2.Description)
	}
}

func TestEnrichmentWithMockLLM(t *testing.T) {
	pm := model.NewProcessModel("p1", "Test")
	pm.AddStep(model.Step{
		ID:          "s1",
		Name:        "fn_123",
		Type:        "Function",
		Description: "old desc",
	})

	enricher := New()
	enricher.SetMockHandler(func(prompt string) (string, error) {
		return `[{"id": "s1", "business_description": "Validate customer discount eligibility."}]`, nil
	})

	err := enricher.Enrich(context.Background(), pm)
	if err != nil {
		t.Fatalf("mock enrichment failed: %v", err)
	}

	step1 := pm.FindStep("s1")
	if step1.Description != "Validate customer discount eligibility." {
		t.Errorf("unexpected description: %s", step1.Description)
	}
}
