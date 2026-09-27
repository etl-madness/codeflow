package liquibase

import (
	"fmt"
	"path/filepath"
	"strings"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

type tableState struct {
	Name       string
	Columns    []ColumnDef
	Remarks    string
	IsDropped  bool
	ForeignKeys []*FKConstraintDef
}

// MapChangeLogToProcessModel converts a parsed ChangeLog into a FileAnalysisResult.
func MapChangeLogToProcessModel(cl *ChangeLog) *analyzer.FileAnalysisResult {
	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	baseName := filepath.Base(cl.FilePath)
	cleanName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	swimlaneID := fmt.Sprintf("lane_liquibase_%s", sanitizeIdent(cleanName))

	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          swimlaneID,
		Name:        fmt.Sprintf("Liquibase (%s)", cleanName),
		Description: fmt.Sprintf("Liquibase migration changelog %s", baseName),
	})

	tables := make(map[string]*tableState)
	var prevStepID string

	for _, cs := range cl.ChangeSets {
		stepID := fmt.Sprintf("%s:%s:%d", cl.FilePath, cs.ID, cs.LineNumber)
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
					Name:    ch.TableName,
					Columns: ch.Columns,
					Remarks: ch.Remarks,
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
					dropName := ch.Columns[0].Name
					var remaining []ColumnDef
					for _, col := range t.Columns {
						if !strings.EqualFold(col.Name, dropName) {
							remaining = append(remaining, col)
						}
					}
					t.Columns = remaining
					changeSummaries = append(changeSummaries, fmt.Sprintf("Drop column %s from %s", dropName, ch.TableName))
				}

			case "modifyDataType":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists && len(ch.Columns) > 0 {
					colName := ch.Columns[0].Name
					newType := ch.Columns[0].Type
					for i := range t.Columns {
						if strings.EqualFold(t.Columns[i].Name, colName) {
							t.Columns[i].Type = newType
							break
						}
					}
					changeSummaries = append(changeSummaries, fmt.Sprintf("Modify column %s type to %s on %s", colName, newType, ch.TableName))
				}

			case "renameColumn":
				stepType = "TableOperation"
				if t, exists := tables[ch.TableName]; exists {
					for i := range t.Columns {
						if strings.EqualFold(t.Columns[i].Name, ch.OldName) {
							t.Columns[i].Name = ch.NewName
							break
						}
					}
					changeSummaries = append(changeSummaries, fmt.Sprintf("Rename column %s to %s on %s", ch.OldName, ch.NewName, ch.TableName))
				}

			case "renameTable":
				stepType = "TableOperation"
				if t, exists := tables[ch.OldName]; exists {
					t.Name = ch.NewName
					tables[ch.NewName] = t
					delete(tables, ch.OldName)
					changeSummaries = append(changeSummaries, fmt.Sprintf("Rename table %s to %s", ch.OldName, ch.NewName))
				}

			case "addPrimaryKey":
				stepType = "TableOperation"
				if ch.PKConstraint != nil && ch.PKConstraint.TableName != "" {
					if t, exists := tables[ch.PKConstraint.TableName]; exists {
						cols := strings.Split(ch.PKConstraint.ColumnNames, ",")
						for _, colName := range cols {
							cleanCol := strings.TrimSpace(colName)
							for i := range t.Columns {
								if strings.EqualFold(t.Columns[i].Name, cleanCol) {
									t.Columns[i].PrimaryKey = true
								}
							}
						}
					}
					changeSummaries = append(changeSummaries, fmt.Sprintf("Add primary key on %s (%s)", ch.PKConstraint.TableName, ch.PKConstraint.ColumnNames))
				}

			case "addForeignKeyConstraint":
				stepType = "TableOperation"
				if ch.FKConstraint != nil {
					baseTbl := ch.FKConstraint.BaseTableName
					if t, exists := tables[baseTbl]; exists {
						t.ForeignKeys = append(t.ForeignKeys, ch.FKConstraint)
					}
					changeSummaries = append(changeSummaries, fmt.Sprintf("Add foreign key %s (%s -> %s)", ch.FKConstraint.ConstraintName, baseTbl, ch.FKConstraint.ReferencedTable))
				}

			case "createView":
				stepType = "View"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create view %s", ch.ViewName))

			case "createProcedure":
				stepType = "StoredProcedure"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create procedure %s", ch.ProcedureName))

			case "createIndex":
				stepType = "TableOperation"
				changeSummaries = append(changeSummaries, fmt.Sprintf("Create index %s on %s", ch.IndexName, ch.TableName))

			case "sql", "sqlFile":
				stepType = "DatabaseQuery"
				changeSummaries = append(changeSummaries, "Execute custom SQL")

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

		result.Steps = append(result.Steps, model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        displayName,
			Description: desc,
			Type:        stepType,
			Language:    "liquibase",
			SourceFile:  cl.FilePath,
			LineNumber:  cs.LineNumber,
			Metadata:    stepMeta,
		})

		// Sequential execution link
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

		tblStepID := fmt.Sprintf("%s:table:%s", cl.FilePath, tbl.Name)
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

		result.Steps = append(result.Steps, model.Step{
			ID:          tblStepID,
			SwimlaneID:  swimlaneID,
			Name:        tbl.Name,
			Description: desc,
			Type:        "DatabaseTable",
			Language:    "liquibase",
			SourceFile:  cl.FilePath,
			Metadata: map[string]any{
				"table":        tbl.Name,
				"columns":      colsList,
				"foreign_keys": fksList,
				"is_liquibase": true,
			},
		})

		// Emit intra-file foreign key relationships between tables
		for _, fk := range tbl.ForeignKeys {
			if _, exists := tables[fk.ReferencedTable]; exists {
				refTableID := fmt.Sprintf("%s:table:%s", cl.FilePath, fk.ReferencedTable)
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
