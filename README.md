# CodeFlow

**CodeFlow** is a multi-language static analysis, workflow extraction, and architecture visualization platform. It recursively scans codebases, parses Abstract Syntax Trees (ASTs) and workflow definitions into a unified canonical `ProcessModel`, correlates cross-service dependencies and data flows, optionally enriches technical steps with AI-generated business descriptions, and exports publication-ready diagrams across multiple formats.

```text
[CLI Flags] ---> [Discovery Engine] ---> [Language Analyzers] ---> [Correlation Engine] ---> [AI Layer] ---> [Exporters]
                       |                        |                         |                    |                 |
                - Path walker            - Go AST & Structs       - Graph linking      - Gemini / OpenAI  - Mermaid (.mmd)
                - Gitignore & Bypass     - C#/Py Tree-sitter      - Cross-package maps - Biz Translation  - Standalone SVG
                - Ext router             - T-SQL & XML Parsers    - Route & DB calls                      - Visio (.xlsx)
                - Test filters           - SSIS .dtsx Packages                                            - JSON Schema
```

---

## Features

- **Multi-Language AST & Schema Analysis:**
  - **Go:** HTTP routes (`http`, `gin`, `chi`, `mux`), gRPC endpoints, database queries (`database/sql`, `gorm`), struct data models (fields, types, and primary keys for ER modeling), docstrings for rich descriptions, method receivers (`Receiver.Method`), and cross-package call correlation via `go/parser` and `go/ast`.
  - **C# & Razor/Blazor:** Controller endpoints (`[HttpGet]`, `[HttpPost]`, `[Route]`), async tasks, Entity Framework queries, and `@page` Blazor components via Tree-sitter.
  - **Python:** FastAPI & Flask endpoints, call hierarchies, and ORM/DB operations via Tree-sitter.
  - **SQL:** Strict DDL parsing and dialect-aware extraction (SQL Server / T-SQL, PostgreSQL, MySQL, SQLite, Oracle) for tables, views, stored procedures, temporary tables (`#table`, `##table`), bracketed schema objects (`[schema].[table]`), and variable-length types (`varchar(max)`, `sysname`).
  - **Flow XML, Pipelines & SSIS:** Native go-flow / ETL Madness pipeline XML (`<pipeline>`, `<preflight>`, `<flow>`, `<parallel>`, `<if>`, `<assert>`, `<script>`, `<sql>`), Custom workflow definitions, BPMN processes, and SQL Server Integration Services (SSIS) `.dtsx` packages (Data Flow Tasks, Execute SQL Tasks, Precedence Constraints, Connection Managers) via `encoding/xml`.
- **Cross-Service Correlation Engine:** Automatically matches HTTP calls, database queries, stored procedure executions, and cross-package function/method invocations across disparate services and swimlanes.
- **6 Supported Diagram Archetypes:**
  - **Flowchart TD (`td`):** Top-to-bottom flowchart with architectural swimlane groupings.
  - **Flowchart LR (`lr`):** Left-to-right horizontal pipeline flowchart.
  - **Journey (`journey`):** User/System staged milestone journey mapping out sequential progress.
  - **Entity-Relationship (`er`):** Data entity boxes with attributes, types, and primary key (`PK`) badges.
  - **Sequence Diagram (`sequence`):** Participant lifelines with numbered cross-service interactions.
  - **State Diagram (`state`):** UML state machine depicting execution states and transition badges.
- **Multiple Publication-Ready Exporters:**
  - **Mermaid (`.mmd`, `.md`):** Clean Mermaid diagrams with unified styling, subgraph swimlanes, and `maxTextSize` support.
  - **Standalone SVG (`.svg`):** Vector diagrams with modern typography, colored step badges, shadow filters, responsive viewports, and multiline word-wrapping.
  - **Microsoft Visio Data Visualizer (`.xlsx`):** Schema-compliant Excel sheets ready to open directly in Visio as automated cross-functional flowcharts.
  - **Custom Interactive Excel Workbook:** Self-contained multi-sheet workbook with Dashboard KPIs, Visual Process Map, Step Registry, and Traceability Matrix (requires **no add-in and no Visio license**).
  - **Canonical JSON (`.json`):** ProcessModel graph schema for automation, indexing, and CI/CD pipelines.
- **Discovery Engine & Filtering:**
  - Recursive directory traversal with root `.gitignore` compliance.
  - `--no-gitignore` flag to bypass ignore rules when scanning specific internal or vendored directories.
  - Built-in ignoring of build artifacts (`.git`, `node_modules`, `vendor`, `bin`, `obj`, `__pycache__`, `.vscode`, `.idea`, `.gemini`).
  - One-switch unit test exclusion via `--ignore-tests` across Go, Python, C#, and TypeScript.
  - Flexible pattern matching (`-i`, `--ignore`) and folder exclusions (`-e`, `--exclude`).
