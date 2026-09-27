package sql

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xwb1989/sqlparser"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/analyzer/liquibase"
	"codeflow/pkg/model"
)

// Analyzer implements static analysis for SQL scripts and queries.
type Analyzer struct{}

// New creates a new SQL Analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "sql"
}

// CanAnalyze returns true for .sql files.
func (a *Analyzer) CanAnalyze(ext string) bool {
	return strings.ToLower(ext) == ".sql"
}

var (
	createProcRegex  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?PROC(?:EDURE)?\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	createViewRegex  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?VIEW\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	createTableRegex = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	execProcRegex    = regexp.MustCompile(`(?i)(?:EXEC|EXECUTE)\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	selectFromRegex  = regexp.MustCompile(`(?i)FROM\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	insertIntoRegex  = regexp.MustCompile(`(?i)INSERT\s+INTO\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	updateRegex      = regexp.MustCompile(`(?i)UPDATE\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
	deleteRegex      = regexp.MustCompile(`(?i)DELETE\s+FROM\s+(?:\[?([#@\w]+)\]?\.)*\[?([#@\w]+)\]?`)
)

// AnalyzeFile parses SQL scripts and extracts table operations, views, stored procedures, and queries.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	// Check for Liquibase Formatted SQL
	if bytes.Contains(content, []byte("--liquibase formatted sql")) || bytes.Contains(content, []byte("--changeset ")) {
		return liquibase.New().AnalyzeFile(path, content)
	}

	fileName := filepath.Base(path)
	defaultSchema := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	swimlaneID := fmt.Sprintf("sql:%s", defaultSchema)

	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          swimlaneID,
		Name:        fmt.Sprintf("SQL Database (%s)", defaultSchema),
		Description: fmt.Sprintf("SQL script in %s", fileName),
	})

	statements := splitStatements(string(content))
	lineOffset := 1

	for idx, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		stepID := fmt.Sprintf("%s:stmt:%d", path, idx+1)
		analyzed := false

		// Try parsing with sqlparser (using ParseStrictDDL to avoid noisy logging on non-MySQL DDL)
		parsed, err := sqlparser.ParseStrictDDL(stmt)
		if err == nil {
			analyzed = a.processParsedStatement(parsed, stmt, path, lineOffset, stepID, swimlaneID, result)
		}

		// Fallback to dialect regex if sqlparser could not parse
		if !analyzed {
			a.processFallbackStatement(stmt, path, lineOffset, stepID, swimlaneID, result)
		}

		lineOffset += strings.Count(stmt, "\n") + 1
	}

	return result, nil
}

