package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanner(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file structure
	files := []string{
		"main.go",
		"service/handler.cs",
		"components/view.razor",
		"scripts/worker.py",
		"db/schema.sql",
		"workflows/pipeline.xml",
		"workflows/package.dtsx",
		"ignored.txt",
		"vendor/dep.go",
		".git/config",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("// sample"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	// Also add a custom .gitignore
	gitignoreContent := "scripts/\n*.dtsx\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	scanner, err := NewScanner(tempDir, true)
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	foundMap := make(map[string]string)
	for _, f := range discovered {
		rel, _ := filepath.Rel(tempDir, f.Path)
		foundMap[filepath.ToSlash(rel)] = f.Language
	}

	expected := map[string]string{
		"main.go":                LangGo,
		"service/handler.cs":     LangCSharp,
		"components/view.razor":  LangCSharp,
		"db/schema.sql":          LangSQL,
		"workflows/pipeline.xml": LangFlowXML,
	}

	if len(foundMap) != len(expected) {
		t.Fatalf("expected %d files, got %d: %+v", len(expected), len(foundMap), foundMap)
	}

	for path, expectedLang := range expected {
		lang, ok := foundMap[path]
		if !ok {
			t.Errorf("expected to find %s", path)
		} else if lang != expectedLang {
			t.Errorf("for %s expected lang %s, got %s", path, expectedLang, lang)
		}
	}
}

func TestScannerExcludeDirs(t *testing.T) {
	tempDir := t.TempDir()

	files := []string{
		"root.go",
		"src/app.go",
		"tests/unit/test.go",
		"docs/samples/sample.py",
		"legacy/old.cs",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		_ = os.WriteFile(fullPath, []byte("// code"), 0644)
	}

	// Exclude "tests" and "docs"
	scanner, err := NewScanner(tempDir, true, "tests", "docs")
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, f := range discovered {
		rel, _ := filepath.Rel(tempDir, f.Path)
		slashPath := filepath.ToSlash(rel)
		if slashPath == "tests/unit/test.go" || slashPath == "docs/samples/sample.py" {
			t.Errorf("excluded file was discovered: %s", slashPath)
		}
	}

	// Make sure root.go, src/app.go, legacy/old.cs were discovered
	if len(discovered) != 3 {
		t.Errorf("expected 3 files discovered, got %d", len(discovered))
	}
}

func TestScannerExcludeRelativeAndAbsolutePaths(t *testing.T) {
	tempDir := t.TempDir()

	files := []string{
		"root.go",
		"examples/example1.go",
		"db_importer/importer.go",
		".private/secret.go",
		"docs/guide.go",
		"keep/service.go",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		_ = os.WriteFile(fullPath, []byte("package main"), 0644)
	}

	// Test excluding using absolute path, relative path from root, and comma-separated
	absExamples := filepath.Join(tempDir, "examples")
	absDbImporter := filepath.Join(tempDir, "db_importer")

	scanner, err := NewScanner(tempDir, true, absExamples, absDbImporter, ".private", "docs")
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Should only discover root.go and keep/service.go
	if len(discovered) != 2 {
		for _, d := range discovered {
			t.Logf("unexpected discovered: %s", d.Path)
		}
		t.Fatalf("expected 2 files discovered, got %d", len(discovered))
	}

	for _, d := range discovered {
		rel, _ := filepath.Rel(tempDir, d.Path)
		slash := filepath.ToSlash(rel)
		if slash != "root.go" && slash != "keep/service.go" {
			t.Errorf("unexpected file found: %s", slash)
		}
	}
}

func TestScannerFilePatterns(t *testing.T) {
	tempDir := t.TempDir()

	files := []string{
		"main.go",
		"main_test.go",
		"builder/auth.go",
		"builder/auth_test.go",
		"services/order.py",
		"services/test_order.py",
		"models/user.cs",
		"models/UserTests.cs",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		_ = os.WriteFile(fullPath, []byte("code"), 0644)
	}

	// Exclude Go unit tests (*_test.go) and Python tests (test_*.py)
	scanner, err := NewScanner(tempDir, true, "*_test.go", "test_*.py")
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, f := range discovered {
		rel, _ := filepath.Rel(tempDir, f.Path)
		slash := filepath.ToSlash(rel)
		if strings.HasSuffix(slash, "_test.go") || strings.Contains(slash, "test_") {
			t.Errorf("file matching pattern was not ignored: %s", slash)
		}
	}

	// Ensure main.go, builder/auth.go, services/order.py, models/user.cs, models/UserTests.cs are present
	if len(discovered) != 5 {
		t.Errorf("expected 5 files, got %d", len(discovered))
	}
}

func TestScannerCommonTestPatterns(t *testing.T) {
	tempDir := t.TempDir()

	files := []string{
		"main.go",
		"main_test.go",
		"service.cs",
		"ServiceTests.cs",
		"worker.py",
		"test_worker.py",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		_ = os.WriteFile(fullPath, []byte("code"), 0644)
	}

	scanner, err := NewScanner(tempDir, true, CommonTestPatterns...)
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, f := range discovered {
		rel, _ := filepath.Rel(tempDir, f.Path)
		slash := filepath.ToSlash(rel)
		if strings.Contains(strings.ToLower(slash), "test") {
			t.Errorf("test file was not ignored: %s", slash)
		}
	}
	if len(discovered) != 3 {
		t.Errorf("expected 3 non-test files, got %d", len(discovered))
	}
}

func TestScannerDTSXMappingAndSingleFile(t *testing.T) {
	lang, ok := MapExtensionToLanguage(".dtsx")
	if !ok || lang != LangSSIS {
		t.Fatalf("expected .dtsx to map to LangSSIS (%s), got %s (ok=%v)", LangSSIS, lang, ok)
	}

	tempDir := t.TempDir()
	dtsxPath := filepath.Join(tempDir, "package.dtsx")
	if err := os.WriteFile(dtsxPath, []byte("<xml/>"), 0644); err != nil {
		t.Fatalf("failed to write test dtsx: %v", err)
	}

	scanner, err := NewScanner(dtsxPath, false)
	if err != nil {
		t.Fatalf("failed to create scanner for single dtsx file: %v", err)
	}

	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered file, got %d", len(discovered))
	}

	if discovered[0].Language != LangSSIS {
		t.Errorf("expected language %s, got %s", LangSSIS, discovered[0].Language)
	}
}


