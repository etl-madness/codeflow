package liquibase

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// XML Changelog Root
type xmlDatabaseChangeLog struct {
	XMLName    xml.Name       `xml:"databaseChangeLog"`
	Includes   []xmlInclude   `xml:"include"`
	IncludeAll []xmlIncludeAll `xml:"includeAll"`
	ChangeSets []xmlChangeSet `xml:"changeSet"`
}

type xmlInclude struct {
	File                   string `xml:"file,attr"`
	RelativeToChangeLogFile string `xml:"relativeToChangelogFile,attr"`
}

type xmlIncludeAll struct {
	Path                   string `xml:"path,attr"`
	RelativeToChangeLogFile string `xml:"relativeToChangelogFile,attr"`
}

type xmlChangeSet struct {
	ID          string `xml:"id,attr"`
	Author      string `xml:"author,attr"`
	Context     string `xml:"context,attr"`
	Labels      string `xml:"labels,attr"`
	RunOnChange string `xml:"runOnChange,attr"`
	RunAlways   string `xml:"runAlways,attr"`
	FailOnError string `xml:"failOnError,attr"`
	Comment     string `xml:"comment"`

	// Direct child elements
	CreateTables          []xmlCreateTable          `xml:"createTable"`
	DropTables            []xmlDropTable            `xml:"dropTable"`
	AddColumns            []xmlAddColumn            `xml:"addColumn"`
	DropColumns           []xmlDropColumn           `xml:"dropColumn"`
	ModifyDataTypes       []xmlModifyDataType       `xml:"modifyDataType"`
	RenameColumns         []xmlRenameColumn         `xml:"renameColumn"`
	RenameTables          []xmlRenameTable          `xml:"renameTable"`
	AddPrimaryKeys        []xmlAddPrimaryKey        `xml:"addPrimaryKey"`
	DropPrimaryKeys       []xmlDropPrimaryKey       `xml:"dropPrimaryKey"`
	AddForeignKeys        []xmlAddForeignKey        `xml:"addForeignKeyConstraint"`
	DropForeignKeys       []xmlDropForeignKey       `xml:"dropForeignKeyConstraint"`
	CreateIndexes         []xmlCreateIndex          `xml:"createIndex"`
	DropIndexes           []xmlDropIndex            `xml:"dropIndex"`
	AddUniqueConstraints  []xmlAddUniqueConstraint  `xml:"addUniqueConstraint"`
	CreateViews           []xmlCreateView           `xml:"createView"`
	DropViews             []xmlDropView             `xml:"dropView"`
	CreateProcedures      []xmlCreateProcedure      `xml:"createProcedure"`
	DropProcedures        []xmlDropProcedure        `xml:"dropProcedure"`
	SQLs                  []xmlSQL                  `xml:"sql"`
	SQLFiles              []xmlSQLFile              `xml:"sqlFile"`
	TagDatabases          []xmlTagDatabase          `xml:"tagDatabase"`
	Rollbacks             []xmlRollback             `xml:"rollback"`
}

type xmlCreateTable struct {
	TableName string      `xml:"tableName,attr"`
	Remarks   string      `xml:"remarks,attr"`
	Columns   []xmlColumn `xml:"column"`
}

type xmlDropTable struct {
	TableName string `xml:"tableName,attr"`
}

type xmlAddColumn struct {
	TableName string      `xml:"tableName,attr"`
	Columns   []xmlColumn `xml:"column"`
}

type xmlDropColumn struct {
	TableName  string `xml:"tableName,attr"`
	ColumnName string `xml:"columnName,attr"`
}

type xmlModifyDataType struct {
	TableName   string `xml:"tableName,attr"`
	ColumnName  string `xml:"columnName,attr"`
	NewDataType string `xml:"newDataType,attr"`
}

type xmlRenameColumn struct {
	TableName     string `xml:"tableName,attr"`
	OldColumnName string `xml:"oldColumnName,attr"`
	NewColumnName string `xml:"newColumnName,attr"`
}

type xmlRenameTable struct {
	OldTableName string `xml:"oldTableName,attr"`
	NewTableName string `xml:"newTableName,attr"`
}

