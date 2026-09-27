package python

import (
	"testing"
)

func TestPythonAnalyzer(t *testing.T) {
	code := `from fastapi import FastAPI
import database

app = FastAPI()

@app.get("/items/{item_id}")
def read_item(item_id: int):
    return get_item_data(item_id)

def get_item_data(item_id: int):
    return database.session.query("SELECT * FROM items")
`

	analyzer := New()
	result, err := analyzer.AnalyzeFile("main.py", []byte(code))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	foundEndpoint := false
	foundDB := false

	for _, step := range result.Steps {
		if step.Type == "Endpoint" && step.Name == "read_item" {
			foundEndpoint = true
			if step.Metadata["route"] != "/items/{item_id}" {
				t.Errorf("expected route /items/{item_id}, got %v", step.Metadata["route"])
			}
			if step.Metadata["method"] != "GET" {
				t.Errorf("expected method GET, got %v", step.Metadata["method"])
			}
		}
		if step.Type == "DatabaseQuery" {
			foundDB = true
		}
	}

	if !foundEndpoint {
		t.Errorf("expected to find FastAPI endpoint step")
	}
	if !foundDB {
		t.Errorf("expected to find DB query step")
	}
}