func (a *Analyzer) processParsedStatement(
	stmt sqlparser.Statement,
	rawSQL string,
	path string,
	line int,
	stepID string,
	swimlaneID string,
	result *analyzer.FileAnalysisResult,
) bool {
	switch s := stmt.(type) {
	case *sqlparser.Select:
		tables := extractTablesFromSelect(s)
		name := "SELECT " + strings.Join(tables, ", ")
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        name,
			Description: fmt.Sprintf("Query on tables: %s", strings.Join(tables, ", ")),
			Type:        "DatabaseQuery",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"operation": "SELECT",
				"tables":    tables,
			},
		})
		return true

	case *sqlparser.Insert:
		tableName := s.Table.Name.String()
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("INSERT INTO %s", tableName),
			Description: fmt.Sprintf("Insert records into table %s", tableName),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"operation": "INSERT",
				"table":     tableName,
			},
		})
		return true

	case *sqlparser.Update:
		tableName := ""
		if len(s.TableExprs) > 0 {
			tableName = sqlparser.String(s.TableExprs[0])
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("UPDATE %s", tableName),
			Description: fmt.Sprintf("Update records in table %s", tableName),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"operation": "UPDATE",
				"table":     tableName,
			},
		})
		return true

	case *sqlparser.Delete:
		tableName := ""
		if len(s.TableExprs) > 0 {
			tableName = sqlparser.String(s.TableExprs[0])
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("DELETE FROM %s", tableName),
			Description: fmt.Sprintf("Delete records from table %s", tableName),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"operation": "DELETE",
				"table":     tableName,
			},
		})
		return true

	case *sqlparser.DDL:
		if matches := createViewRegex.FindStringSubmatch(rawSQL); len(matches) > 2 {
			viewName := matches[2]
			if viewName == "" {
				viewName = matches[1]
			}
			result.Steps = append(result.Steps, model.Step{
				ID:          stepID,
				SwimlaneID:  swimlaneID,
				Name:        fmt.Sprintf("View: %s", viewName),
				Description: fmt.Sprintf("SQL View %s", viewName),
				Type:        "View",
				Language:    "sql",
				SourceFile:  path,
				LineNumber:  line,
				Metadata: map[string]any{
					"view": viewName,
				},
			})
			return true
		}

		op := s.Action
		tableName := s.Table.Name.String()
		if tableName == "" {
			if matches := createTableRegex.FindStringSubmatch(rawSQL); len(matches) > 2 {
				tableName = matches[2]
				if tableName == "" {
					tableName = matches[1]
				}
			}
		}
		cols := extractTableColumns(rawSQL)
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("%s %s", strings.ToUpper(op), tableName),
			Description: fmt.Sprintf("DDL operation %s on %s", op, tableName),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"operation": op,
				"table":     tableName,
				"columns":   cols,
			},
		})
		return true
	}

	return false
}

func (a *Analyzer) processFallbackStatement(
	stmt string,
	path string,
	line int,
	stepID string,
	swimlaneID string,
	result *analyzer.FileAnalysisResult,
) {
	// Stored procedure declaration
	if matches := createProcRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		procName := matches[2]
		if procName == "" {
			procName = matches[1]
		}
		schema := matches[1]

		procStep := model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("Proc: %s", procName),
			Description: fmt.Sprintf("Stored Procedure %s", procName),
			Type:        "StoredProcedure",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"procedure": procName,
				"schema":    schema,
			},
		}
		result.Steps = append(result.Steps, procStep)

		// Check if stored procedure calls other tables or procs inside its body
		if fromMatch := selectFromRegex.FindStringSubmatch(stmt); len(fromMatch) > 2 {
			tbl := fromMatch[2]
			if tbl == "" {
				tbl = fromMatch[1]
			}
			queryStepID := fmt.Sprintf("%s:inner_select:%s", stepID, tbl)
			result.Steps = append(result.Steps, model.Step{
				ID:          queryStepID,
				SwimlaneID:  swimlaneID,
				Name:        fmt.Sprintf("Query %s", tbl),
				Description: fmt.Sprintf("Reads table %s within stored procedure %s", tbl, procName),
				Type:        "DatabaseQuery",
				Language:    "sql",
				SourceFile:  path,
				LineNumber:  line,
			})
			result.Links = append(result.Links, model.Link{
				ID:           fmt.Sprintf("%s->%s", stepID, queryStepID),
				SourceStepID: stepID,
				TargetStepID: queryStepID,
				Label:        "queries",
			})
		}
		return
	}

	// View creation
	if matches := createViewRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		viewName := matches[2]
		if viewName == "" {
			viewName = matches[1]
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("View: %s", viewName),
			Description: fmt.Sprintf("SQL View %s", viewName),
			Type:        "View",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"view": viewName,
			},
		})
		return
	}

	// Create table
	if matches := createTableRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		tableName := matches[2]
		if tableName == "" {
			tableName = matches[1]
		}
		cols := extractTableColumns(stmt)
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("CREATE TABLE %s", tableName),
			Description: fmt.Sprintf("Table schema definition %s", tableName),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"table":     tableName,
				"operation": "CREATE",
				"columns":   cols,
			},
		})
		return
	}

	// Stored procedure call (EXEC)
	if matches := execProcRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		proc := matches[2]
		if proc == "" {
			proc = matches[1]
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("EXEC %s", proc),
			Description: fmt.Sprintf("Executes stored procedure %s", proc),
			Type:        "StoredProcedure",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"procedure": proc,
			},
		})
		return
	}

	// INSERT fallback
	if matches := insertIntoRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		tbl := matches[2]
		if tbl == "" {
			tbl = matches[1]
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("INSERT INTO %s", tbl),
			Description: fmt.Sprintf("Insert records into table %s", tbl),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
		})
		return
	}

	// UPDATE fallback
	if matches := updateRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		tbl := matches[2]
		if tbl == "" {
			tbl = matches[1]
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("UPDATE %s", tbl),
			Description: fmt.Sprintf("Update records in table %s", tbl),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
		})
		return
	}

	// DELETE fallback
	if matches := deleteRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		tbl := matches[2]
		if tbl == "" {
			tbl = matches[1]
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("DELETE FROM %s", tbl),
			Description: fmt.Sprintf("Delete records from table %s", tbl),
			Type:        "TableOperation",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
		})
		return
	}

	// SELECT fallback
	if matches := selectFromRegex.FindStringSubmatch(stmt); len(matches) > 2 {
		tbl := matches[2]
		if tbl == "" {
			tbl = matches[1]
		}
		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        fmt.Sprintf("SELECT FROM %s", tbl),
			Description: fmt.Sprintf("Query table %s", tbl),
			Type:        "DatabaseQuery",
			Language:    "sql",
			SourceFile:  path,
			LineNumber:  line,
		})
		return
	}
}

