# CodeFlow

**CodeFlow** is a multi-language static analysis and workflow extraction platform. It recursively scans multi-language repositories, parses source code Abstract Syntax Trees (ASTs) and workflow definitions into a unified canonical `ProcessModel`, correlates cross-service dependencies, optionally enriches technical steps with AI-generated business descriptions, and exports publication-ready diagrams in multiple formats.

```text
[CLI Flags] ---> [Discovery Engine] ---> [Language Analyzers] ---> [Correlation Engine] ---> [AI Layer] ---> [Exporters]
                       |                        |                         |                    |                 |
                - Path walker            - Go AST                 - Graph linking      - Gemini / OpenAI  - Mermaid (.mmd)
                - Gitignore              - C#/Py Tree-sitter      - Cross-lang maps    - Biz Translation  - Visio (.xlsx)
                - Ext router             - SQL & XML Parsers                                              - JSON & SVG
```

---

## Features

- **Multi-Language AST Analysis:**
  - **Go:** Routes (`http`, `gin`, `chi`, `mux`), gRPC endpoints, DB operations (`database/sql`, `gorm`), and function calls via `go/parser` and `go/ast`.
  - **C# & Razor/Blazor:** Controller endpoints (`[HttpGet]`, `[HttpPost]`), async tasks, Entity Framework queries, and `@page` Blazor components via Tree-sitter.
  - **Python:** FastAPI & Flask endpoints, call hierarchies, and ORM/DB operations via Tree-sitter.
  - **SQL:** Dialect-aware parsing (SQL Server, PostgreSQL, MySQL, SQLite, Oracle) for tables, views, stored procedures, and DDL/DML queries via `sqlparser` and regex dialect extractors.
  - **Flow XML, Pipelines & SSIS:** Native go-flow / ETL Madness pipeline XML (`<pipeline>`, `<preflight>`, `<flow>`, `<parallel>`, `<if>`, `<assert>`, `<script>`, `<sql>`), Custom workflow definitions, BPMN processes, and SSIS `.dtsx` packages via `encoding/xml`.
- **Cross-Service Correlation Engine:** Automatically matches HTTP calls and queries in code against declared routes, database tables, and stored procedures across services.
- **Intelligent Discovery & Directory Exclusion:**
  - Automatic recursive directory traversal with project `.gitignore` compliance.
  - Built-in ignoring of build artifacts, package managers, and IDE metadata (`.git`, `node_modules`, `vendor`, `bin`, `obj`, `__pycache__`, `.vscode`, `.idea`, `.gemini`).
  - Flexible directory exclusion via `-e, --exclude`, and `--exclude-dir` supporting folder names, comma-separated lists, and relative paths.
- **Multiple Diagram & Data Exporters:**
  - **Mermaid (`.mmd`):** Flowcharts with subgraphs representing architectural swimlanes.
  - **Microsoft Visio Data Visualizer (`.xlsx`):** Schema-compliant Excel sheets ready to open directly in Visio as automated cross-functional flowcharts.
  - **SVG (`.svg`):** Standalone, interactive, responsive vector diagrams with color-coded step types and connecting arrows.
  - **JSON (`.json`):** Canonical `ProcessModel` graph schema for automation and CI/CD pipelines.
- **Optional AI Enrichment Layer:** Automatically converts technical method names and AST structures into clear, business-readable workflow steps using Gemini or OpenAI.

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

CodeFlow provides a CLI command: `codeflow analyze`.

```text
Usage:
  codeflow analyze [flags]

Flags:
      --ai-enrichment         Enable AI enrichment for business-readable descriptions
      --custom-workbook       Export Excel Data Visualizer Schema (custom workbook) with interactive dashboard and swimlanes
  -d, --diagram-type string   Mermaid diagram type: td (Flowchart TD), lr (Flowchart LR), journey (User/System Journey), er (Entity-Relationship), all (default "td")
      --mermaid-type string   Alias for --diagram-type
  -e, --exclude strings       Comma-separated or repeated directory names/paths to exclude (e.g. -e tests,docs)
      --exclude-dir strings   Alias for --exclude
  -f, --format string         Comma-separated export formats: mermaid, markdown, journey, er, excel, custom-excel, json, svg (default "mermaid,excel,json")
  -h, --help                  Help for analyze
  -i, --ignore strings        Comma-separated or repeated file/directory patterns to ignore (e.g. -i '*_test.go', '*.spec.ts')
      --ignore-pattern string Alias for --ignore
      --exclude-pattern str   Alias for --ignore
      --ignore-tests          Ignore all standard unit test files across languages (*_test.go, test_*.py, *Test*.cs, *.spec.*)
  -o, --output string         Base path for output files without extension (default "output")
  -r, --recursive             Traverse directories recursively (default true)
  -s, --source string         Root directory to scan for code and workflows (default ".")
```