type xmlAddPrimaryKey struct {
	TableName      string `xml:"tableName,attr"`
	ColumnNames    string `xml:"columnNames,attr"`
	ConstraintName string `xml:"constraintName,attr"`
}

type xmlDropPrimaryKey struct {
	TableName      string `xml:"tableName,attr"`
	ConstraintName string `xml:"constraintName,attr"`
}

type xmlAddForeignKey struct {
	ConstraintName    string `xml:"constraintName,attr"`
	BaseTableName     string `xml:"baseTableName,attr"`
	BaseColumnNames   string `xml:"baseColumnNames,attr"`
	ReferencedTable   string `xml:"referencedTableName,attr"`
	ReferencedColumns string `xml:"referencedColumnNames,attr"`
}

type xmlDropForeignKey struct {
	BaseTableName  string `xml:"baseTableName,attr"`
	ConstraintName string `xml:"constraintName,attr"`
}

type xmlCreateIndex struct {
	IndexName string      `xml:"indexName,attr"`
	TableName string      `xml:"tableName,attr"`
	Unique    string      `xml:"unique,attr"`
	Columns   []xmlColumn `xml:"column"`
}

type xmlDropIndex struct {
	IndexName string `xml:"indexName,attr"`
	TableName string `xml:"tableName,attr"`
}

type xmlAddUniqueConstraint struct {
	TableName   string `xml:"tableName,attr"`
	ColumnNames string `xml:"columnNames,attr"`
}

type xmlCreateView struct {
	ViewName string `xml:"viewName,attr"`
	Body     string `xml:",chardata"`
}

type xmlDropView struct {
	ViewName string `xml:"viewName,attr"`
}

type xmlCreateProcedure struct {
	ProcedureName string `xml:"procedureName,attr"`
	Body          string `xml:",chardata"`
}

type xmlDropProcedure struct {
	ProcedureName string `xml:"procedureName,attr"`
}

type xmlSQL struct {
	Text string `xml:",chardata"`
}

type xmlSQLFile struct {
	Path string `xml:"path,attr"`
}

type xmlTagDatabase struct {
	Tag string `xml:"tag,attr"`
}

type xmlRollback struct {
	Text string `xml:",chardata"`
}

type xmlColumn struct {
	Name          string         `xml:"name,attr"`
	Type          string         `xml:"type,attr"`
	DefaultValue  string         `xml:"defaultValue,attr"`
	AutoIncrement string         `xml:"autoIncrement,attr"`
	Remarks       string         `xml:"remarks,attr"`
	Constraints   xmlConstraints `xml:"constraints"`
}

type xmlConstraints struct {
	PrimaryKey string `xml:"primaryKey,attr"`
	Nullable   string `xml:"nullable,attr"`
	Unique     string `xml:"unique,attr"`
	References string `xml:"references,attr"`
}

// ParseXMLChangeLog unmarshals a Liquibase XML changelog preserving document element order.
func ParseXMLChangeLog(filePath string, content []byte) (*ChangeLog, error) {
	cl := &ChangeLog{
		FilePath: filePath,
	}

	lineNumbers := findChangeSetLineNumbers(content)

	decoder := xml.NewDecoder(bytes.NewReader(content))
	for {
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		se, ok := t.(xml.StartElement)
		if !ok {
			continue
		}

		switch se.Name.Local {
		case "include":
			var inc xmlInclude
			if err := decoder.DecodeElement(&inc, &se); err == nil {
				rel := strings.EqualFold(inc.RelativeToChangeLogFile, "true") || inc.RelativeToChangeLogFile == "1"
				incFile := IncludeFile{
					File:                   inc.File,
					RelativeToChangeLogFile: rel,
					IsAll:                  false,
				}
				cl.Includes = append(cl.Includes, incFile)
				cl.Entries = append(cl.Entries, ChangeLogEntry{
					Type:    EntryInclude,
					Include: &incFile,
				})
			}

		case "includeAll":
			var incAll xmlIncludeAll
			if err := decoder.DecodeElement(&incAll, &se); err == nil {
				rel := strings.EqualFold(incAll.RelativeToChangeLogFile, "true") || incAll.RelativeToChangeLogFile == "1"
				incFile := IncludeFile{
					Path:                   incAll.Path,
					RelativeToChangeLogFile: rel,
					IsAll:                  true,
				}
				cl.Includes = append(cl.Includes, incFile)
				cl.Entries = append(cl.Entries, ChangeLogEntry{
					Type:    EntryIncludeAll,
					Include: &incFile,
				})
			}

		case "changeSet":
			var cs xmlChangeSet
			if err := decoder.DecodeElement(&cs, &se); err == nil {
				line := len(cl.ChangeSets) + 1
				if l, ok := lineNumbers[cs.ID]; ok {
					line = l
				}
				changeSet := convertXMLChangeSet(cs, filePath, line)
				cl.ChangeSets = append(cl.ChangeSets, changeSet)
				cl.Entries = append(cl.Entries, ChangeLogEntry{
					Type:      EntryChangeSet,
					ChangeSet: &changeSet,
				})
			}
		}
	}

	return cl, nil
}

