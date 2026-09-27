# Liquibase Changelog Architecture & ER Diagrams

This directory contains visual diagrams, process flows, Entity-Relationship (ER) models, and Visio-compatible Excel workbooks generated from Liquibase database migration changelogs.

Source examples located in: [`examples/liquibase/`](../../examples/liquibase/)
- **Primary Master Changelog**: `examples/liquibase/changesets.xml`
- **Core Schema XML Changelog**: `examples/liquibase/changeset/core_changeset.1.xml`
- **Stored Procedure XML Changelog**: `examples/liquibase/changeset/customer_changeset.1.xml`
- **Commerce Transactions Formatted SQL**: `examples/liquibase/changeset/sql_changeset.1.sql`
- **Stored Procedure DDL**: `examples/liquibase/ddl/dbo_usp_GetCustomer.1.sql`

---

## Primary Execution Order

When analyzing database migrations with Liquibase, master/root changelogs (such as `changesets.xml`, `master.xml`, `db.changelog-master.xml`) specify the precise execution order of child changelogs using `<include>` and `<includeAll>`.

You can specify a primary order changelog using the `--primary-changelog` flag (or pass it directly via `--source`):

```bash
.\bin\codeflow.exe analyze \
  --source ./examples/liquibase \
  --primary-changelog ./examples/liquibase/changesets.xml \
  --recursive \
  --format mermaid,excel,json,svg \
  -n liquibase \
  --diagram-type all \
  --output-dir ./docs/liquibase \
  --no-truncate
```

### Key Execution Capabilities
1. **Deterministic Execution Sequence**: Changsets execute sequentially across changelog files (`core_changeset` &rarr; `customer_changeset` &rarr; `sql_changeset`).
2. **Continuous Execution Chain**: The execution chain (`executes next`) flows continuously across changelog boundaries without interruption.
3. **Cumulative Schema Tracking**: Table structures, column additions, and constraints evolve across files so foreign keys reference tables defined in earlier changelogs.
4. **Multi-Format Inclusions**: Seamlessly traverses XML changelogs, Formatted SQL files (`--liquibase formatted sql`), and `<sqlFile>` references.

---

## 1. Entity-Relationship (ER) Model

Shows the cumulative database schema with primary keys (`PK`), column types, and foreign key relationships (`||--o{`) correlated across changelog files.

### Standalone Vector Image
- **SVG**: [liquibase_er.svg](liquibase_er.svg)

### Mermaid ER Diagram
```mermaid
erDiagram
    customers {
        bigint id PK
        varchar_50 first_name
        varchar_50 last_name
        varchar_255 email
        datetime created_at
    }

    merchants {
        bigint id PK
        varchar_100 merchant_name
        varchar_64 api_key
        varchar_20 status
    }

    orders {
        BIGINT id PK
        BIGINT customer_id
        BIGINT merchant_id
        DECIMAL_12_2 order_total
        VARCHAR_30 order_status
        DATETIME created_at
    }

    payments {
        BIGINT id PK
        BIGINT order_id
        VARCHAR_50 payment_method
        DECIMAL_12_2 amount
        VARCHAR_30 payment_status
        VARCHAR_100 transaction_ref
        DATETIME processed_at
    }

    orders ||--o{ customers : "references (fk_orders_customer)"
    orders ||--o{ merchants : "references (fk_orders_merchant)"
    payments ||--o{ orders : "references (fk_payments_order)"
```

---

## 2. Top-Down Flowchart (Flowchart TD)

Shows changeSets grouped by changelog file swimlanes, unbroken sequential execution order across files, and cross-file foreign key dependencies.

### Standalone Vector Image
- **SVG**: [liquibase.svg](liquibase.svg)

