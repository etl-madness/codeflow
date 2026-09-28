package liquibase

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codeflow/pkg/analyzer"
)

// NormalizeFilePath returns a normalized, lowercase forward-slash path suitable for cross-platform deduplication.
func NormalizeFilePath(p string) string {
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	return strings.ToLower(filepath.ToSlash(p))
}

// ResolveIncludePath resolves a child changelog path relative to the including changelog file.
func ResolveIncludePath(currentFile, targetPath string) string {
	// First check relative to current changelog directory
	currDir := filepath.Dir(currentFile)
	cand1 := filepath.Join(currDir, targetPath)
	if _, err := os.Stat(cand1); err == nil {
		return cand1
	}

	// Next check targetPath directly
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath
	}

	// Fallback to relative candidate
	return cand1
}

// ResolveChangelogOrder traverses changelog includes starting from primaryPath and computes execution order.
func ResolveChangelogOrder(primaryPath string) (*OrderedExecution, error) {
	cleanPrimary := filepath.Clean(primaryPath)
	exec := &OrderedExecution{
		PrimaryFile:  cleanPrimary,
		ChangeSets:   make([]ChangeSet, 0),
		FileOrder:    make([]string, 0),
		HandledFiles: make(map[string]bool),
		SQLFiles:     make([]SQLFileRef, 0),
	}

	visited := make(map[string]bool)
	if err := resolveFileRecursive(cleanPrimary, exec, visited); err != nil {
		return nil, err
	}

	return exec, nil
}

func resolveFileRecursive(filePath string, exec *OrderedExecution, visited map[string]bool) error {
	norm := NormalizeFilePath(filePath)
	if visited[norm] {
		return nil // Avoid circular includes
	}
	visited[norm] = true
	exec.HandledFiles[norm] = true
	exec.HandledFiles[filepath.Clean(filePath)] = true

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read changelog file %s: %w", filePath, err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	switch {
	case ext == ".xml" || bytes.Contains(content, []byte("<databaseChangeLog")):
		cl, err := ParseXMLChangeLog(filePath, content)
		if err != nil {
			return fmt.Errorf("failed to parse XML changelog %s: %w", filePath, err)
		}

		addedThisFile := false
		ensureFileInOrder := func() {
			if !addedThisFile {
				exec.FileOrder = append(exec.FileOrder, filePath)
				addedThisFile = true
			}
		}

		for _, entry := range cl.Entries {
			switch entry.Type {
			case EntryChangeSet:
				ensureFileInOrder()
				if entry.ChangeSet != nil {
					exec.ChangeSets = append(exec.ChangeSets, *entry.ChangeSet)
					for _, ch := range entry.ChangeSet.Changes {
						if ch.Type == "sqlFile" && ch.RawSQL != "" {
							resolvedSQL := ResolveIncludePath(filePath, ch.RawSQL)
							exec.SQLFiles = append(exec.SQLFiles, SQLFileRef{
								ChangeSetID:  entry.ChangeSet.ID,
								SourceFile:   filePath,
								RawPath:      ch.RawSQL,
								ResolvedPath: resolvedSQL,
							})
						}
					}
				}

			case EntryInclude:
				if entry.Include != nil && entry.Include.File != "" {
					childPath := ResolveIncludePath(filePath, entry.Include.File)
					if _, statErr := os.Stat(childPath); statErr != nil {
						fmt.Printf("[-] Warning: Included changelog file not found: %s (referenced in %s)\n", entry.Include.File, filePath)
						continue
					}
					if err := resolveFileRecursive(childPath, exec, visited); err != nil {
						return err
					}
				}

			case EntryIncludeAll:
				if entry.Include != nil && entry.Include.Path != "" {
					childDir := ResolveIncludePath(filePath, entry.Include.Path)
					fi, statErr := os.Stat(childDir)
					if statErr != nil || !fi.IsDir() {
						fmt.Printf("[-] Warning: Included directory not found: %s (referenced in %s)\n", entry.Include.Path, filePath)
						continue
					}

					entries, readErr := os.ReadDir(childDir)
					if readErr != nil {
						return fmt.Errorf("failed to read changelog directory %s: %w", childDir, readErr)
					}

					var fileNames []string
					for _, d := range entries {
						if d.IsDir() {
							continue
						}
						e := strings.ToLower(filepath.Ext(d.Name()))
						if e == ".xml" || e == ".sql" || e == ".yaml" || e == ".yml" || e == ".json" {
							fileNames = append(fileNames, d.Name())
						}
					}
					// Liquibase executes includeAll files in alphabetical order
					sort.Strings(fileNames)

					for _, name := range fileNames {
						childFile := filepath.Join(childDir, name)
						if err := resolveFileRecursive(childFile, exec, visited); err != nil {
							return err
						}
					}
				}
			}
		}

	case ext == ".sql" || bytes.Contains(content, []byte("--liquibase formatted sql")) || bytes.Contains(content, []byte("--changeset")):
		cl, err := ParseFormattedSQLChangeLog(filePath, content)
		if err != nil {
			return fmt.Errorf("failed to parse Formatted SQL changelog %s: %w", filePath, err)
		}

		if len(cl.ChangeSets) > 0 {
			exec.FileOrder = append(exec.FileOrder, filePath)
			exec.ChangeSets = append(exec.ChangeSets, cl.ChangeSets...)
		}

	default:
		// Unsupported or non-liquibase file encountered in include, skip gracefully
	}

	return nil
}

// AnalyzeWithPrimaryOrder parses a primary order changelog and returns a unified FileAnalysisResult.
func (a *Analyzer) AnalyzeWithPrimaryOrder(primaryPath string) (*analyzer.FileAnalysisResult, error) {
	exec, err := ResolveChangelogOrder(primaryPath)
	if err != nil {
		return nil, err
	}
	return MapOrderedExecutionToProcessModel(exec), nil
}
