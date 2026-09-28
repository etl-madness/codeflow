package liquibase_test

import (
	"os"
	"strings"
	"testing"

	"codeflow/pkg/analyzer/liquibase"
	"codeflow/pkg/exporter/mermaid"
	"codeflow/pkg/exporter/svg"
	"codeflow/pkg/model"
)

func TestLiquibaseXMLAnalyzer(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<databaseChangeLog
    xmlns="http://www.liquibase.org/xml/ns/dbchangelog"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://www.liquibase.org/xml/ns/dbchangelog
        http://www.liquibase.org/xml/ns/dbchangelog/dbchangelog-latest.xsd">

    <changeSet id="1" author="alice" context="dev,prod" labels="v1.0" runOnChange="true">
        <comment>Create customers table</comment>
        <createTable tableName="customers" remarks="Customer registry">
            <column name="id" type="bigint" autoIncrement="true">
                <constraints primaryKey="true" nullable="false"/>
            </column>
            <column name="name" type="varchar(100)">
                <constraints nullable="false"/>
            </column>
            <column name="email" type="varchar(255)">
                <constraints unique="true"/>
            </column>
        </createTable>
        <rollback>
            DROP TABLE customers;
        </rollback>
    </changeSet>

    <changeSet id="2" author="alice">
        <createTable tableName="orders" remarks="Customer purchase orders">
            <column name="id" type="bigint" autoIncrement="true">
                <constraints primaryKey="true" nullable="false"/>
            </column>
            <column name="customer_id" type="bigint">
                <constraints nullable="false"/>
            </column>
            <column name="total_amount" type="decimal(10,2)"/>
        </createTable>
        <addForeignKeyConstraint
            constraintName="fk_orders_customer"
            baseTableName="orders"
            baseColumnNames="customer_id"
            referencedTableName="customers"
            referencedColumnNames="id"/>
    </changeSet>

    <changeSet id="3" author="bob">
        <addColumn tableName="orders">
            <column name="status" type="varchar(50)" defaultValue="PENDING"/>
        </addColumn>
        <createIndex indexName="idx_orders_status" tableName="orders">
            <column name="status"/>
        </createIndex>
    </changeSet>

    <changeSet id="4" author="bob">
        <createView viewName="v_customer_orders">
            SELECT c.name, o.total_amount FROM customers c JOIN orders o ON c.id = o.customer_id
        </createView>
        <tagDatabase tag="v1.0"/>
    </changeSet>
</databaseChangeLog>`

	az := liquibase.New()
	res, err := az.AnalyzeFile("db/changelog/001_initial.xml", []byte(xmlContent))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(res.Swimlanes) != 1 {
		t.Fatalf("expected 1 swimlane, got %d", len(res.Swimlanes))
	}
	if !strings.Contains(res.Swimlanes[0].Name, "001_initial") {
		t.Errorf("unexpected swimlane name: %s", res.Swimlanes[0].Name)
	}

	// 4 ChangeSets + 2 DatabaseTable entities (customers, orders) = 6 steps
	if len(res.Steps) < 6 {
		t.Fatalf("expected at least 6 steps, got %d", len(res.Steps))
	}

	// Verify ChangeSet 1
	var cs1 *model.Step
	for i := range res.Steps {
		if res.Steps[i].Name == "alice:1" {
			cs1 = &res.Steps[i]
			break
		}
	}
	if cs1 == nil {
		t.Fatal("ChangeSet alice:1 not found")
	}
	if cs1.Description != "Create customers table" {
		t.Errorf("expected comment description, got: %s", cs1.Description)
	}
	if cs1.Metadata["author"] != "alice" {
		t.Errorf("expected author alice, got: %v", cs1.Metadata["author"])
	}
	if cs1.Metadata["contexts"] != "dev,prod" {
		t.Errorf("expected contexts dev,prod, got: %v", cs1.Metadata["contexts"])
	}

	// Verify cumulative schema for orders table (should have columns: id, customer_id, total_amount, status)
	var ordersTable *model.Step
	for i := range res.Steps {
		if res.Steps[i].Type == "DatabaseTable" && res.Steps[i].Name == "orders" {
			ordersTable = &res.Steps[i]
			break
		}
	}
	if ordersTable == nil {
		t.Fatal("DatabaseTable orders not found in steps")
	}

	cols, ok := ordersTable.Metadata["columns"].([]map[string]string)
	if !ok {
		t.Fatalf("expected columns slice in table metadata, got: %T", ordersTable.Metadata["columns"])
	}
	if len(cols) != 4 {
		t.Errorf("expected 4 columns in orders (id, customer_id, total_amount, status), got %d", len(cols))
	}

	// Verify Foreign Key link
	var fkLink *model.Link
	for i := range res.Links {
		if strings.Contains(res.Links[i].Label, "fk_orders_customer") {
			fkLink = &res.Links[i]
			break
		}
	}
	if fkLink == nil {
		t.Error("Foreign key link fk_orders_customer not found in links")
	}

	// Test Mermaid and SVG ER Diagram generation
	pm := &model.ProcessModel{
		ID:        "test-liquibase",
		Name:      "Liquibase Test",
		Swimlanes: res.Swimlanes,
		Steps:     res.Steps,
		Links:     res.Links,
	}

	mmdExp := mermaid.New()
	erMMD := mmdExp.GenerateERDiagram(pm)
	if !strings.Contains(erMMD, "customers {") || !strings.Contains(erMMD, "orders {") {
		t.Errorf("Mermaid ER diagram missing table blocks: %s", erMMD)
	}
	if !strings.Contains(erMMD, "bigint id PK") {
		t.Errorf("Mermaid ER diagram missing PK definition: %s", erMMD)
	}

	svgExp := svg.New()
	erSVG := svgExp.GenerateERDiagram(pm)
	if !strings.Contains(erSVG, "customers") || !strings.Contains(erSVG, "orders") {
		t.Errorf("SVG ER diagram missing table names")
	}
}

func TestLiquibaseFormattedSQLAnalyzer(t *testing.T) {
	sqlContent := `--liquibase formatted sql

--changeset developer1:init-schema context:dev labels:v1.0 runOnChange:false
--comment: Initial customer table setup
CREATE TABLE customer (
    id INT PRIMARY KEY,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    email VARCHAR(100) UNIQUE
);
--rollback DROP TABLE customer;

--changeset developer2:orders-schema
--comment: Create orders and link to customer
CREATE TABLE orders (
    id INT PRIMARY KEY,
    customer_id INT,
    order_date DATETIME,
    CONSTRAINT fk_orders_cust FOREIGN KEY (customer_id) REFERENCES customer(id)
);
--rollback DROP TABLE orders;

--changeset developer2:create-summary-view
CREATE VIEW v_order_summary AS
SELECT customer_id, COUNT(*) as total_orders
FROM orders
GROUP BY customer_id;
`

	az := liquibase.New()
	res, err := az.AnalyzeFile("migrations/002_orders.sql", []byte(sqlContent))
	if err != nil {
		t.Fatalf("AnalyzeFile failed on Formatted SQL: %v", err)
	}

	if len(res.Swimlanes) != 1 {
		t.Fatalf("expected 1 swimlane, got %d", len(res.Swimlanes))
	}

	// 3 changesets + 2 tables (customer, orders) = at least 5 steps
	if len(res.Steps) < 5 {
		t.Fatalf("expected at least 5 steps, got %d", len(res.Steps))
	}

	// Verify Rollback captured
	var cs1 *model.Step
	for i := range res.Steps {
		if res.Steps[i].Name == "developer1:init-schema" {
			cs1 = &res.Steps[i]
			break
		}
	}
	if cs1 == nil {
		t.Fatal("developer1:init-schema not found")
	}
	if cs1.Metadata["rollback"] != "DROP TABLE customer;" {
		t.Errorf("expected rollback 'DROP TABLE customer;', got: %v", cs1.Metadata["rollback"])
	}

	// Verify Foreign Key relationship link
	var fkLink *model.Link
	for i := range res.Links {
		if strings.Contains(res.Links[i].Label, "fk_orders_cust") {
			fkLink = &res.Links[i]
			break
		}
	}
	if fkLink == nil {
		t.Error("expected foreign key link fk_orders_cust not found")
	}
}

func TestLiquibasePrimaryChangelogOrder(t *testing.T) {
	primaryPath := "../../../examples/liquibase/changesets.xml"
	if _, err := os.Stat(primaryPath); err != nil {
		primaryPath = "../../examples/liquibase/changesets.xml"
	}
	if _, err := os.Stat(primaryPath); err != nil {
		primaryPath = "examples/liquibase/changesets.xml"
	}
	az := liquibase.New()

	res, err := az.AnalyzeWithPrimaryOrder(primaryPath)
	if err != nil {
		t.Fatalf("AnalyzeWithPrimaryOrder failed: %v", err)
	}

	// Verify swimlanes order
	if len(res.Swimlanes) < 3 {
		t.Fatalf("expected at least 3 swimlanes, got %d", len(res.Swimlanes))
	}
	if !strings.Contains(res.Swimlanes[0].Name, "core_changeset") {
		t.Errorf("expected first swimlane to be core_changeset, got: %s", res.Swimlanes[0].Name)
	}
	if !strings.Contains(res.Swimlanes[1].Name, "customer_changeset") {
		t.Errorf("expected second swimlane to be customer_changeset, got: %s", res.Swimlanes[1].Name)
	}
	if !strings.Contains(res.Swimlanes[2].Name, "sql_changeset") {
		t.Errorf("expected third swimlane to be sql_changeset, got: %s", res.Swimlanes[2].Name)
	}

	// Verify all changeSets exist in order
	expectedOrder := []string{
		"db_admin:1",
		"db_admin:2",
		"db_admin:3",
		"me:20260927001",
		"architect:orders-v1",
		"architect:payments-v1",
		"architect:views-and-procs",
		"architect:milestone-v1.1",
	}

	var foundSteps []string
	for _, s := range res.Steps {
		if s.Type != "DatabaseTable" {
			foundSteps = append(foundSteps, s.Name)
		}
	}

	if len(foundSteps) != len(expectedOrder) {
		t.Fatalf("expected %d changeset steps, got %d: %v", len(expectedOrder), len(foundSteps), foundSteps)
	}

	for i, expected := range expectedOrder {
		if i == 3 {
			if !strings.HasPrefix(foundSteps[i], "me:") {
				t.Errorf("step %d: expected author me, got %s", i, foundSteps[i])
			}
			continue
		}
		if foundSteps[i] != expected {
			t.Errorf("step %d: expected %s, got %s", i, expected, foundSteps[i])
		}
	}

	// Verify continuous sequential link bridging across file boundary
	var crossFileExecLink *model.Link
	for i := range res.Links {
		l := &res.Links[i]
		if l.Label == "executes next" && strings.Contains(l.SourceStepID, "core_changeset.1.xml:3") && strings.Contains(l.TargetStepID, "customer_changeset.1.xml:") {
			crossFileExecLink = l
			break
		}
	}
	if crossFileExecLink == nil {
		t.Errorf("expected cross-file execution link from core_changeset to customer_changeset")
	}

	// Verify cross-file foreign key relationship
	var crossFileFK *model.Link
	for i := range res.Links {
		l := &res.Links[i]
		if strings.Contains(l.Label, "fk_orders_customer") {
			crossFileFK = l
			break
		}
	}
	if crossFileFK == nil {
		t.Errorf("expected cross-file FK link fk_orders_customer from orders to customers")
	}
}