### Quick Examples

#### 1. Ignore Unit Tests and Spec Files by Pattern
You can ignore unit tests and non-production files using pattern matching or the `--ignore-tests` switch:

```bash
# Ignore Go unit tests (*_test.go)
.\bin\codeflow.exe analyze --source . --ignore "*_test.go"

# Ignore multiple patterns (Go unit tests, Python tests, TypeScript specs)
.\bin\codeflow.exe analyze --source . -i "*_test.go,test_*.py,*.spec.ts"

# Automatically ignore ALL standard test files across all supported languages:
.\bin\codeflow.exe analyze --source . --ignore-tests

# Combine directory exclusions and file patterns:
.\bin\codeflow.exe analyze --source ..\go-flow --exclude examples,db_importer,docs --ignore-tests
```

#### 2. Export User/System Journey or Entity-Relationship (ER) Diagrams
CodeFlow can export standard top-down flowcharts (`td`), horizontal flowcharts (`lr`), universal journey flowcharts (`journey`), sequence diagrams (`sequence`), state diagrams (`state`), and database/service entity-relationship diagrams (`er`):

```bash
# Export a User/System Journey Diagram (uses universal 'flowchart LR' with milestone stages)
.\bin\codeflow.exe analyze --source . --diagram-type journey

# Export an Entity-Relationship (ER) Diagram (uses 'erDiagram' header)
.\bin\codeflow.exe analyze --source . --diagram-type er

# Export a Sequence Diagram (uses 'sequenceDiagram' header)
.\bin\codeflow.exe analyze --source . --diagram-type sequence

# Export ALL diagram types at once:
.\bin\codeflow.exe analyze --source . --diagram-type all

# Export specific diagrams directly via --format:
.\bin\codeflow.exe analyze --source . --format mermaid,journey,er,sequence,markdown
```

Outputs generated:
- `output.mmd` & `output.svg` (Flowchart TD - vertical swimlane columns)
- `output_lr.mmd` & `output_lr.svg` (Flowchart LR - horizontal swimlane rows)
- `output_journey.mmd` & `output_journey.svg` (Journey - staged milestone progress cards and timeline)
- `output_sequence.mmd` & `output_sequence.svg` (Sequence Diagram - participant lifelines and numbered call interactions)
- `output_er.mmd` & `output_er.svg` (Entity-Relationship Diagram - entity table boxes, attribute lists, and relationships)
- `output_state.mmd` & `output_state.svg` (State Diagram - UML state machine with start/end states and transition badges)

#### 3. Custom Output Directory & File Naming
By default, all files use the base prefix `output` (e.g. `output_sequence.mmd`, `output.json`). You can customize the destination directory and replace `output` with your own pipeline or project name:

```bash
# Save all outputs into a dedicated directory with a custom file name
.\bin\codeflow.exe analyze --source ..\go-flow\examples\gcloud_billing.xml \
  --output-dir .\diagrams\billing \
  --name gcloud_billing \
  --format mermaid,svg,excel,json \
  --diagram-type all

# Shorthand with -n
.\bin\codeflow.exe analyze --source . --output-dir ./docs/architecture -n payment_service --diagram-type sequence
```

