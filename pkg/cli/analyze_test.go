package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeCommand(t *testing.T) {
	tempDir := t.TempDir()

	// Create sample multi-language files in tempDir
	goFile := filepath.Join(tempDir, "main.go")
	goCode := `package main
import "net/http"
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/orders", OrderHandler)
}
func OrderHandler(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(goFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write go file: %v", err)
	}

	sqlFile := filepath.Join(tempDir, "schema.sql")
	sqlCode := `CREATE TABLE orders (id INT, item_name VARCHAR(100));`
	if err := os.WriteFile(sqlFile, []byte(sqlCode), 0644); err != nil {
		t.Fatalf("failed to write sql file: %v", err)
	}

	outBase := filepath.Join(tempDir, "output")

	opts := &AnalyzeOptions{
		SourceDir:    tempDir,
		Recursive:    true,
		Formats:      "mermaid,excel,json,svg",
		OutputBase:   outBase,
		AIEnrichment: true,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze failed: %v", err)
	}

	// Verify generated outputs exist and are non-empty
	for _, ext := range []string{".mmd", ".xlsx", ".json", ".svg"} {
		fPath := outBase + ext
		fi, err := os.Stat(fPath)
		if err != nil {
			t.Errorf("expected output file %s to exist: %v", fPath, err)
			continue
		}
		if fi.Size() == 0 {
			t.Errorf("output file %s is empty", fPath)
		}
	}
}

func TestAnalyzeCommandWithExclude(t *testing.T) {
	tempDir := t.TempDir()

	// Main code
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc Run(){}"), 0644)

	// Excluded dir
	testDir := filepath.Join(tempDir, "tests")
	_ = os.MkdirAll(testDir, 0755)
	_ = os.WriteFile(filepath.Join(testDir, "test.go"), []byte("package tests\nfunc TestA(){}"), 0644)

	outBase := filepath.Join(tempDir, "output")

	opts := &AnalyzeOptions{
		SourceDir:   tempDir,
		Recursive:   true,
		Formats:     "json",
		OutputBase:  outBase,
		ExcludeDirs: []string{"tests"},
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze with exclude failed: %v", err)
	}

	content, err := os.ReadFile(outBase + ".json")
	if err != nil {
		t.Fatalf("failed to read output json: %v", err)
	}

	if string(content) == "" {
		t.Fatalf("expected non-empty json")
	}
}

func TestAnalyzeCommandDiagramTypes(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "service.go"), []byte("package main\nfunc HandleOrder(){}"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "schema.sql"), []byte("CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(50));"), 0644)

	outBase := filepath.Join(tempDir, "output")

	// Test diagram-type all with journey, er, and markdown
	opts := &AnalyzeOptions{
		SourceDir:   tempDir,
		Recursive:   true,
		Formats:     "mermaid,markdown,journey,er",
		DiagramType: "all",
		OutputBase:  outBase,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze with diagram-type all failed: %v", err)
	}

	expectedFiles := []string{
		outBase + ".mmd",
		outBase + "_lr.mmd",
		outBase + "_journey.mmd",
		outBase + "_er.mmd",
		outBase + ".md",
	}

	for _, ef := range expectedFiles {
		fi, err := os.Stat(ef)
		if err != nil {
			t.Errorf("expected diagram file %s to exist: %v", ef, err)
			continue
		}
		if fi.Size() == 0 {
			t.Errorf("file %s is empty", ef)
		}
	}
}

func TestAnalyzeCommandIgnorePattern(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc Run(){}"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "main_test.go"), []byte("package main\nfunc TestRun(){}"), 0644)

	outBase := filepath.Join(tempDir, "output")

	opts := &AnalyzeOptions{
		SourceDir:      tempDir,
		Recursive:      true,
		Formats:        "json",
		OutputBase:     outBase,
		IgnorePatterns: []string{"*_test.go"},
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze with ignore pattern failed: %v", err)
	}

	content, err := os.ReadFile(outBase + ".json")
	if err != nil {
		t.Fatalf("failed to read output json: %v", err)
	}

	if strings.Contains(string(content), "TestRun") {
		t.Errorf("expected TestRun to be ignored, but found in json: %s", string(content))
	}
	if !strings.Contains(string(content), "Run") {
		t.Errorf("expected Run to be included in json")
	}
}

func TestAnalyzeCommandIgnoreTests(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "server.go"), []byte("package main\nfunc Start(){}"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "server_test.go"), []byte("package main\nfunc TestStart(){}"), 0644)

	outBase := filepath.Join(tempDir, "output")

	opts := &AnalyzeOptions{
		SourceDir:   tempDir,
		Recursive:   true,
		Formats:     "json",
		OutputBase:  outBase,
		IgnoreTests: true,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze with ignore-tests failed: %v", err)
	}

	content, err := os.ReadFile(outBase + ".json")
	if err != nil {
		t.Fatalf("failed to read output json: %v", err)
	}

	if strings.Contains(string(content), "TestStart") {
		t.Errorf("expected TestStart to be ignored with --ignore-tests")
	}
	if !strings.Contains(string(content), "Start") {
		t.Errorf("expected Start to be included in json")
	}
}

func TestAnalyzeCommandSingleFile(t *testing.T) {
	tempDir := t.TempDir()

	singleFilePath := filepath.Join(tempDir, "handler.go")
	_ = os.WriteFile(singleFilePath, []byte("package main\nfunc ProcessItem(){}"), 0644)

	outBase := filepath.Join(tempDir, "output")

	opts := &AnalyzeOptions{
		SourceDir:  singleFilePath,
		Recursive:  false,
		Formats:    "mermaid,json",
		OutputBase: outBase,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze on single file failed: %v", err)
	}

	content, err := os.ReadFile(outBase + ".mmd")
	if err != nil {
		t.Fatalf("failed to read output mmd: %v", err)
	}

	if !strings.Contains(string(content), "ProcessItem") {
		t.Errorf("expected ProcessItem in single file output, got:\n%s", string(content))
	}
}

func TestAnalyzeCommandSVGDiagramTypes(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "api.go"), []byte("package main\nfunc GetUser(){}"), 0644)

	outBase := filepath.Join(tempDir, "output")

	opts := &AnalyzeOptions{
		SourceDir:   tempDir,
		Recursive:   false,
		Formats:     "svg",
		DiagramType: "all",
		OutputBase:  outBase,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze with svg and diagram-type all failed: %v", err)
	}

	expectedFiles := []string{
		outBase + ".svg",
		outBase + "_lr.svg",
		outBase + "_journey.svg",
		outBase + "_er.svg",
		outBase + "_sequence.svg",
		outBase + "_state.svg",
	}

	for _, f := range expectedFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("expected generated SVG file %s: %v", f, err)
		} else if len(data) == 0 {
			t.Errorf("file %s is empty", f)
		}
	}
}

func TestAnalyzeCommandOutputDirAndName(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src")
	_ = os.MkdirAll(sourceDir, 0755)
	_ = os.WriteFile(filepath.Join(sourceDir, "worker.go"), []byte("package main\nfunc Work(){}"), 0644)

	customOutDir := filepath.Join(tempDir, "my-diagrams", "subfolder")

	opts := &AnalyzeOptions{
		SourceDir:   sourceDir,
		OutputDir:   customOutDir,
		OutputName:  "etl_pipeline",
		Formats:     "mermaid,svg,json,excel",
		DiagramType: "all",
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze with output-dir and name failed: %v", err)
	}

	expectedPrefix := filepath.Join(customOutDir, "etl_pipeline")
	expectedFiles := []string{
		expectedPrefix + ".json",
		expectedPrefix + ".xlsx",
		expectedPrefix + ".mmd",
		expectedPrefix + "_lr.mmd",
		expectedPrefix + "_journey.mmd",
		expectedPrefix + "_sequence.mmd",
		expectedPrefix + "_state.mmd",
		expectedPrefix + "_er.mmd",
		expectedPrefix + ".svg",
		expectedPrefix + "_lr.svg",
		expectedPrefix + "_journey.svg",
		expectedPrefix + "_sequence.svg",
		expectedPrefix + "_state.svg",
		expectedPrefix + "_er.svg",
	}

	for _, f := range expectedFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("expected generated file %s: %v", f, err)
		} else if len(data) == 0 {
			t.Errorf("file %s is empty", f)
		}
	}

	// Verify no "output*" files were generated in that directory
	entries, err := os.ReadDir(customOutDir)
	if err != nil {
		t.Fatalf("failed to read custom output dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "output") {
			t.Errorf("found unwanted 'output' prefixed file in custom dir: %s", entry.Name())
		}
	}
}

func TestAnalyzeCommandNameWithExtension(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc Main(){}"), 0644)

	customOutDir := filepath.Join(tempDir, "out")

	opts := &AnalyzeOptions{
		SourceDir:   tempDir,
		OutputDir:   customOutDir,
		OutputName:  "myflow.mmd", // user passed extension accidentally
		Formats:     "mermaid,svg",
		DiagramType: "sequence",
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze failed: %v", err)
	}

	expectedFile := filepath.Join(customOutDir, "myflow.mmd")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected %s to exist: %v", expectedFile, err)
	}

	// Verify it didn't create "myflow.mmd.mmd" or "myflow.mmd_sequence.mmd"
	badFile := filepath.Join(customOutDir, "myflow.mmd.mmd")
	if _, err := os.Stat(badFile); err == nil {
		t.Errorf("found unwanted double extension file: %s", badFile)
	}
}

func TestAnalyzeCommandLiquibase(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a Liquibase XML changelog
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog">
    <changeSet id="1" author="admin">
        <comment>Create users table</comment>
        <createTable tableName="users">
            <column name="id" type="int" autoIncrement="true">
                <constraints primaryKey="true" nullable="false"/>
            </column>
            <column name="username" type="varchar(50)"/>
        </createTable>
    </changeSet>
</databaseChangeLog>`
	if err := os.WriteFile(filepath.Join(tempDir, "changelog.xml"), []byte(xmlContent), 0644); err != nil {
		t.Fatalf("failed to write changelog.xml: %v", err)
	}

	// 2. Create a Liquibase Formatted SQL changelog
	sqlContent := `--liquibase formatted sql
--changeset admin:2
CREATE TABLE audit_logs (
    id INT PRIMARY KEY,
    user_id INT,
    action VARCHAR(100),
    CONSTRAINT fk_audit_users FOREIGN KEY (user_id) REFERENCES users(id)
);
`
	if err := os.WriteFile(filepath.Join(tempDir, "changelog.sql"), []byte(sqlContent), 0644); err != nil {
		t.Fatalf("failed to write changelog.sql: %v", err)
	}

	outDir := filepath.Join(tempDir, "docs")

	opts := &AnalyzeOptions{
		SourceDir:   tempDir,
		OutputDir:   outDir,
		OutputName:  "liquibase_test",
		Formats:     "mermaid,svg,excel,json",
		DiagramType: "all",
		NoTruncate:  true,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze failed on Liquibase files: %v", err)
	}

	// Verify all diagram types generated
	expectedFiles := []string{
		"liquibase_test.mmd",
		"liquibase_test.svg",
		"liquibase_test_lr.mmd",
		"liquibase_test_lr.svg",
		"liquibase_test_er.mmd",
		"liquibase_test_er.svg",
		"liquibase_test_journey.mmd",
		"liquibase_test_journey.svg",
		"liquibase_test_sequence.mmd",
		"liquibase_test_sequence.svg",
		"liquibase_test_state.mmd",
		"liquibase_test_state.svg",
		"liquibase_test.json",
		"liquibase_test.xlsx",
	}

	for _, ef := range expectedFiles {
		p := filepath.Join(outDir, ef)
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected generated file %s to exist: %v", ef, err)
			continue
		}
		if fi.Size() == 0 {
			t.Errorf("generated file %s is empty", ef)
		}
	}
}

