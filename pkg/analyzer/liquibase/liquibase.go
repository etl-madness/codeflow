package liquibase

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"codeflow/pkg/analyzer"
)

// Analyzer implements static analysis for Liquibase database changelogs.
type Analyzer struct{}

// New creates a new Liquibase analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "liquibase"
}

// CanAnalyze returns true for recognized changelog file extensions.
func (a *Analyzer) CanAnalyze(ext string) bool {
	l := strings.ToLower(ext)
	return l == ".xml" || l == ".sql" || l == ".yaml" || l == ".yml" || l == ".json"
}

// AnalyzeFile parses a Liquibase changelog and maps it to canonical ProcessModel elements.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	ext := strings.ToLower(filepath.Ext(path))

	var cl *ChangeLog
	var err error

	switch {
	case ext == ".xml" || bytes.Contains(content, []byte("<databaseChangeLog")):
		cl, err = ParseXMLChangeLog(path, content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Liquibase XML changelog %s: %w", path, err)
		}

	case ext == ".sql" || bytes.Contains(content, []byte("--liquibase formatted sql")) || bytes.Contains(content, []byte("--changeset")):
		cl, err = ParseFormattedSQLChangeLog(path, content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Liquibase Formatted SQL changelog %s: %w", path, err)
		}

	default:
		return nil, fmt.Errorf("unsupported Liquibase format for file: %s", path)
	}

	return MapChangeLogToProcessModel(cl), nil
}