- **Optional AI Enrichment Layer:** Automatically converts technical method names and AST structures into clear, business-readable workflow steps using Google Gemini or OpenAI.

---

## Prerequisites

- **Go:** Version 1.22 or higher (`go version`)
- **C Compiler (CGO):** Required for Tree-sitter C bindings.
  - **Windows:** MSYS2 MinGW-w64 (`gcc`), TDM-GCC, or Visual Studio C++ tools.
  - **Linux:** GCC (`sudo apt-get install build-essential`)
  - **macOS:** Xcode Command Line Tools (`xcode-select --install`)

---

## How to Build

### 1. Clone & Download Dependencies

```bash
git clone https://github.com/etl-madness/CodeFlow.git
cd CodeFlow
go mod download
```

### 2. Compile Binary

#### Windows (PowerShell / CMD):
```powershell
go build -o bin/codeflow.exe ./cmd/codeflow
```

#### Linux / macOS (Bash / Zsh):
```bash
go build -o bin/codeflow ./cmd/codeflow
chmod +x bin/codeflow
```

### 3. Run Verification Tests

Run the full suite of unit tests with coverage:

```bash
go test -v -cover ./pkg/...
```

---

## How to Use

CodeFlow provides a unified CLI command: `codeflow analyze`.

```text
Usage:
  codeflow analyze [flags]

Flags:
      --ai-enrichment             Enable AI enrichment for business-readable descriptions
      --base-name string          Alias for --name
      --custom-workbook           Export Excel Data Visualizer Schema (custom workbook) with interactive dashboard and swimlanes
  -d, --diagram-type string       Mermaid diagram type: td (Flowchart TD), lr (Flowchart LR), journey (User/System Journey), er (Entity-Relationship), all (default "td")
  -e, --exclude strings           Comma-separated or repeated directory names/paths to exclude (e.g. -e tests,docs)
      --exclude-dir strings       Alias for --exclude
      --exclude-pattern strings   Alias for --ignore
      --filename string           Alias for --name
  -f, --format string             Comma-separated export formats: mermaid, markdown, journey, er, excel, custom-excel, json, svg (default "mermaid,excel,json")
      --full-text                 Alias for --no-truncate
  -h, --help                      help for analyze
  -i, --ignore strings            Comma-separated or repeated file/directory patterns to ignore (e.g. -i '*_test.go', '*.spec.ts')
      --ignore-pattern strings    Alias for --ignore
      --ignore-tests              Ignore all standard unit test files across languages (*_test.go, test_*.py, *Test*.cs, *.spec.*)
      --mermaid-type string       Alias for --diagram-type (default "td")
  -n, --name string               Base name for output files (e.g. 'myflow' generates myflow_sequence.mmd, myflow.json, etc.)
      --no-gitignore              Do not read or apply root .gitignore file during scanning
      --no-truncate               Preserve full node title and description length without truncation in diagrams
      --out-dir string            Alias for --output-dir
      --outdir string             Alias for --output-dir
  -o, --output string             Base path or filename for output files (without extension) (default "output")
      --output-dir string         Directory to save output files (created if it does not exist)
      --output-name string        Alias for --name
  -r, --recursive                 Traverse directories recursively (default true)
  -s, --source string             Root directory or single source file to scan (default ".")
```

---

## Command Examples

### 1. Generate All Diagram Types (Mermaid, SVG, Excel, JSON)

Generate all 6 diagram archetypes simultaneously across Mermaid and SVG:

```bash
.\bin\codeflow.exe analyze --source . \
  --recursive \
  --format mermaid,svg,excel,json \
  -n codeflow_arch \
  --diagram-type all \
  --output-dir ./docs/architecture \
  --no-truncate \
  --ignore-tests
```

**Files produced in `./docs/architecture/`:**
| File | Format | Diagram Archetype | Description |
| :--- | :--- | :--- | :--- |
| `codeflow_arch.mmd` | Mermaid | Flowchart TD | Vertical architectural swimlanes |
| `codeflow_arch.svg` | SVG | Flowchart TD | Standalone vector flowchart (top-to-bottom) |
| `codeflow_arch_lr.mmd` | Mermaid | Flowchart LR | Horizontal pipeline swimlanes |
| `codeflow_arch_lr.svg` | SVG | Flowchart LR | Standalone horizontal vector flowchart |
| `codeflow_arch_journey.mmd` | Mermaid | Journey | Milestone-based staged journey diagram |
| `codeflow_arch_journey.svg` | SVG | Journey | Standalone staged timeline vector diagram |
| `codeflow_arch_er.mmd` | Mermaid | ER Diagram | Entity-relationship data model (`erDiagram`) |
| `codeflow_arch_er.svg` | SVG | ER Diagram | Standalone vector entity schema with attributes and PKs |
| `codeflow_arch_sequence.mmd` | Mermaid | Sequence | Cross-service sequence interaction lifelines |
| `codeflow_arch_sequence.svg` | SVG | Sequence | Standalone vector sequence diagram with lifelines |
| `codeflow_arch_state.mmd` | Mermaid | State | UML state machine with transitions |
| `codeflow_arch_state.svg` | SVG | State | Standalone vector state diagram |
| `codeflow_arch.xlsx` | Excel | Visio / Schema | Cross-functional flowchart data visualizer workbook |
| `codeflow_arch.json` | JSON | Model Schema | Canonical `ProcessModel` graph representation |