func convertXMLChangeSet(cs xmlChangeSet, filePath string, line int) ChangeSet {
	changeSet := ChangeSet{
		ID:          cs.ID,
		Author:      cs.Author,
		FilePath:    filePath,
		LineNumber:  line,
		Contexts:    cs.Context,
		Labels:      cs.Labels,
		RunOnChange: parseBool(cs.RunOnChange),
		RunAlways:   parseBool(cs.RunAlways),
		FailOnError: parseBoolDefaultTrue(cs.FailOnError),
		Comment:     strings.TrimSpace(cs.Comment),
	}

	for _, ct := range cs.CreateTables {
		c := Change{
			Type:      "createTable",
			TableName: ct.TableName,
			Remarks:   ct.Remarks,
		}
		for _, col := range ct.Columns {
			c.Columns = append(c.Columns, mapColumnDef(col))
		}
		changeSet.Changes = append(changeSet.Changes, c)
	}

	for _, dt := range cs.DropTables {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "dropTable",
			TableName: dt.TableName,
		})
	}

	for _, ac := range cs.AddColumns {
		c := Change{
			Type:      "addColumn",
			TableName: ac.TableName,
		}
		for _, col := range ac.Columns {
			c.Columns = append(c.Columns, mapColumnDef(col))
		}
		changeSet.Changes = append(changeSet.Changes, c)
	}

	for _, dc := range cs.DropColumns {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "dropColumn",
			TableName: dc.TableName,
			Columns:   []ColumnDef{{Name: dc.ColumnName}},
		})
	}

	for _, md := range cs.ModifyDataTypes {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "modifyDataType",
			TableName: md.TableName,
			Columns:   []ColumnDef{{Name: md.ColumnName, Type: md.NewDataType}},
		})
	}

	for _, rc := range cs.RenameColumns {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "renameColumn",
			TableName: rc.TableName,
			OldName:   rc.OldColumnName,
			NewName:   rc.NewColumnName,
		})
	}

	for _, rt := range cs.RenameTables {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "renameTable",
			TableName: rt.OldTableName,
			OldName:   rt.OldTableName,
			NewName:   rt.NewTableName,
		})
	}

	for _, pk := range cs.AddPrimaryKeys {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "addPrimaryKey",
			TableName: pk.TableName,
			PKConstraint: &PKConstraintDef{
				ConstraintName: pk.ConstraintName,
				TableName:      pk.TableName,
				ColumnNames:    pk.ColumnNames,
			},
		})
	}

	for _, dpk := range cs.DropPrimaryKeys {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "dropPrimaryKey",
			TableName: dpk.TableName,
			PKConstraint: &PKConstraintDef{
				ConstraintName: dpk.ConstraintName,
				TableName:      dpk.TableName,
			},
		})
	}

	for _, fk := range cs.AddForeignKeys {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "addForeignKeyConstraint",
			TableName: fk.BaseTableName,
			FKConstraint: &FKConstraintDef{
				ConstraintName:    fk.ConstraintName,
				BaseTableName:     fk.BaseTableName,
				BaseColumnNames:   fk.BaseColumnNames,
				ReferencedTable:   fk.ReferencedTable,
				ReferencedColumns: fk.ReferencedColumns,
			},
		})
	}

	for _, dfk := range cs.DropForeignKeys {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "dropForeignKeyConstraint",
			TableName: dfk.BaseTableName,
			FKConstraint: &FKConstraintDef{
				ConstraintName: dfk.ConstraintName,
				BaseTableName:  dfk.BaseTableName,
			},
		})
	}

	for _, ci := range cs.CreateIndexes {
		c := Change{
			Type:      "createIndex",
			TableName: ci.TableName,
			IndexName: ci.IndexName,
		}
		for _, col := range ci.Columns {
			c.Columns = append(c.Columns, ColumnDef{Name: col.Name})
		}
		changeSet.Changes = append(changeSet.Changes, c)
	}

	for _, di := range cs.DropIndexes {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "dropIndex",
			TableName: di.TableName,
			IndexName: di.IndexName,
		})
	}

	for _, auc := range cs.AddUniqueConstraints {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:      "addUniqueConstraint",
			TableName: auc.TableName,
			Columns:   []ColumnDef{{Name: auc.ColumnNames, Unique: true}},
		})
	}

	for _, cv := range cs.CreateViews {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:     "createView",
			ViewName: cv.ViewName,
			RawSQL:   strings.TrimSpace(cv.Body),
		})
	}

	for _, dv := range cs.DropViews {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:     "dropView",
			ViewName: dv.ViewName,
		})
	}

	for _, cp := range cs.CreateProcedures {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:          "createProcedure",
			ProcedureName: cp.ProcedureName,
			RawSQL:        strings.TrimSpace(cp.Body),
		})
	}

	for _, dp := range cs.DropProcedures {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:          "dropProcedure",
			ProcedureName: dp.ProcedureName,
		})
	}

	for _, s := range cs.SQLs {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:   "sql",
			RawSQL: strings.TrimSpace(s.Text),
		})
	}

	for _, sf := range cs.SQLFiles {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type:   "sqlFile",
			RawSQL: sf.Path,
		})
	}

	for _, tag := range cs.TagDatabases {
		changeSet.Changes = append(changeSet.Changes, Change{
			Type: "tagDatabase",
			Tag:  tag.Tag,
		})
	}

	for _, rb := range cs.Rollbacks {
		if t := strings.TrimSpace(rb.Text); t != "" {
			changeSet.Rollbacks = append(changeSet.Rollbacks, t)
		}
	}

	return changeSet
}