func extractTablesFromSelect(sel *sqlparser.Select) []string {
	var tables []string
	for _, expr := range sel.From {
		if aliased, ok := expr.(*sqlparser.AliasedTableExpr); ok {
			if tableName, okTable := aliased.Expr.(sqlparser.TableName); okTable {
				tables = append(tables, tableName.Name.String())
			}
		}
	}
	return tables
}

func splitStatements(content string) []string {
	// First split by GO statements if present
	goRegex := regexp.MustCompile(`(?m)^\s*GO\s*$`)
	parts := goRegex.Split(content, -1)

	var statements []string
	for _, part := range parts {
		// Split by semicolon
		rawStmts := strings.Split(part, ";")
		for _, s := range rawStmts {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				statements = append(statements, trimmed)
			}
		}
	}
	return statements
}

func extractTableColumns(stmt string) []map[string]string {
	var columns []map[string]string
	openIdx := strings.Index(stmt, "(")
	closeIdx := strings.LastIndex(stmt, ")")
	if openIdx == -1 || closeIdx == -1 || closeIdx <= openIdx {
		return columns
	}

	body := stmt[openIdx+1 : closeIdx]
	lines := strings.Split(body, "\n")
	colRegex := regexp.MustCompile(`^\s*\[?([#@a-zA-Z0-9_]+)\]?\s+\[?([a-zA-Z0-9_]+)\]?(?:\(([a-zA-Z0-9,\s]+)\))?`)
	ignoreKeywords := map[string]bool{
		"constraint": true, "primary": true, "foreign": true, "check": true, "index": true, "key": true, "unique": true, "references": true,
	}

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "--") {
			continue
		}
		matches := colRegex.FindStringSubmatch(l)
		if len(matches) > 2 {
			colName := matches[1]
			colType := matches[2]
			if len(matches) > 3 && matches[3] != "" {
				colType = fmt.Sprintf("%s(%s)", matches[2], matches[3])
			}
			if !ignoreKeywords[strings.ToLower(colName)] {
				isPK := strings.Contains(strings.ToUpper(l), "PRIMARY KEY")
				isFK := strings.Contains(strings.ToUpper(l), "FOREIGN KEY") || strings.Contains(strings.ToUpper(l), "REFERENCES")
				keyType := ""
				if isPK {
					keyType = "PK"
				} else if isFK {
					keyType = "FK"
				}
				columns = append(columns, map[string]string{
					"name": colName,
					"type": colType,
					"key":  keyType,
				})
			}
		}
	}
	return columns
}
