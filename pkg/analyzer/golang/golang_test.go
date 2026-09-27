package golang

import (
	"testing"
)

func TestGoAnalyzer(t *testing.T) {
	code := `package main

import (
	"database/sql"
	"net/http"
)

func SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/orders", HandleOrders)
}

func HandleOrders(w http.ResponseWriter, r *http.Request) {
	ProcessPayment()
}

func ProcessPayment() {
	var db *sql.DB
	db.Exec("INSERT INTO payments (id, amount) VALUES (1, 100)")
}
`

	analyzer := New()
	result, err := analyzer.AnalyzeFile("main.go", []byte(code))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(result.Swimlanes) == 0 {
		t.Errorf("expected at least 1 swimlane")
	}

	foundRoute := false
	foundDB := false
	foundCall := false

	for _, step := range result.Steps {
		if step.Type == "Endpoint" && step.Metadata["route"] == "/api/v1/orders" {
			foundRoute = true
		}
		if step.Type == "DatabaseQuery" && step.Metadata["operation"] == "Exec" {
			foundDB = true
		}
	}

	for _, link := range result.Links {
		if link.Label == "dispatches to" || link.Label == "calls" {
			foundCall = true
		}
	}

	if !foundRoute {
		t.Errorf("expected to find HTTP route endpoint")
	}
	if !foundDB {
		t.Errorf("expected to find DB Exec query step")
	}
	if !foundCall {
		t.Errorf("expected to find call link")
	}
}