### Mermaid Flowchart
```mermaid
flowchart TD
    classDef default fill:#FFFFFF,stroke:#CBD5E1,stroke-width:1.5px,color:#0F172A,rx:6px,ry:6px;
    classDef defaultNode fill:#FFFFFF,stroke:#CBD5E1,stroke-width:1.5px,color:#0F172A,rx:6px,ry:6px;
    classDef dbNode fill:#FFFFFF,stroke:#10B981,stroke-width:2px,color:#065F46,rx:6px,ry:6px;
    classDef apiNode fill:#FFFFFF,stroke:#3B82F6,stroke-width:2px,color:#1E40AF,rx:6px,ry:6px;
    classDef scriptNode fill:#FFFFFF,stroke:#6366F1,stroke-width:2px,color:#3730A3,rx:6px,ry:6px;
    classDef procNode fill:#FFFFFF,stroke:#8B5CF6,stroke-width:2px,color:#5B21B6,rx:6px,ry:6px;
    classDef taskNode fill:#FFFFFF,stroke:#F59E0B,stroke-width:2px,color:#92400E,rx:6px,ry:6px;
    classDef assertNode fill:#FFFFFF,stroke:#EC4899,stroke-width:2px,color:#9D174D,rx:6px,ry:6px;
    subgraph lane_liquibase_core_changeset_1 ["Liquibase (core_changeset.1)"]
        s1[("<b>db_admin:1</b><br/><small>Table Operation</small><br/><sub>Create customers table</sub>")]:::dbNode
        s2[("<b>db_admin:2</b><br/><small>Table Operation</small><br/><sub>Create merchants table</sub>")]:::dbNode
        s3["<b>db_admin:3</b><br/><small>Milestone</small><br/><sub>Create index idx_customers_email on customers; Tag database milestone: v1.0-baseline</sub>"]:::defaultNode
        s9[("<b>customers</b><br/><small>Database Table</small><br/><sub>Customer profile and authentication records</sub>")]:::dbNode
        s10[("<b>merchants</b><br/><small>Database Table</small><br/><sub>Merchant store entities</sub>")]:::dbNode
    end

    subgraph lane_liquibase_customer_changeset_1 ["Liquibase (customer_changeset.1)"]
        s4["<b>me:dbbo_usp_GetCustomer.1</b><br/><small>Stored Procedure</small><br/><sub>Execute &lt;sqlFile&gt;: dbo_usp_GetCustomer.1.sql</sub>"]:::procNode
    end

    subgraph lane_liquibase_sql_changeset_1 ["Liquibase (sql_changeset.1)"]
        s5[("<b>architect:orders-v1</b><br/><small>Table Operation</small><br/><sub>Create orders table with foreign keys to customers and merchants</sub>")]:::dbNode
        s6[("<b>architect:payments-v1</b><br/><small>Table Operation</small><br/><sub>Create payments table with foreign key to orders</sub>")]:::dbNode
        s7["<b>architect:views-and-procs</b><br/><small>DatabaseView</small><br/><sub>Create analytics view for merchant billing</sub>"]:::defaultNode
        s8[("<b>architect:milestone-v1.1</b><br/><small>SQL Query</small><br/><sub>Tag database release milestone</sub>")]:::dbNode
        s11[("<b>orders</b><br/><small>Database Table</small><br/><sub>Database Table orders</sub>")]:::dbNode
        s12[("<b>payments</b><br/><small>Database Table</small><br/><sub>Database Table payments</sub>")]:::dbNode
    end

    subgraph lane_sql_dbo_usp_GetCustomer_1 ["SQL Database (dbo_usp_GetCustomer.1)"]
        s13["<b>Proc: usp_GetCustomer</b><br/><small>Stored Procedure</small><br/><sub>Stored Procedure Proc: usp_GetCustomer</sub>"]:::procNode
        s14[("<b>Query Customer</b><br/><small>SQL Query</small><br/><sub>Reads table Customer within stored procedure usp_GetCustomer</sub>")]:::dbNode
    end

    s1 -->|"executes next"| s2
    s2 -->|"executes next"| s3
    s3 -->|"executes next"| s4
    s4 -->|"executes next"| s5
    s5 -->|"executes next"| s6
    s6 -->|"executes next"| s7
    s7 -->|"executes next"| s8
    s11 -->|"references (fk_orders_customer)"| s9
    s11 -->|"references (fk_orders_merchant)"| s10
    s12 -->|"references (fk_payments_order)"| s11
    s13 -->|"queries"| s14
    s4 -.->|"calls <sqlFile>"| s13

    style lane_liquibase_core_changeset_1 fill:#F8FAFC,stroke:#E2E8F0,stroke-width:1.5px,rx:8px,ry:8px;
    style lane_liquibase_customer_changeset_1 fill:#F8FAFC,stroke:#E2E8F0,stroke-width:1.5px,rx:8px,ry:8px;
    style lane_liquibase_sql_changeset_1 fill:#F8FAFC,stroke:#E2E8F0,stroke-width:1.5px,rx:8px,ry:8px;
    style lane_sql_dbo_usp_GetCustomer_1 fill:#F8FAFC,stroke:#E2E8F0,stroke-width:1.5px,rx:8px,ry:8px;
```

---

## 3. Left-to-Right Flowchart (Flowchart LR)

Optimized for horizontal readability across complex multi-step pipelines.

- **Mermaid**: [liquibase_lr.mmd](liquibase_lr.mmd)
- **SVG**: [liquibase_lr.svg](liquibase_lr.svg)

---

## 4. Lifecycle Journey & State Diagrams

- **User/Pipeline Journey**: [liquibase_journey.mmd](liquibase_journey.mmd) | [SVG](liquibase_journey.svg)
- **Execution Sequence**: [liquibase_sequence.mmd](liquibase_sequence.mmd) | [SVG](liquibase_sequence.svg)
- **Database State Transition**: [liquibase_state.mmd](liquibase_state.mmd) | [SVG](liquibase_state.svg)

---

## 5. Structured Data and Visio Workbooks

- **Visio-Compatible Excel Workbook**: [liquibase.xlsx](liquibase.xlsx)
  - Contains step IDs, swimlanes, changeSet authors, labels, contexts, rollback scripts, and incoming/outgoing connectors ready for Visio Data Visualizer import.
- **Canonical ProcessModel JSON**: [liquibase.json](liquibase.json)