---

### 2. Full-Text & Word Wrapping (`--no-truncate`)

By default, diagram nodes truncate long method names and descriptions to keep shapes compact. Pass `--no-truncate` (or `--full-text`) to preserve complete descriptions:

```bash
.\bin\codeflow.exe analyze --source . --format mermaid,svg -n pipeline --no-truncate
```

- **In SVG:** Node cards and swimlanes dynamically widen, and descriptions automatically word-wrap across multiple lines inside shapes without overflowing borders.
- **In Mermaid:** Node definitions retain complete function signatures and docstrings.

---

### 3. Bypass `.gitignore` Rules (`--no-gitignore`)

If a repository's `.gitignore` contains broad rules (e.g. `/pkg/` from legacy Go templates, or generated asset folders) that you still want to include in architectural analysis, pass `--no-gitignore`:

```bash
.\bin\codeflow.exe analyze --source . --no-gitignore --recursive -n full_codebase
```

---

### 4. Ignore Unit Tests & Non-Production Code

Filter out test suites, mocks, and fixtures using `--ignore-tests` or custom glob patterns:

```bash
# Automatically ignore test files across Go, Python, C#, and TypeScript:
.\bin\codeflow.exe analyze --source . --ignore-tests

# Filter by custom glob patterns:
.\bin\codeflow.exe analyze --source . -i "*_test.go,test_*.py,*.spec.ts,*mock*"

# Combine directory exclusions and file patterns:
.\bin\codeflow.exe analyze --source . --exclude tests,docs,fixtures --ignore-tests
```

---

### 5. Custom Output Directory & File Naming

Customize where artifacts are stored and how they are named:

```bash
# Save into docs/billing/ named 'gcloud_billing.*'
.\bin\codeflow.exe analyze --source ..\ETL_Pipelines\billing.xml \
  --output-dir ./docs/billing \
  --name gcloud_billing \
  --format mermaid,svg,excel,json \
  --diagram-type all
```

*Supported aliases:*
- **Directory:** `--output-dir`, `--out-dir`, `--outdir` (created automatically if missing)
- **File Base Name:** `-n`, `--name`, `--output-name`, `--filename`, `--base-name`

---

### 6. Analyze SQL Schemas, Stored Procedures & Temp Tables

CodeFlow parses complex SQL scripts, including T-SQL temporary tables and bracketed identifiers:

```bash
.\bin\codeflow.exe analyze --source .\db\migrations \
  --recursive \
  --format mermaid,svg \
  --diagram-type er \
  -n database_schema \
  --output-dir ./docs/sql
```

CodeFlow extracts `CREATE TABLE`, `#temp_tables`, `CREATE PROCEDURE`, `CREATE VIEW`, `INSERT INTO`, and `SELECT FROM` queries into correlated tables and data lineage flows.

---

### 7. Analyze SSIS Packages (`.dtsx`)

Analyze SQL Server Integration Services packages to extract task hierarchies, precedence constraints, and connection managers:

```bash
.\bin\codeflow.exe analyze --source .\SSIS\ETL_Package.dtsx \
  --format mermaid,svg,excel,json \
  -n ssis_flow \
  --diagram-type all \
  --output-dir ./docs/ssis
```

---

### 8. Export Excel Data Visualizer Schema (Custom Workbook)

Export an interactive, self-contained multi-sheet Excel workbook that requires **no add-ins and no Visio license**:

```bash
.\bin\codeflow.exe analyze --source . --custom-workbook -n architecture
# OR
.\bin\codeflow.exe analyze --source . --format custom-excel,mermaid,json -n architecture
```

**Workbook Sheets:**
1. **Dashboard & KPIs:** Process metrics, step type breakdowns, language distribution, and cross-service integration statistics.
2. **Visual Process Map:** In-Excel swimlane layout with color-coded step cards and connection badges.
3. **Process Steps:** Data Visualizer schema table (`Process Step ID`, `Step Description`, `Next Step ID`, `Connector Label`, `Step Type`, `Owner / Function`).
4. **Traceability Matrix:** Source-to-target dependency mapping identifying **`CROSS-SERVICE`** vs. **`INTERNAL`** boundaries.
5. **Services & Swimlanes:** Catalog of all architectural components and call volumes.

