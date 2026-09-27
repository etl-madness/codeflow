package ssis

import (
	"testing"
)

func TestSSISAnalyzer_ExecuteSQLTask(t *testing.T) {
	dtsxContent := `<?xml version="1.0"?>
<DTS:Executable xmlns:DTS="www.microsoft.com/SqlServer/Dts"
  DTS:refId="Package"
  DTS:ObjectName="7045kHz_cross_db_collection"
  DTS:ExecutableType="Microsoft.Package">
  <DTS:ConnectionManagers>
    <DTS:ConnectionManager
      DTS:refId="Package.ConnectionManagers[T15P.TRAIN]"
      DTS:CreationName="OLEDB"
      DTS:DTSID="{4FCB8913-2050-41CA-B3E9-38E9CA5027EC}"
      DTS:ObjectName="T15P.TRAIN">
      <DTS:ObjectData>
        <DTS:ConnectionManager
          DTS:ConnectionString="Data Source=T15P;Initial Catalog=TRAIN;Provider=SQLOLEDB.1;Integrated Security=SSPI;" />
      </DTS:ObjectData>
    </DTS:ConnectionManager>
  </DTS:ConnectionManagers>
  <DTS:Executables>
    <DTS:Executable
      DTS:refId="Package\Crossdatabase_Dependencies_Collection"
      DTS:CreationName="Microsoft.ExecuteSQLTask"
      DTS:ObjectName="Crossdatabase_Dependencies_Collection">
      <DTS:ObjectData>
        <SQLTask:SqlTaskData
          SQLTask:Connection="{4FCB8913-2050-41CA-B3E9-38E9CA5027EC}"
          SQLTask:SqlStatementSource="EXEC [dbo].[get_crossdatabase_dependencies]" xmlns:SQLTask="www.microsoft.com/sqlserver/dts/tasks/sqltask" />
      </DTS:ObjectData>
    </DTS:Executable>
  </DTS:Executables>
</DTS:Executable>`

	az := New()
	res, err := az.AnalyzeFile("7045kHz_cross_db_collection.dtsx", []byte(dtsxContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Swimlanes) < 2 {
		t.Fatalf("expected at least 2 swimlanes (package + DB), got %d", len(res.Swimlanes))
	}

	if len(res.Steps) < 2 {
		t.Fatalf("expected at least 2 steps (SQL task + stored proc), got %d", len(res.Steps))
	}

	var foundSQLTask, foundProc bool
	for _, s := range res.Steps {
		if s.Name == "Crossdatabase_Dependencies_Collection" && s.Type == "StoredProcedure" {
			foundSQLTask = true
		}
		if s.Name == "dbo.get_crossdatabase_dependencies" && s.Type == "StoredProcedure" {
			foundProc = true
		}
	}

	if !foundSQLTask {
		t.Errorf("failed to find Crossdatabase_Dependencies_Collection step")
	}
	if !foundProc {
		t.Errorf("failed to find dbo.get_crossdatabase_dependencies step")
	}

	if len(res.Links) == 0 {
		t.Errorf("expected cross-service link from task to stored procedure")
	} else {
		if res.Links[0].Label != "EXEC" {
			t.Errorf("expected link label 'EXEC', got '%s'", res.Links[0].Label)
		}
		if !res.Links[0].IsCrossService {
			t.Errorf("expected link to be IsCrossService=true")
		}
	}
}

func TestSSISAnalyzer_PrecedenceConstraintsAndVariables(t *testing.T) {
	dtsxContent := `<?xml version="1.0"?>
<DTS:Executable xmlns:DTS="www.microsoft.com/SqlServer/Dts"
  DTS:refId="Package"
  DTS:ObjectName="DupeAlertFail"
  DTS:ExecutableType="Microsoft.Package">
  <DTS:Variables>
    <DTS:Variable
      DTS:Namespace="User"
      DTS:ObjectName="TOTAL_DUPS">
      <DTS:VariableValue DTS:DataType="3">0</DTS:VariableValue>
    </DTS:Variable>
  </DTS:Variables>
  <DTS:Executables>
    <DTS:Executable
      DTS:refId="Package\CheckDupCount"
      DTS:CreationName="Microsoft.ExecuteSQLTask"
      DTS:ObjectName="CheckDupCount" />
    <DTS:Executable
      DTS:refId="Package\DO IF DUPES"
      DTS:CreationName="Microsoft.Pipeline"
      DTS:ObjectName="DO IF DUPES" />
    <DTS:Executable
      DTS:refId="Package\DO IT NO DUPES"
      DTS:CreationName="Microsoft.Pipeline"
      DTS:ObjectName="DO IT NO DUPES" />
  </DTS:Executables>
  <DTS:PrecedenceConstraints>
    <DTS:PrecedenceConstraint
      DTS:refId="Package.PrecedenceConstraints[Constraint]"
      DTS:EvalOp="1"
      DTS:Expression="@[User::TOTAL_DUPS] &gt; 0"
      DTS:From="Package\CheckDupCount"
      DTS:To="Package\DO IF DUPES" />
    <DTS:PrecedenceConstraint
      DTS:refId="Package.PrecedenceConstraints[Constraint 1]"
      DTS:EvalOp="1"
      DTS:Expression="@[User::TOTAL_DUPS] == 0"
      DTS:From="Package\CheckDupCount"
      DTS:To="Package\DO IT NO DUPES" />
  </DTS:PrecedenceConstraints>
</DTS:Executable>`

	az := New()
	res, err := az.AnalyzeFile("DupeAlertFail.dtsx", []byte(dtsxContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(res.Steps))
	}

	if len(res.Links) != 2 {
		t.Fatalf("expected 2 precedence constraint links, got %d", len(res.Links))
	}

	foundGT, foundEQ := false, false
	for _, l := range res.Links {
		if l.Condition == "TOTAL_DUPS > 0" {
			foundGT = true
		}
		if l.Condition == "TOTAL_DUPS == 0" {
			foundEQ = true
		}
	}
	if !foundGT || !foundEQ {
		t.Errorf("precedence conditions not properly formatted: %+v", res.Links)
	}
}

func TestSSISAnalyzer_DataFlowTask(t *testing.T) {
	dtsxContent := `<?xml version="1.0"?>
<DTS:Executable xmlns:DTS="www.microsoft.com/SqlServer/Dts"
  DTS:refId="Package"
  DTS:ObjectName="Loader"
  DTS:ExecutableType="Microsoft.Package">
  <DTS:Executables>
    <DTS:Executable
      DTS:refId="Package\Data Flow Task"
      DTS:CreationName="Microsoft.Pipeline"
      DTS:ObjectName="Data Flow Task">
      <DTS:ObjectData>
        <pipeline version="1">
          <components>
            <component name="Flat File Source" componentClassID="Microsoft.FlatFileSource" />
            <component name="Data Conversion" componentClassID="Microsoft.DataConvert" />
            <component name="OLE DB Destination" componentClassID="Microsoft.OLEDBDestination">
              <properties>
                <property name="OpenRowset">[temp].[SOURCE_DUPES]</property>
              </properties>
            </component>
          </components>
        </pipeline>
      </DTS:ObjectData>
    </DTS:Executable>
  </DTS:Executables>
</DTS:Executable>`

	az := New()
	res, err := az.AnalyzeFile("Loader.dtsx", []byte(dtsxContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Steps) < 2 {
		t.Fatalf("expected at least 2 steps (Data Flow Task + destination table), got %d", len(res.Steps))
	}

	var foundPipeline, foundTable bool
	for _, s := range res.Steps {
		if s.Name == "Data Flow Task" && s.Type == "WorkflowStep" {
			foundPipeline = true
		}
		if s.Name == "temp.SOURCE_DUPES" && s.Type == "DatabaseTable" {
			foundTable = true
		}
	}

	if !foundPipeline {
		t.Errorf("failed to find Data Flow Task step")
	}
	if !foundTable {
		t.Errorf("failed to find temp.SOURCE_DUPES table step")
	}

	if len(res.Links) == 0 {
		t.Errorf("expected link from pipeline to destination table")
	} else if res.Links[0].Label != "LOAD" {
		t.Errorf("expected link label 'LOAD', got '%s'", res.Links[0].Label)
	}
}