Outputs generated in `.\diagrams\billing\`:
- `gcloud_billing.mmd` & `gcloud_billing.svg` (Flowchart TD)
- `gcloud_billing_lr.mmd` & `gcloud_billing_lr.svg` (Flowchart LR)
- `gcloud_billing_journey.mmd` & `gcloud_billing_journey.svg` (Journey)
- `gcloud_billing_sequence.mmd` & `gcloud_billing_sequence.svg` (Sequence Diagram)
- `gcloud_billing_state.mmd` & `gcloud_billing_state.svg` (State Diagram)
- `gcloud_billing_er.mmd` & `gcloud_billing_er.svg` (ER Diagram)
- `gcloud_billing.xlsx` (Excel workbook)
- `gcloud_billing.json` (JSON model)

*Supported flags and aliases:*
- **Directory:** `--output-dir`, `--out-dir`, `--outdir` (created automatically if missing)
- **Base file name:** `-n`, `--name`, `--output-name`, `--filename`, `--base-name`

#### 4. Export Excel Data Visualizer Schema (Custom Workbook)
Because the Microsoft Visio Data Visualizer add-in is retired / end-of-life, export the self-contained custom workbook that requires **no add-ins and no Visio license**:

```bash
# Windows
.\bin\codeflow.exe analyze --source . --custom-workbook --output output

# OR specify custom-excel, markdown, and json in formats:
.\bin\codeflow.exe analyze --source . --format custom-excel,mermaid,markdown,json --output output
```

Output generated:
- `output.xlsx` (Multi-sheet interactive workbook with Dashboard, Visual Process Map, Process Steps, Traceability Matrix, and Services)
- `output.mmd` (Mermaid flowchart)
- `output.md` (Markdown document with embedded Mermaid block and maxTextSize init directive)
- `output.json` (Canonical AST ProcessModel graph)

#### 5. Exclude Specific Directories
Exclude test suites, third-party libraries, build output, or documentation directories from analysis:

```bash
# Comma-separated directory names
.\bin\codeflow.exe analyze --source . --exclude tests,docs,scratch,dist

# Or repeated flags / relative path
.\bin\codeflow.exe analyze --source . -e tests -e pkg/legacy --format custom-excel
```

#### 6. Analyze Current Repository (Standard Formats)
```bash
.\bin\codeflow.exe analyze --source . --recursive --format mermaid,excel,json
```

#### 7. Analyze a Single Source File
You don't need to scan an entire repository; you can target an individual file directly:
```bash
# Analyze a single Go file
.\bin\codeflow.exe analyze --source .\pkg\discovery\scanner.go

# Analyze a single SQL migration or schema
.\bin\codeflow.exe analyze --source .\db\migrations\001_orders.sql --diagram-type er

