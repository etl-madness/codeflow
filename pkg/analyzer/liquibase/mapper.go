package liquibase

import (
	"fmt"
	"path/filepath"
	"strings"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

type tableState struct {
	Name        string
	Columns     []ColumnDef
	Remarks     string
	IsDropped   bool
	ForeignKeys []*FKConstraintDef
	SourceFile  string
	SwimlaneID  string
}

// MapChangeLogToProcessModel converts a parsed ChangeLog into a FileAnalysisResult.
func MapChangeLogToProcessModel(cl *ChangeLog) *analyzer.FileAnalysisResult {
	exec := &OrderedExecution{
		PrimaryFile: cl.FilePath,
		ChangeSets:  cl.ChangeSets,
		FileOrder:   []string{cl.FilePath},
	}
	return MapOrderedExecutionToProcessModel(exec)
}

// MapOrderedExecutionToProcessModel converts an OrderedExecution into a FileAnalysisResult.
func MapOrderedExecutionToProcessModel(exec *OrderedExecution) *analyzer.FileAnalysisResult {
	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	seenSwimlanes := make(map[string]bool)
	swimlaneByFile := make(map[string]string)

	for _, fPath := range exec.FileOrder {
		baseName := filepath.Base(fPath)
		cleanName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		swimlaneID := fmt.Sprintf("lane_liquibase_%s", sanitizeIdent(cleanName))

		if !seenSwimlanes[swimlaneID] {
			seenSwimlanes[swimlaneID] = true
			result.Swimlanes = append(result.Swimlanes, model.Swimlane{
				ID:          swimlaneID,
				Name:        fmt.Sprintf("Liquibase (%s)", cleanName),
				Description: fmt.Sprintf("Liquibase migration changelog %s", baseName),
			})
		}
		swimlaneByFile[fPath] = swimlaneID
	}

	tables := make(map[string]*tableState)
	var prevStepID string

	for _, cs := range exec.ChangeSets {
		swimlaneID := swimlaneByFile[cs.FilePath]
		if swimlaneID == "" {
			baseName := filepath.Base(cs.FilePath)
			cleanName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
			swimlaneID = fmt.Sprintf("lane_liquibase_%s", sanitizeIdent(cleanName))
			swimlaneByFile[cs.FilePath] = swimlaneID
			if !seenSwimlanes[swimlaneID] {
				seenSwimlanes[swimlaneID] = true
				result.Swimlanes = append(result.Swimlanes, model.Swimlane{
					ID:          swimlaneID,
					Name:        fmt.Sprintf("Liquibase (%s)", cleanName),
					Description: fmt.Sprintf("Liquibase migration changelog %s", baseName),
				})
			}
		}

		stepID := fmt.Sprintf("%s:%s:%d", cs.FilePath, cs.ID, cs.LineNumber)
		displayName := fmt.Sprintf("%s:%s", cs.Author, cs.ID)

		stepType := "TableOperation"
		primaryTable := ""
		var changeSummaries []string

		// Process changes & update cumulative schema
		for _, ch := range cs.Changes {
			if ch.TableName != "" && primaryTable == "" {
				primaryTable = ch.TableName
			}

			switch ch.Type {
			case "createTable":
				stepType = "TableOperation"
				tables[ch.TableName] = &tableState{
					Name:       ch.TableName,
					Columns:    ch.Columns,
					Remarks:    ch.Remarks,
					SourceFile: cs.FilePath,
					SwimlaneID: swimlaneID,
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create table %s with %d columns", ch.TableName, len(ch.Columns)))

			case "dropTable":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists {
					t.IsDropped = true
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Drop table %s", ch.TableName))

			case "addColumn":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists {
					t.Columns = append(t.Columns, ch.Columns...)
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Add %d columns to %s", len(ch.Columns), ch.TableName))

			case "dropColumn":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists && len(ch.Columns) > 0 {
					var remaining []ColumnDef
					dropName := ch.Columns[0].Name
					for _, c := range t.Columns {
						if !strings.EqualFold(c.Name, dropName) {
							remaining = append(remaining, c)
						}
					}
					t.Columns = remaining
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Drop column from %s", ch.TableName))

			case "modifyDataType":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists && len(ch.Columns) > 0 {
					modCol := ch.Columns[0]
					for i := range t.Columns {
						if strings.EqualFold(t.Columns[i].Name, modCol.Name) {
							t.Columns[i].Type = modCol.Type
							break
						}
					}
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Modify column data type on %s", ch.TableName))

			case "renameColumn":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists {
					for i := range t.Columns {
						if strings.EqualFold(t.Columns[i].Name, ch.OldName) {
							t.Columns[i].Name = ch.NewName
							break
						}
					}
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Rename column %s to %s on %s", ch.OldName, ch.NewName, ch.TableName))

			case "renameTable":
				stepType = "TableOperation"
				if t, exists := tables[ch.OldName]; exists {
					delete(tables, ch.OldName)
					t.Name = ch.NewName
					tables[ch.NewName] = t
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Rename table %s to %s", ch.OldName, ch.NewName))

			case "addPrimaryKey":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists && ch.PKConstraint != nil {
					pkCols := strings.Split(ch.PKConstraint.ColumnNames, ",")
					for _, pkCol := range pkCols {
						pkColTrim := strings.TrimSpace(pkCol)
						for i := range t.Columns {
							if strings.EqualFold(t.Columns[i].Name, pkColTrim) {
								t.Columns[i].PrimaryKey = true
							}
						}
					}
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Add primary key to %s", ch.TableName))

			case "dropPrimaryKey":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists {
					for i := range t.Columns {
						t.Columns[i].PrimaryKey = false
					}
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Drop primary key from %s", ch.TableName))

			case "addForeignKeyConstraint":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists && ch.FKConstraint != nil {
					t.ForeignKeys = append(t.ForeignKeys, ch.FKConstraint)
				}
				cName := ""
				if ch.FKConstraint != nil {
					cName = ch.FKConstraint.ConstraintName
				}
				changeSummaries = append(changeSummaries, fmt.Sprintf("Add foreign key %s on %s", cName, ch.TableName))

			case "createView":
				stepType = "DatabaseView"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create view %s", ch.ViewName))

			case "dropView":
				stepType = "DatabaseView"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Drop view %s", ch.ViewName))

			case "createProcedure":
				stepType = "StoredProcedure"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create procedure %s", ch.ProcedureName))

			case "dropProcedure":
				stepType = "StoredProcedure"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Drop procedure %s", ch.ProcedureName))

			case "createIndex":
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create index %s on %s", ch.IndexName, ch.TableName))

			case "dropIndex":
				changeSummaries = append(changeSummaries, fmt.Sprintf("Drop index %s", ch.IndexName))

			case "sql", "sqlFile":
				stepType = "DatabaseQuery"
				if ch.Type == "sqlFile" {
					base := filepath.Base(ch.RawSQL)
					if strings.Contains(strings.ToLower(base), "usp_") || strings.Contains(strings.ToLower(base), "proc") {
						stepType = "StoredProcedure"
					}
					changeSummaries = append(changeSummaries, fmt.Sprintf("Execute <sqlFile>: %s", base))
				} else {
					raw := ch.RawSQL
					if len(raw) > 60 {
						raw = raw[:57] + "..."
					}
					changeSummaries = append(changeSummaries, fmt.Sprintf("Execute SQL: %s", raw))
				}

			case "tagDatabase":
				stepType = "Milestone"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Tag database milestone: %s", ch.Tag))
			}
		}

		desc := cs.Comment
		if desc == "" {
			if len(changeSummaries) > 0 {
				desc = strings.Join(changeSummaries, "; ")
			} else {
				desc = fmt.Sprintf("Liquibase ChangeSet %s by %s", cs.ID, cs.Author)
			}
		}

		stepMeta := map[string]any{
			"author":        cs.Author,
			"changeset_id":  cs.ID,
			"contexts":      cs.Contexts,
			"labels":        cs.Labels,
			"run_on_change": cs.RunOnChange,
			"run_always":    cs.RunAlways,
			"is_liquibase":  true,
		}
		if primaryTable != "" {
			stepMeta["table"] = primaryTable
		}
		if len(cs.Rollbacks) > 0 {
			stepMeta["rollback"] = strings.Join(cs.Rollbacks, "; ")
		}

		var sqlFiles []string
		for _, ch := range cs.Changes {
			if ch.Type == "sqlFile" && ch.RawSQL != "" {
				sqlFiles = append(sqlFiles, ch.RawSQL)
			}
		}
		if len(sqlFiles) > 0 {
			stepMeta["sql_files"] = sqlFiles
			stepMeta["sql_file"] = sqlFiles[0]
			stepMeta["sql_file_base"] = filepath.Base(sqlFiles[0])
		}

		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        displayName,
			Description: desc,
			Type:        stepType,
			Language:    "liquibase",
			SourceFile:  cs.FilePath,
			LineNumber:  cs.LineNumber,
			Metadata:    stepMeta,
		})

		// Continuous sequential execution link
		if prevStepID != "" {
			result.Links = append(result.Links, model.Link{
				ID:           fmt.Sprintf("%s->%s:exec", prevStepID, stepID),
				SourceStepID: prevStepID,
				TargetStepID: stepID,
				Label:        "executes next",
			})
		}
		prevStepID = stepID
	}

	// Emit final cumulative tables for ER Diagram generation
	for _, tbl := range tables {
		if tbl.IsDropped {
			continue
		}

		srcFile := tbl.SourceFile
		if srcFile == "" {
			srcFile = exec.PrimaryFile
		}

		tblStepID := fmt.Sprintf("%s:table:%s", srcFile, tbl.Name)
		var colsList []map[string]string

		for _, col := range tbl.Columns {
			key := ""
			if col.PrimaryKey {
				key = "PK"
			}
			colsList = append(colsList, map[string]string{
				"name": col.Name,
				"type": col.Type,
				"key":  key,
			})
		}

		desc := fmt.Sprintf("Database Table %s", tbl.Name)
		if tbl.Remarks != "" {
			desc = tbl.Remarks
		}

		var fksList []map[string]string
		for _, fk := range tbl.ForeignKeys {
			fksList = append(fksList, map[string]string{
				"constraint_name":    fk.ConstraintName,
				"base_table":         tbl.Name,
				"base_columns":       fk.BaseColumnNames,
				"referenced_table":   fk.ReferencedTable,
				"referenced_columns": fk.ReferencedColumns,
			})
		}

		laneID := tbl.SwimlaneID
		if laneID == "" && len(result.Swimlanes) > 0 {
			laneID = result.Swimlanes[0].ID
		}

		result.Steps = append(result.Steps, model.Step{
			ID:          tblStepID,
			SwimlaneID:  laneID,
			Name:        tbl.Name,
			Description: desc,
			Type:        "DatabaseTable",
			Language:    "liquibase",
			SourceFile:  srcFile,
			Metadata: map[string]any{
				"table":        tbl.Name,
				"columns":      colsList,
				"foreign_keys": fksList,
				"is_liquibase": true,
			},
		})

		// Emit foreign key relationships between tables
		for _, fk := range tbl.ForeignKeys {
			if refTbl, exists := tables[fk.ReferencedTable]; exists {
				refFile := refTbl.SourceFile
				if refFile == "" {
					refFile = exec.PrimaryFile
				}
				refTableID := fmt.Sprintf("%s:table:%s", refFile, fk.ReferencedTable)
				label := "references"
				if fk.ConstraintName != "" {
					label = fmt.Sprintf("references (%s)", fk.ConstraintName)
				}
				result.Links = append(result.Links, model.Link{
					ID:           fmt.Sprintf("%s->%s:fk", tblStepID, refTableID),
					SourceStepID: tblStepID,
					TargetStepID: refTableID,
					Label:        label,
				})
			}
		}
	}

	return result
}

func sanitizeIdent(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}
