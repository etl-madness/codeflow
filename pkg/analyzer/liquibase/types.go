package liquibase

// ChangeLog represents a parsed Liquibase changelog file.
type ChangeLog struct {
	FilePath   string
	ChangeSets []ChangeSet
	Includes   []IncludeFile
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