# Analyze a single C# service or controller
.\bin\codeflow.exe analyze --source .\Controllers\PaymentController.cs
```

---

### Excluding Directories & Ignoring File Patterns

CodeFlow offers several layers of filtering to ensure your architecture diagrams focus solely on production workflows:

#### 1. File Pattern Matching (`-i`, `--ignore`, `--ignore-tests`)
- **Ignore Unit Tests Switch:** `--ignore-tests` automatically filters out test files across all supported languages:
  - **Go:** `*_test.go`
  - **Python:** `test_*.py`, `*_test.py`
  - **C#:** `*Test*.cs`, `*Tests*.cs`, `*.Test.cs`, `*.Tests.cs`
  - **JavaScript / TypeScript:** `*.test.js`, `*.spec.js`, `*.test.ts`, `*.spec.ts`
- **Custom Patterns:** Pass custom wildcards with `-i` or `--ignore`:
  - Filename wildcards: `-i "*_test.go"`, `-i "*.spec.*"`, `-i "*mock*"`
  - Path wildcards: `-i "builder/*_test.go"`, `-i "**/fixtures/**"`
  - Comma-separated: `-i "*_test.go,*.gen.go,*.pb.go"`

#### 2. Directory Exclusion Flags (`-e`, `--exclude`, `--exclude-dir`)
- **Flag names:** `-e`, `--exclude`, and `--exclude-dir` (all equivalent).
- **Comma-separated values:** `--exclude tests,docs,scripts`
- **Repeated flags:** `-e tests -e docs -e pkg/internal/mock`
- **Folder names vs. Paths:**
  - Specifying a folder name (e.g. `tests`) excludes *any* directory named `tests` throughout the project tree.
  - Specifying a relative path (e.g. `pkg/legacy` or `src/fixtures`) excludes only that specific subfolder.
- **Fast pruning:** The scanner skips traversing into excluded directories entirely (`filepath.SkipDir`), making scans significantly faster on large repositories.

#### 3. Automatic Default Exclusions
By default, CodeFlow automatically ignores common non-production folders without requiring any flags:
- **Version Control:** `.git`, `.svn`, `.hg`
- **Package Managers & Dependencies:** `node_modules`, `vendor`
- **Build Outputs:** `bin`, `obj`
- **Python Caches:** `__pycache__`, `*.pyc`
- **IDEs & Agents:** `.vscode`, `.idea`, `.gemini`

#### 4. Project `.gitignore` Integration
CodeFlow automatically detects and respects any `.gitignore` file located at the root of your target source directory. Any pattern declared in `.gitignore` is honored during discovery.

---

## Excel Data Visualizer Guide

Due to the Microsoft Visio Data Visualizer add-in reaching **End of Life (EOL)**, CodeFlow provides an **Excel Data Visualizer Schema (custom workbook)** that functions completely standalone directly in Microsoft Excel, Google Sheets, or LibreOffice without needing Visio or any add-in.

---

### Option A: Excel Data Visualizer Schema (Custom Workbook) [Recommended]

Exported with `--custom-workbook` or `--format custom-excel`, this workbook provides an in-Excel interactive visual workflow viewer across 5 dedicated sheets:

1. **Dashboard & KPIs:**
   - Process banner with total steps, total services/swimlanes, cross-service connection counts, and internal calls.
   - Breakdown charts/tables by source language (Go, C#, Python, SQL, XML) and step types (Endpoints, Queries, Tasks).
2. **Visual Process Map (In-Excel Swimlanes):**
   - Architectural swimlane sections organized by service, API gateway, or database.
   - Color-coded step cards (Blue for Endpoints, Green for Databases, Purple for Stored Procedures/Views, Amber for Tasks) with descriptions, source file line numbers, and downstream connection badges.
3. **Process Steps (Data Visualizer Schema):**
   - Standard Microsoft Data Visualizer schema table (`Process Step ID`, `Step Description`, `Next Step ID`, `Connector Label`, `Step Type`, `Owner / Function`, `Language`, `Source File`, `Line Number`).
   - Pre-formatted with Excel sorting, filtering, and wrap-text styling.
4. **Traceability Matrix:**
   - Comprehensive source-to-target dependency mapping across all components.
   - Visual badges distinguishing **`CROSS-SERVICE`** integration boundaries from **`INTERNAL`** service calls.
5. **Services & Swimlanes:**
   - Service catalog listing every architectural component, total steps, incoming calls, and outgoing calls.

---

### Option B: Import into Microsoft Visio (Desktop & Web)

If you have a standalone Microsoft Visio license and wish to use the desktop Data Visualizer wizard:

1. **Launch Visio:** Open Microsoft Visio Desktop or Visio Web.
2. **Start the Data Visualizer Wizard:** Go to **File** > **New** > **Templates** > **Flowchart** > **Data Visualizer - Cross-Functional Flowchart** (*Create from Data*).
3. **Select the Workbook:** Browse to `output.xlsx`.
4. **Confirm Field Mappings:** Visio automatically maps the standard headers (`Process Step ID`, `Step Description`, `Next Step ID`, `Connector Label`, `Step Type`, and `Owner / Function`).
5. **Finish:** Click **Finish**. Visio will arrange all swimlanes, render styled cards, draw arrows between dependencies, and place labels on cross-service interactions.

---

### Excel Data Visualizer Schema Reference

The generated Excel workbook contains the following schema:

| Column | Header | Description | Example Value |
| :--- | :--- | :--- | :--- |
| **A** | `Process Step ID` | Unique ID of the code step or workflow activity | `order.go:CreateOrder:24` |
| **B** | `Step Description` | Business or technical description displayed inside the shape | `Create customer order and register transaction` |
| **C** | `Next Step ID` | Target step ID(s). Comma-separated for multiple downstream branches | `queries.sql:stmt:1, payment.go:Pay:12` |
| **D** | `Connector Label` | Text label drawn on the connector line(s) | `accesses table orders, invokes payment` |
| **E** | `Step Type` | Shape category (`Endpoint`, `DatabaseQuery`, `TableOperation`, `Task`, `Function`, `View`) | `Endpoint` *(renders as Start/Terminal or Process shape)* |
| **F** | `Owner / Function` | Container swimlane (service, controller class, or DB schema) | `Go Service (order)`, `SQL Database (schema)` |

> [!TIP]
> **Branching & Decisions:** If a step connects to multiple subsequent steps, CodeFlow lists them separated by commas in `Next Step ID` (e.g., `StepA, StepB`) with matching comma-separated labels in `Connector Label` (e.g., `On Success, On Failure`). Visio automatically splits these into multiple outgoing arrows!

---

## How to Setup with AI (Enrichment Layer)

The AI Enrichment Layer translates technical source code elements (such as `_context.Orders.AddAsync` or `ProcessPayment`) into business-level workflow descriptions (such as *"Process customer payment transaction and verify billing information"*).

AI enrichment is **completely optional** and controlled by the `--ai-enrichment` flag.

### Supported Providers

CodeFlow natively supports:
1. **Google Gemini API** (recommended, fast & cost-efficient via `gemini-1.5-flash`)
2. **OpenAI API** (`gpt-4o-mini`)
3. **Built-in Semantic Fallback:** If `--ai-enrichment` is enabled without an API key, CodeFlow uses an internal rule-based semantic translation engine so execution never fails.

### Setting API Keys

Set either `GEMINI_API_KEY` or `OPENAI_API_KEY` in your environment before running the tool.

#### Windows (PowerShell):
```powershell
# For Google Gemini
$env:GEMINI_API_KEY = "your-gemini-api-key-here"

