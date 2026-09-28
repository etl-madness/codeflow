package correlation

import (
	"testing"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

func TestCorrelationEngine(t *testing.T) {
	// 1. Go file with db query referencing 'payments' table
	goResult := &analyzer.FileAnalysisResult{
		Swimlanes: []model.Swimlane{
			{ID: "lane-go", Name: "Go Backend"},
		},
		Steps: []model.Step{
			{
				ID:          "go-handler",
				SwimlaneID:  "lane-go",
				Name:        "ProcessPayment",
				Description: "Handles payments",
				Type:        "Endpoint",
				Language:    "go",
				Metadata: map[string]any{
					"route":  "/api/payments",
					"method": "POST",
				},
			},
			{
				ID:          "go-db",
				SwimlaneID:  "lane-go",
				Name:        "DB Exec",
				Description: "INSERT INTO payments",
				Type:        "DatabaseQuery",
				Language:    "go",
				Metadata: map[string]any{
					"query": "INSERT INTO payments (id, amount) VALUES (1, 100)",
				},
			},
		},
		Links: []model.Link{
			{
				ID:           "go-handler->go-db",
				SourceStepID: "go-handler",
				TargetStepID: "go-db",
				Label:        "calls",
			},
		},
	}

	// 2. SQL file defining 'payments' table
	sqlResult := &analyzer.FileAnalysisResult{
		Swimlanes: []model.Swimlane{
			{ID: "lane-sql", Name: "Database"},
		},
		Steps: []model.Step{
			{
				ID:          "sql-table-payments",
				SwimlaneID:  "lane-sql",
				Name:        "CREATE TABLE payments",
				Description: "Payments schema",
				Type:        "TableOperation",
				Language:    "sql",
				Metadata: map[string]any{
					"table": "payments",
				},
			},
		},
	}

	engine := New()
	pm := engine.Correlate("model-1", "Test Model", []*analyzer.FileAnalysisResult{goResult, sqlResult})

	if len(pm.Swimlanes) != 2 {
		t.Errorf("expected 2 swimlanes, got %d", len(pm.Swimlanes))
	}

	if len(pm.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(pm.Steps))
	}

	// Should have cross-service link between go-db and sql-table-payments
	foundCrossLink := false
	for _, l := range pm.Links {
		if l.IsCrossService && l.SourceStepID == "go-db" && l.TargetStepID == "sql-table-payments" {
			foundCrossLink = true
		}
	}

	if !foundCrossLink {
		t.Errorf("expected cross-service link between go DB step and SQL table step")
	}
}

func TestCorrelationLiquibaseSQLFile(t *testing.T) {
	// 1. Liquibase changeset executing <sqlFile path="../ddl/dbo_usp_GetCustomer.1.sql"/>
	lbResult := &analyzer.FileAnalysisResult{
		Swimlanes: []model.Swimlane{
			{ID: "lane-lb", Name: "Liquibase (customer_changeset.1)"},
		},
		Steps: []model.Step{
			{
				ID:          "cs-getcustomer",
				SwimlaneID:  "lane-lb",
				Name:        "me:dbbo_usp_GetCustomer.1",
				Description: "Execute <sqlFile>: dbo_usp_GetCustomer.1.sql",
				Type:        "StoredProcedure",
				Language:    "liquibase",
				SourceFile:  "examples/liquibase/changeset/customer_changeset.1.xml",
				Metadata: map[string]any{
					"is_liquibase":  true,
					"sql_file":      "../ddl/dbo_usp_GetCustomer.1.sql",
					"sql_file_base": "dbo_usp_GetCustomer.1.sql",
				},
			},
		},
	}

	// 2. SQL file defining CREATE OR ALTER PROCEDURE dbo.usp_GetCustomer
	sqlResult := &analyzer.FileAnalysisResult{
		Swimlanes: []model.Swimlane{
			{ID: "lane-sql-cust", Name: "SQL Database (dbo_usp_GetCustomer.1)"},
		},
		Steps: []model.Step{
			{
				ID:          "sql-usp-getcustomer",
				SwimlaneID:  "lane-sql-cust",
				Name:        "Proc: usp_GetCustomer",
				Description: "Stored Procedure Proc: usp_GetCustomer",
				Type:        "StoredProcedure",
				Language:    "sql",
				SourceFile:  "examples/liquibase/ddl/dbo_usp_GetCustomer.1.sql",
				Metadata: map[string]any{
					"procedure": "usp_GetCustomer",
					"schema":    "dbo",
				},
			},
		},
	}

	engine := New()
	pm := engine.Correlate("model-lb-sql", "Test LB SQL", []*analyzer.FileAnalysisResult{lbResult, sqlResult})

	foundSQLFileLink := false
	for _, l := range pm.Links {
		if l.IsCrossService && l.SourceStepID == "cs-getcustomer" && l.TargetStepID == "sql-usp-getcustomer" {
			if l.Label == "calls <sqlFile>" {
				foundSQLFileLink = true
			}
		}
	}

	if !foundSQLFileLink {
		t.Errorf("expected cross-service 'calls <sqlFile>' link between Liquibase changeset and SQL procedure step")
	}
}
