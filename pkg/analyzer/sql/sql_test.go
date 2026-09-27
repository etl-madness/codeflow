package sql

import (
	"testing"
)

func TestSQLAnalyzer(t *testing.T) {
	sqlScript := `
CREATE TABLE customers (
    id INT PRIMARY KEY,
    name VARCHAR(100)
);

CREATE VIEW active_customers AS
SELECT id, name FROM customers WHERE active = 1;

CREATE PROCEDURE dbo.ProcessOrders
AS
BEGIN
    SELECT * FROM orders;
END
GO

INSERT INTO customers (id, name) VALUES (1, 'Alice');
`

	analyzer := New()
	result, err := analyzer.AnalyzeFile("orders.sql", []byte(sqlScript))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	foundTable := false
	foundView := false
	foundProc := false
	foundInsert := false

	for _, step := range result.Steps {
		if step.Type == "TableOperation" && step.Metadata["operation"] == "create" {
			foundTable = true
		}
		if step.Type == "View" {
			foundView = true
		}
		if step.Type == "StoredProcedure" {
			foundProc = true
		}
		if step.Type == "TableOperation" && step.Metadata["operation"] == "INSERT" {
			foundInsert = true
		}
	}

	if !foundTable {
		t.Errorf("expected to find CREATE TABLE step")
	}
	if !foundView {
		t.Errorf("expected to find CREATE VIEW step")
	}
	if !foundProc {
		t.Errorf("expected to find StoredProcedure step")
	}
	if !foundInsert {
		t.Errorf("expected to find INSERT step")
	}
}

func TestTSQLTempTablesAndTypes(t *testing.T) {
	tsqlScript := `
CREATE TABLE #databases(
    database_id int, 
    database_name sysname
);
GO

CREATE TABLE #dependencies(
    referencing_servername varchar(max),
    referencing_id int,
    [IsActive] [nvarchar](3) NULL
);
GO

CREATE PROCEDURE [dbo].[get_crossdatabase_dependencies] AS
BEGIN
    INSERT INTO #databases(database_id, database_name)
    SELECT database_id, [database_name] FROM cross_db_databases;
END
GO
`
	analyzer := New()
	result, err := analyzer.AnalyzeFile("test_tsql.sql", []byte(tsqlScript))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	foundDatabases := false
	foundDependencies := false
	foundProc := false

	for _, step := range result.Steps {
		if step.Name == "CREATE TABLE #databases" {
			foundDatabases = true
			cols, ok := step.Metadata["columns"].([]map[string]string)
			if !ok || len(cols) != 2 {
				t.Errorf("expected 2 columns for #databases, got %v", cols)
			} else {
				if cols[0]["name"] != "database_id" || cols[0]["type"] != "int" {
					t.Errorf("unexpected col 0: %v", cols[0])
				}
				if cols[1]["name"] != "database_name" || cols[1]["type"] != "sysname" {
					t.Errorf("unexpected col 1: %v", cols[1])
				}
			}
		}
		if step.Name == "CREATE TABLE #dependencies" {
			foundDependencies = true
			cols, ok := step.Metadata["columns"].([]map[string]string)
			if !ok || len(cols) != 3 {
				t.Errorf("expected 3 columns for #dependencies, got %v", cols)
			} else {
				if cols[0]["type"] != "varchar(max)" {
					t.Errorf("expected varchar(max), got %s", cols[0]["type"])
				}
				if cols[2]["name"] != "IsActive" || cols[2]["type"] != "nvarchar(3)" {
					t.Errorf("expected IsActive nvarchar(3), got %v", cols[2])
				}
			}
		}
		if step.Name == "Proc: get_crossdatabase_dependencies" {
			foundProc = true
		}
	}

	if !foundDatabases {
		t.Errorf("expected to find CREATE TABLE #databases")
	}
	if !foundDependencies {
		t.Errorf("expected to find CREATE TABLE #dependencies")
	}
	if !foundProc {
		t.Errorf("expected to find Proc: get_crossdatabase_dependencies")
	}
}
