package liquibase

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

var (
	changesetHeaderRegex = regexp.MustCompile(`(?i)^--\s*changeset\s+([^:\s]+):([^\s]+)(.*)$`)
	commentLineRegex     = regexp.MustCompile(`(?i)^--\s*comment[:\s]+(.*)$`)
	rollbackLineRegex    = regexp.MustCompile(`(?i)^--\s*rollback[:\s]+(.*)$`)
	preconditionsRegex   = regexp.MustCompile(`(?i)^--\s*preconditions[:\s]+(.*)$`)

	createTableRegex = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([#@\w\."\[\]]+)`)
	dropTableRegex   = regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([#@\w\."\[\]]+)`)
	createViewRegex  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?VIEW\s+([#@\w\."\[\]]+)`)
	createProcRegex  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+ALTER\s+)?PROC(?:EDURE)?\s+([#@\w\."\[\]]+)`)
	createIndexRegex = regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+([#@\w\."\[\]]+)\s+ON\s+([#@\w\."\[\]]+)`)
	addFKRegex       = regexp.MustCompile(`(?i)CONSTRAINT\s+([#@\w\."\[\]]+)\s+FOREIGN\s+KEY\s*\(([^\)]+)\)\s*REFERENCES\s+([#@\w\."\[\]]+)\s*\(([^\)]+)\)`)
	alterTableFKRegex = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+([#@\w\."\[\]]+)\s+ADD\s+(?:CONSTRAINT\s+([#@\w\."\[\]]+)\s+)?FOREIGN\s+KEY\s*\(([^\)]+)\)\s*REFERENCES\s+([#@\w\."\[\]]+)\s*\(([^\)]+)\)`)
)

// ParseFormattedSQLChangeLog parses a Liquibase formatted SQL changelog file.
func ParseFormattedSQLChangeLog(filePath string, content []byte) (*ChangeLog, error) {
	cl := &ChangeLog{
		FilePath: filePath,
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	var currentCS *ChangeSet
	var currentSQLLines []string

	flushCurrent := func() {
		if currentCS != nil {
			rawSQL := strings.TrimSpace(strings.Join(currentSQLLines, "\n"))
			if rawSQL != "" {
				extractChangesFromSQL(rawSQL, currentCS)
			}
			cl.ChangeSets = append(cl.ChangeSets, *currentCS)
			currentCS = nil
			currentSQLLines = nil
		}
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for --changeset author:id
		if matches := changesetHeaderRegex.FindStringSubmatch(trimmed); len(matches) >= 3 {
			flushCurrent()

			author := matches[1]
			id := matches[2]
			attrStr := ""
			if len(matches) > 3 {
				attrStr = matches[3]
			}

			cs := ChangeSet{
				ID:          id,
				Author:      author,
				FilePath:    filePath,
				LineNumber:  lineNum,
				FailOnError: true,
			}

			parseChangesetAttributes(attrStr, &cs)
			currentCS = &cs
			continue
		}

		if currentCS == nil {
			continue
		}

		// Check for --comment
		if cMatches := commentLineRegex.FindStringSubmatch(trimmed); len(cMatches) >= 2 {
			cText := strings.TrimSpace(cMatches[1])
			if currentCS.Comment == "" {
				currentCS.Comment = cText
			} else {
				currentCS.Comment += " " + cText
			}
			continue
		}

		// Check for --rollback
		if rMatches := rollbackLineRegex.FindStringSubmatch(trimmed); len(rMatches) >= 2 {
			rText := strings.TrimSpace(rMatches[1])
			if rText != "" {
				currentCS.Rollbacks = append(currentCS.Rollbacks, rText)
			}
			continue
		}

		// Check for --preconditions
		if pMatches := preconditionsRegex.FindStringSubmatch(trimmed); len(pMatches) >= 2 {
			currentCS.PreCondition = strings.TrimSpace(pMatches[1])
			continue
		}

		// Regular SQL line inside changeset
		currentSQLLines = append(currentSQLLines, line)
	}

	flushCurrent()
	return cl, nil
}

func parseChangesetAttributes(attrStr string, cs *ChangeSet) {
	parts := strings.Fields(attrStr)
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])

		switch key {
		case "context", "contexts":
			cs.Contexts = val
		case "labels", "label":
			cs.Labels = val
		case "runonchange":
			cs.RunOnChange = strings.EqualFold(val, "true") || val == "1"
		case "runalways":
			cs.RunAlways = strings.EqualFold(val, "true") || val == "1"
		case "failonerror":
			cs.FailOnError = !strings.EqualFold(val, "false") && val != "0"
		}
	}
}

func extractChangesFromSQL(sql string, cs *ChangeSet) {
	// 1. Check for ALTER TABLE ... ADD FOREIGN KEY
	if fkMatches := alterTableFKRegex.FindAllStringSubmatch(sql, -1); len(fkMatches) > 0 {
		for _, m := range fkMatches {
			baseTable := cleanSQLIdent(m[1])
			cName := cleanSQLIdent(m[2])
			baseCols := strings.TrimSpace(m[3])
			refTable := cleanSQLIdent(m[4])
			refCols := strings.TrimSpace(m[5])

			cs.Changes = append(cs.Changes, Change{
				Type:      "addForeignKeyConstraint",
				TableName: baseTable,
				FKConstraint: &FKConstraintDef{
					ConstraintName:    cName,
					BaseTableName:     baseTable,
					BaseColumnNames:   baseCols,
					ReferencedTable:   refTable,
					ReferencedColumns: refCols,
				},
			})
		}
	}

	// 2. Check for CREATE TABLE
	if tblMatches := createTableRegex.FindAllStringSubmatch(sql, -1); len(tblMatches) > 0 {
		for _, m := range tblMatches {
			tblName := cleanSQLIdent(m[1])
			cols, fks := extractColumnsFromCreateTable(sql, tblName)
			c := Change{
				Type:      "createTable",
				TableName: tblName,
				Columns:   cols,
			}
			cs.Changes = append(cs.Changes, c)

			for _, fk := range fks {
				cs.Changes = append(cs.Changes, Change{
					Type:         "addForeignKeyConstraint",
					TableName:    tblName,
					FKConstraint: fk,
				})
			}
		}
	}

	// 3. Check for CREATE VIEW
	if vMatches := createViewRegex.FindAllStringSubmatch(sql, -1); len(vMatches) > 0 {
		for _, m := range vMatches {
			cs.Changes = append(cs.Changes, Change{
				Type:     "createView",
				ViewName: cleanSQLIdent(m[1]),
				RawSQL:   sql,
			})
		}
	}

	// 4. Check for CREATE PROCEDURE
	if pMatches := createProcRegex.FindAllStringSubmatch(sql, -1); len(pMatches) > 0 {
		for _, m := range pMatches {
			cs.Changes = append(cs.Changes, Change{
				Type:          "createProcedure",
				ProcedureName: cleanSQLIdent(m[1]),
				RawSQL:        sql,
			})
		}
	}

	// 5. Check for CREATE INDEX
	if iMatches := createIndexRegex.FindAllStringSubmatch(sql, -1); len(iMatches) > 0 {
		for _, m := range iMatches {
			cs.Changes = append(cs.Changes, Change{
				Type:      "createIndex",
				IndexName: cleanSQLIdent(m[1]),
				TableName: cleanSQLIdent(m[2]),
			})
		}
	}

	// 6. Check for DROP TABLE
	if dtMatches := dropTableRegex.FindAllStringSubmatch(sql, -1); len(dtMatches) > 0 {
		for _, m := range dtMatches {
			cs.Changes = append(cs.Changes, Change{
				Type:      "dropTable",
				TableName: cleanSQLIdent(m[1]),
			})
		}
	}

	// If no structured changes matched, capture raw SQL
	if len(cs.Changes) == 0 {
		cs.Changes = append(cs.Changes, Change{
			Type:   "sql",
			RawSQL: sql,
		})
	}
}

func extractColumnsFromCreateTable(sql, targetTable string) ([]ColumnDef, []*FKConstraintDef) {
	var cols []ColumnDef
	var fks []*FKConstraintDef

	// Find the column block inside (...)
	startIdx := strings.Index(sql, "(")
	endIdx := strings.LastIndex(sql, ")")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return cols, fks
	}

	block := sql[startIdx+1 : endIdx]
	lines := splitSQLColumns(block)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check for table-level FOREIGN KEY
		if fkMatch := addFKRegex.FindStringSubmatch(trimmed); len(fkMatch) >= 5 {
			fks = append(fks, &FKConstraintDef{
				ConstraintName:    cleanSQLIdent(fkMatch[1]),
				BaseTableName:     targetTable,
				BaseColumnNames:   strings.TrimSpace(fkMatch[2]),
				ReferencedTable:   cleanSQLIdent(fkMatch[3]),
				ReferencedColumns: strings.TrimSpace(fkMatch[4]),
			})
			continue
		}

		// Check for table-level PRIMARY KEY (col1, col2)
		if strings.HasPrefix(strings.ToUpper(trimmed), "PRIMARY KEY") || strings.HasPrefix(strings.ToUpper(trimmed), "CONSTRAINT") && strings.Contains(strings.ToUpper(trimmed), "PRIMARY KEY") {
			continue
		}

		parts := strings.Fields(trimmed)
		if len(parts) >= 2 {
			colName := cleanSQLIdent(parts[0])
			colType := parts[1]
			if strings.Contains(colType, "(") && !strings.Contains(colType, ")") && len(parts) >= 3 {
				colType += parts[2]
			}

			upperLine := strings.ToUpper(trimmed)
			isPK := strings.Contains(upperLine, "PRIMARY KEY")
			isNotNull := strings.Contains(upperLine, "NOT NULL")
			isUnique := strings.Contains(upperLine, "UNIQUE")

			ref := ""
			if refIdx := strings.Index(upperLine, "REFERENCES"); refIdx != -1 {
				refPart := strings.TrimSpace(trimmed[refIdx+len("REFERENCES"):])
				ref = refPart
			}

			cols = append(cols, ColumnDef{
				Name:       colName,
				Type:       colType,
				PrimaryKey: isPK,
				Nullable:   !isNotNull,
				Unique:     isUnique,
				References: ref,
			})
		}
	}

	return cols, fks
}

func cleanSQLIdent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"'[]")
	if parts := strings.Split(s, "."); len(parts) > 1 {
		return strings.Trim(parts[len(parts)-1], "`\"'[]")
	}
	return s
}

func splitSQLColumns(block string) []string {
	var lines []string
	var current strings.Builder
	depth := 0

	for _, r := range block {
		switch r {
		case '(':
			depth++
			current.WriteRune(r)
		case ')':
			if depth > 0 {
				depth--
			}
			current.WriteRune(r)
		case ',':
			if depth == 0 {
				lines = append(lines, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