# OR for OpenAI
$env:OPENAI_API_KEY = "your-openai-api-key-here"
```

#### Windows (Command Prompt):
```cmd
set GEMINI_API_KEY=your-gemini-api-key-here
rem OR
set OPENAI_API_KEY=your-openai-api-key-here
```

#### Linux / macOS (Bash / Zsh):
```bash
# For Google Gemini
export GEMINI_API_KEY="your-gemini-api-key-here"

# OR for OpenAI
export OPENAI_API_KEY="your-openai-api-key-here"
```

### Running with AI Enrichment Enabled

Pass the `--ai-enrichment` flag:

```bash
.\bin\codeflow.exe analyze --source . --recursive --format mermaid,excel,json,svg --ai-enrichment
```

Sample output:
```text
[*] Scanning repository at: . (recursive: true)
[+] Discovered 30 relevant source and workflow files
[*] Running AI Enrichment layer...
[+] AI Enrichment completed successfully
[+] Process model assembled: 16 swimlanes, 101 steps, 42 links
[+] Generated Mermaid diagram: output.mmd
[+] Generated Visio Excel workbook: output.xlsx
[+] Generated JSON: output.json
[+] Generated SVG diagram: output.svg
```

---

## Canonical Model Schema (`ProcessModel`)

Exported JSON files adhere to the canonical schema:

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
| **Go** | `.go` | `go/parser`, `go/ast` | HTTP handlers & routes, gRPC services, SQL queries (`Exec`/`Query`/`GORM`), internal function calls |
| **C#** | `.cs` | Tree-sitter C# | ASP.NET Controllers, HTTP verbs (`[HttpGet]`, `[HttpPost]`), async tasks, Entity Framework queries |
| **Razor / Blazor** | `.razor` | Regex + Tree-sitter C# | `@page` routes, `@inject` dependencies, `@code` component logic |
| **Python** | `.py` | Tree-sitter Python | FastAPI & Flask endpoints, call chains, DB sessions & queries |
| **SQL** | `.sql` | `sqlparser` + dialect regex | Tables, Views, Stored Procedures, DDL & DML operations |
| **Flow XML & SSIS**| `.xml`, `.dtsx` | `encoding/xml` | Workflow steps, transitions, BPMN tasks, SSIS Executables & constraints |

---

## License

This project is licensed under the [MIT License](LICENSE).