func TestAnalyzeCommandLiquibasePrimaryOrder(t *testing.T) {
	outDir := t.TempDir()

	opts := &AnalyzeOptions{
		SourceDir:        "../../examples/liquibase",
		PrimaryChangelog: "../../examples/liquibase/changesets.xml",
		Recursive:        true,
		Formats:          "mermaid,json",
		OutputName:       "primary_order_test",
		OutputDir:        outDir,
		DiagramType:      "all",
		NoTruncate:       true,
	}

	if err := RunAnalyze(opts); err != nil {
		t.Fatalf("RunAnalyze failed with primary changelog: %v", err)
	}

	mmdPath := filepath.Join(outDir, "primary_order_test.mmd")
	cnt, err := os.ReadFile(mmdPath)
	if err != nil {
		t.Fatalf("failed to read primary_order_test.mmd: %v", err)
	}
	content := string(cnt)

	// Verify changelog swimlanes exist
	if !strings.Contains(content, "subgraph lane_liquibase_core_changeset") {
		t.Errorf("expected subgraph lane_liquibase_core_changeset in output")
	}
	if !strings.Contains(content, "subgraph lane_liquibase_sql_changeset") {
		t.Errorf("expected subgraph lane_liquibase_sql_changeset in output")
	}

	// Verify sequential execution link
	if !strings.Contains(content, "executes next") {
		t.Errorf("expected 'executes next' link in output diagram")
	}
}





