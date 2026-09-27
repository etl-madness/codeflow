package flowxml

import (
	"testing"
)

func TestFlowXMLAnalyzer(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="utf-8"?>
<workflow id="wf-billing" name="Billing Workflow">
    <step id="s1" name="AuthorizeCard" type="PaymentStep" swimlane="Payments" line="10">
        <description>Authorizes payment card</description>
    </step>
    <step id="s2" name="GenerateInvoice" type="BillingStep" swimlane="Invoicing" line="25">
        <description>Generates PDF invoice</description>
    </step>
    <transition id="t1" from="s1" to="s2" label="On Approved" condition="status == 200" />
</workflow>
`

	analyzer := New()
	result, err := analyzer.AnalyzeFile("billing.xml", []byte(xmlContent))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}

	if len(result.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(result.Links))
	}

	link := result.Links[0]
	if link.SourceStepID != "s1" || link.TargetStepID != "s2" {
		t.Errorf("unexpected link endpoints: %+v", link)
	}
	if link.Condition != "status == 200" {
		t.Errorf("expected condition 'status == 200', got %v", link.Condition)
	}
}

func TestSSISXMLAnalyzer(t *testing.T) {
	dtsxContent := `<?xml version="1.0"?>
<DTS:Executable xmlns:DTS="www.microsoft.com/SqlServer/Dts">
    <DTS:Executables>
        <DTS:Executable DTS:ObjectName="Extract Source Data" DTS:CreationName="Microsoft.Pipeline" />
        <DTS:Executable DTS:ObjectName="Load Warehouse" DTS:CreationName="Microsoft.ExecuteSQLTask" />
    </DTS:Executables>
</DTS:Executable>
`
	analyzer := New()
	result, err := analyzer.AnalyzeFile("package.dtsx", []byte(dtsxContent))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps from SSIS package, got %d", len(result.Steps))
	}

	if result.Steps[0].Name != "Extract Source Data" {
		t.Errorf("expected step name 'Extract Source Data', got %s", result.Steps[0].Name)
	}
}

func TestPipelineXMLAnalyzer(t *testing.T) {
	xmlContent := `<pipeline description="Sample ETL Pipeline">
    <variables>
        <variable name="GCLOUD_JSON" value="" />
        <variable name="TABLE" value="dbo.BillingReport" />
    </variables>
    <databases>
        <database name="primary_db" description="Primary SQL DB" />
    </databases>
    <preflight>
        <sql id="preflight_check" db="primary_db" description="Check DB connectivity">
            SELECT 1;
        </sql>
        <assert id="db_assert" var="CHECK_RESULT" equals="1" description="Ensure DB online" />
    </preflight>
    <flow>
        <script id="ExtractData" language="bash" description="Extract from GCP" output_var="GCLOUD_JSON">
            ./extract.exe
        </script>
        <sql id="LoadData" db="primary_db" description="Load into SQL">
            INSERT INTO {{TABLE}} SELECT * FROM OPENJSON('{{GCLOUD_JSON}}')
        </sql>
    </flow>
</pipeline>
`

	analyzer := New()
	result, err := analyzer.AnalyzeFile("gcloud_billing.xml", []byte(xmlContent))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Steps: preflight_check, db_assert, ExtractData, LoadData, plus table dbo.BillingReport
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps (including database table), got %d", len(result.Steps))
	}

	// Swimlanes: Preflight, Flow, and Database (primary_db)
	if len(result.Swimlanes) != 3 {
		t.Fatalf("expected 3 swimlanes, got %d", len(result.Swimlanes))
	}

	// Verify links
	if len(result.Links) < 4 {
		t.Fatalf("expected at least 4 links, got %d", len(result.Links))
	}

	// Check table insertion link exists
	hasInsertLink := false
	for _, l := range result.Links {
		if l.SourceStepID == "LoadData" && l.Label == "INSERT INTO" && l.IsCrossService {
			hasInsertLink = true
			break
		}
	}
	if !hasInsertLink {
		t.Errorf("expected cross-service INSERT INTO link from LoadData to dbo.BillingReport")
	}
}