func mapColumnDef(col xmlColumn) ColumnDef {
	isPK := strings.EqualFold(col.Constraints.PrimaryKey, "true") || col.Constraints.PrimaryKey == "1"
	isNullable := !strings.EqualFold(col.Constraints.Nullable, "false") && col.Constraints.Nullable != "0"
	isUnique := strings.EqualFold(col.Constraints.Unique, "true") || col.Constraints.Unique == "1"
	autoInc := strings.EqualFold(col.AutoIncrement, "true") || col.AutoIncrement == "1"

	return ColumnDef{
		Name:          col.Name,
		Type:          col.Type,
		DefaultValue:  col.DefaultValue,
		AutoIncrement: autoInc,
		Remarks:       col.Remarks,
		PrimaryKey:    isPK,
		Nullable:      isNullable,
		Unique:        isUnique,
		References:    col.Constraints.References,
	}
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(s))
	return b
}

func parseBoolDefaultTrue(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return true
	}
	return b
}

func findChangeSetLineNumbers(content []byte) map[string]int {
	lineMap := make(map[string]int)
	decoder := xml.NewDecoder(bytes.NewReader(content))
	line := 1
	var prevOffset int64

	for {
		token, err := decoder.Token()
		if err == io.EOF || token == nil {
			break
		}
		offset := decoder.InputOffset()
		chunk := content[prevOffset:offset]
		line += bytes.Count(chunk, []byte("\n"))
		prevOffset = offset

		if se, ok := token.(xml.StartElement); ok {
			if strings.EqualFold(se.Name.Local, "changeSet") {
				id := ""
				for _, attr := range se.Attr {
					if strings.EqualFold(attr.Name.Local, "id") {
						id = attr.Value
						break
					}
				}
				if id != "" {
					lineMap[id] = line
				}
			}
		}
	}
	return lineMap
}