---

### 9. Import into Microsoft Visio (Desktop & Web)

If you have a Microsoft Visio license and wish to use the native Data Visualizer wizard:

1. Open Microsoft Visio Desktop or Visio Web.
2. Select **File** > **New** > **Templates** > **Flowchart** > **Data Visualizer - Cross-Functional Flowchart** (*Create from Data*).
3. Browse to the generated `output.xlsx` (or `<name>.xlsx`).
4. Visio will automatically map the standard column headers (`Process Step ID`, `Step Description`, `Next Step ID`, `Connector Label`, `Step Type`, and `Owner / Function`).
5. Click **Finish**. Visio will draw all swimlanes, render styled cards, and connect dependencies with directional arrows.

---

## AI Enrichment Layer (Optional)

The AI Enrichment Layer translates technical source code symbols (such as `_context.Orders.AddAsync` or `ProcessPayment`) into human-readable business descriptions (such as *"Process customer payment transaction and verify billing information"*).

AI enrichment is **completely optional** and controlled by the `--ai-enrichment` flag.

### Supported Providers
1. **Google Gemini API** (recommended, fast & cost-efficient via `gemini-1.5-flash`)
2. **OpenAI API** (`gpt-4o-mini`)
3. **Built-in Semantic Fallback:** If `--ai-enrichment` is enabled without an API key, CodeFlow uses a local rule-based semantic translation engine so execution never fails.

### Setting API Keys

#### Windows (PowerShell):
```powershell
# For Google Gemini
$env:GEMINI_API_KEY = "your-gemini-api-key-here"

# OR for OpenAI
$env:OPENAI_API_KEY = "your-openai-api-key-here"
```

#### Linux / macOS (Bash / Zsh):
```bash
export GEMINI_API_KEY="your-gemini-api-key-here"
# OR
export OPENAI_API_KEY="your-openai-api-key-here"
```

### Running with AI Enrichment
```bash
.\bin\codeflow.exe analyze --source . --ai-enrichment --format mermaid,svg,excel
```

---

## Canonical Model Schema (`ProcessModel`)

The exported JSON file represents the full AST dependency graph:

```json
{
  "id": "process-CodeFlow",
  "name": "CodeFlow Workflow",
  "swimlanes": [
    {
      "id": "go:order",
      "name": "Go Service (order)",
      "description": "Go package order in order.go"
    }
  ],
  "steps": [
    {
      "id": "order.go:CreateOrder:24",
      "swimlane_id": "go:order",
      "name": "CreateOrder",
      "description": "Create new customer order and register initial purchase transaction",
      "type": "Endpoint",
      "language": "go",
      "source_file": "order.go",
      "line_number": 24,
      "metadata": {
        "route": "/api/v1/orders",
        "method": "POST"
      }
    }
  ],
  "links": [
    {
      "id": "order.go:CreateOrder:24->queries.sql:stmt:1",
      "source_step_id": "order.go:CreateOrder:24",
      "target_step_id": "queries.sql:stmt:1",
      "label": "accesses table orders",
      "is_cross_service": true
    }
  ]
}
```

---

## Supported File Types & Analyzers

| Language / Format | Extensions | Primary Parser | Key Extracted Entities |
| :--- | :--- | :--- | :--- |
| **Go** | `.go` | `go/parser`, `go/ast` | HTTP handlers & routes, gRPC services, SQL queries (`Exec`/`Query`/`GORM`), struct schemas (fields, types, PKs), method receivers, docstrings, cross-package call linking |
| **C#** | `.cs` | Tree-sitter C# | ASP.NET Controllers, HTTP verbs (`[HttpGet]`, `[HttpPost]`), async tasks, Entity Framework queries |
| **Razor / Blazor** | `.razor` | Regex + Tree-sitter C# | `@page` routes, `@inject` dependencies, `@code` component logic |
| **Python** | `.py` | Tree-sitter Python | FastAPI & Flask endpoints, call chains, DB sessions & queries |
| **SQL** | `.sql` | `sqlparser` + dialect regex | Tables, Views, Stored Procedures, T-SQL temporary tables (`#table`, `##table`), bracketed schemas, DDL & DML operations |
| **Flow XML & Pipelines** | `.xml` | `encoding/xml` | Pipeline tasks (`<flow>`, `<parallel>`, `<if>`, `<assert>`, `<script>`, `<sql>`), transitions, BPMN processes |
| **SSIS Packages** | `.dtsx` | `encoding/xml` | Data Flow Tasks, Execute SQL Tasks, Precedence Constraints, Connection Managers, Variables |

---

## License

This project is licensed under the [MIT License](LICENSE).
