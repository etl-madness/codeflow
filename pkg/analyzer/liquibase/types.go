package liquibase

// ChangeLogEntryType defines whether an entry is a changeSet or an include directive.
type ChangeLogEntryType string

const (
	EntryChangeSet  ChangeLogEntryType = "changeSet"
	EntryInclude    ChangeLogEntryType = "include"
	EntryIncludeAll ChangeLogEntryType = "includeAll"
)

// ChangeLogEntry preserves document-order sequence of elements in a changelog.
type ChangeLogEntry struct {
	Type      ChangeLogEntryType
	ChangeSet *ChangeSet
	Include   *IncludeFile
}

// SQLFileRef represents a reference to an external SQL file from a changeSet.
type SQLFileRef struct {
	ChangeSetID  string
	SourceFile   string
	RawPath      string
	ResolvedPath string
}

// OrderedExecution represents an execution plan of changesets in sequence.
type OrderedExecution struct {
	PrimaryFile  string
	ChangeSets   []ChangeSet
	FileOrder    []string
	HandledFiles map[string]bool
	SQLFiles     []SQLFileRef
}

// ChangeLog represents a parsed Liquibase changelog file.
type ChangeLog struct {
	FilePath   string
	ChangeSets []ChangeSet
	Includes   []IncludeFile
	Entries    []ChangeLogEntry
}

// IncludeFile represents an <include> or <includeAll> directive.
type IncludeFile struct {
	File                   string
	Path                   string
	RelativeToChangeLogFile bool
	IsAll                  bool
}

// ChangeSet represents a single unit of change in Liquibase.
type ChangeSet struct {
	ID           string
	Author       string
	FilePath     string
	LineNumber   int
	Contexts     string
	Labels       string
	RunOnChange  bool
	RunAlways    bool
	FailOnError  bool
	Comment      string
	PreCondition string
	Changes      []Change
	Rollbacks    []string
}

// Change represents an individual operation within a changeSet.
type Change struct {
	Type         string // "createTable", "addColumn", "addForeignKeyConstraint", "createView", "createProcedure", "sql", etc.
	TableName    string
	Columns      []ColumnDef
	FKConstraint *FKConstraintDef
	PKConstraint *PKConstraintDef
	RawSQL       string
	Remarks      string
	OldName      string
	NewName      string
	ViewName     string
	ProcedureName string
	IndexName    string
	Tag          string
}

// ColumnDef represents a column definition in a table.
type ColumnDef struct {
	Name          string
	Type          string
	DefaultValue  string
	AutoIncrement bool
	Remarks       string
	PrimaryKey    bool
	Nullable      bool
	Unique        bool
	References    string
}

// FKConstraintDef represents a foreign key constraint definition.
type FKConstraintDef struct {
	ConstraintName    string
	BaseTableName     string
	BaseColumnNames   string
	ReferencedTable   string
	ReferencedColumns string
}

// PKConstraintDef represents a primary key constraint definition.
type PKConstraintDef struct {
	ConstraintName string
	TableName      string
	ColumnNames    string
}
