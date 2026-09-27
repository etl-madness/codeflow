package ssis

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

// Analyzer implements static analysis for SQL Server Integration Services (SSIS) .dtsx packages.
type Analyzer struct{}

// New creates a new SSIS Analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "ssis"
}

// CanAnalyze returns true for .dtsx and SSIS .xml files.
func (a *Analyzer) CanAnalyze(ext string) bool {
	l := strings.ToLower(ext)
	return l == ".dtsx" || l == ".xml"
}

// dtsxNode represents a node in the XML hierarchy with normalized attribute lookups.
type dtsxNode struct {
	Tag      string
	Attrs    map[string]string
	Content  string
	Line     int
	Children []*dtsxNode
}

func (n *dtsxNode) Attr(name string) string {
	lowerName := strings.ToLower(name)
	// Direct match
	if v, ok := n.Attrs[lowerName]; ok {
		return v
	}
	// Match stripping any prefix before colon
	for k, v := range n.Attrs {
		if idx := strings.Index(k, ":"); idx != -1 {
			if k[idx+1:] == lowerName {
				return v
			}
		}
	}
	return ""
}

func (n *dtsxNode) FindChild(tag string) *dtsxNode {
	lowerTag := strings.ToLower(tag)
	for _, c := range n.Children {
		cleanTag := c.Tag
		if idx := strings.Index(cleanTag, ":"); idx != -1 {
			cleanTag = cleanTag[idx+1:]
		}
		if cleanTag == lowerTag {
			return c
		}
	}
	return nil
}

func (n *dtsxNode) FindChildren(tag string) []*dtsxNode {
	var result []*dtsxNode
	lowerTag := strings.ToLower(tag)
	for _, c := range n.Children {
		cleanTag := c.Tag
		if idx := strings.Index(cleanTag, ":"); idx != -1 {
			cleanTag = cleanTag[idx+1:]
		}
		if cleanTag == lowerTag {
			result = append(result, c)
		}
	}
	return result
}

func (n *dtsxNode) FindDescendant(tag string) *dtsxNode {
	lowerTag := strings.ToLower(tag)
	for _, c := range n.Children {
		cleanTag := c.Tag
		if idx := strings.Index(cleanTag, ":"); idx != -1 {
			cleanTag = cleanTag[idx+1:]
		}
		if cleanTag == lowerTag {
			return c
		}
		if d := c.FindDescendant(tag); d != nil {
			return d
		}
	}
	return nil
}

func (n *dtsxNode) FindDescendants(tag string) []*dtsxNode {
	var result []*dtsxNode
	lowerTag := strings.ToLower(tag)
	for _, c := range n.Children {
		cleanTag := c.Tag
		if idx := strings.Index(cleanTag, ":"); idx != -1 {
			cleanTag = cleanTag[idx+1:]
		}
		if cleanTag == lowerTag {
			result = append(result, c)
		}
		result = append(result, c.FindDescendants(tag)...)
	}
	return result
}

// connInfo stores parsed metadata about a connection manager.
type connInfo struct {
	ID               string
	RefID            string
	Name             string
	CreationName     string
	ConnectionString string
	Database         string
	Server           string
	Resource         string
}

// AnalyzeFile unmarshals SSIS package XML definitions into canonical ProcessModel elements.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	rootNode, err := parseDTSXTree(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSIS XML in %s: %w", path, err)
	}

	fileName := filepath.Base(path)
	packageName := rootNode.Attr("ObjectName")
	if packageName == "" {
		packageName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}

	// 1. Parse Variables
	variables := make(map[string]string)
	for _, varsNode := range rootNode.FindDescendants("Variables") {
		for _, varNode := range varsNode.FindChildren("Variable") {
			varName := varNode.Attr("ObjectName")
			ns := varNode.Attr("Namespace")
			val := ""
			if valNode := varNode.FindDescendant("VariableValue"); valNode != nil {
				val = valNode.Content
			}
			if val == "" {
				val = varNode.Attr("Expression")
			}
			if varName != "" {
				if ns != "" {
					variables[fmt.Sprintf("%s::%s", ns, varName)] = val
					variables[fmt.Sprintf("@[%s::%s]", ns, varName)] = val
				}
				variables[varName] = val
				variables["@["+varName+"]"] = val
			}
		}
	}

	// 2. Parse Connection Managers
	connections := make(map[string]*connInfo)
	for _, cmsNode := range rootNode.FindDescendants("ConnectionManagers") {
		for _, cmNode := range cmsNode.FindChildren("ConnectionManager") {
			ci := parseConnectionManager(cmNode, variables)
			if ci != nil {
				if ci.ID != "" {
					connections[strings.ToLower(ci.ID)] = ci
					connections[strings.ToLower(strings.Trim(ci.ID, "{}"))] = ci
				}
				if ci.RefID != "" {
					connections[strings.ToLower(ci.RefID)] = ci
				}
				if ci.Name != "" {
					connections[strings.ToLower(ci.Name)] = ci
				}
			}
		}
	}

	// Helper to resolve connection
	resolveConnection := func(connRef string) *connInfo {
		if connRef == "" {
			return nil
		}
		cleaned := strings.ToLower(strings.TrimSpace(connRef))
		if ci, ok := connections[cleaned]; ok {
			return ci
		}
		cleaned = strings.Trim(cleaned, "{}")
		if ci, ok := connections[cleaned]; ok {
			return ci
		}
		// Search partial match
		for _, ci := range connections {
			if strings.Contains(strings.ToLower(ci.RefID), cleaned) || strings.Contains(strings.ToLower(ci.Name), cleaned) {
				return ci
			}
		}
		return nil
	}

	// 3. Establish Swimlanes
	pkgLaneID := fmt.Sprintf("ssis:%s", packageName)
	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          pkgLaneID,
		Name:        fmt.Sprintf("SSIS Package (%s)", packageName),
		Description: fmt.Sprintf("SSIS ETL Package %s", fileName),
	})

	createdLanes := make(map[string]bool)
	createdLanes[pkgLaneID] = true

	ensureDBLane := func(dbName string) string {
		if dbName == "" {
			dbName = "Database"
		}
		laneID := fmt.Sprintf("db:%s", strings.ToLower(dbName))
		if !createdLanes[laneID] {
			createdLanes[laneID] = true
			result.Swimlanes = append(result.Swimlanes, model.Swimlane{
				ID:          laneID,
				Name:        fmt.Sprintf("Database (%s)", dbName),
				Description: fmt.Sprintf("Database %s referenced by SSIS package", dbName),
			})
		}
		return laneID
	}

	// Track created steps & database targets
	stepIDByRef := make(map[string]string)
	stepIDByName := make(map[string]string)
	createdDBSteps := make(map[string]string)
	var controlFlowStepIDs []string

	// 4. Parse Control Flow Executables
	var stepSeq int
	var processExecutable func(execNode *dtsxNode, parentLane string)

	processExecutable = func(execNode *dtsxNode, parentLane string) {
		refID := execNode.Attr("refId")
		// Skip root package executable
		if refID == "Package" || strings.EqualFold(execNode.Attr("CreationName"), "Microsoft.Package") {
			// Process direct child executables
			if execsNode := execNode.FindChild("Executables"); execsNode != nil {
				for _, child := range execsNode.FindChildren("Executable") {
					processExecutable(child, parentLane)
				}
			}
			return
		}

		stepSeq++
		name := execNode.Attr("ObjectName")
		if name == "" {
			name = fmt.Sprintf("Step_%d", stepSeq)
		}
		creationName := execNode.Attr("CreationName")
		desc := execNode.Attr("Description")
		if desc == "" {
			desc = name
		}

		stepID := refID
		if stepID == "" {
			stepID = fmt.Sprintf("%s:exec:%d", path, stepSeq)
		}

		stepType := classifySSISTask(creationName)
		laneID := parentLane

		// Specific task handling
		var sqlQuery string
		var targetDB string
		var targetProc string
		var targetTable string
		var targetAction string

		// A. Execute SQL Task
		if strings.Contains(strings.ToLower(creationName), "executesqltask") {
			sqlData := execNode.FindDescendant("SqlTaskData")
			if sqlData != nil {
				connRef := sqlData.Attr("Connection")
				ci := resolveConnection(connRef)
				if ci != nil {
					targetDB = ci.Database
				}
				sqlSource := sqlData.Attr("SqlStatementSource")
				if sqlData.Attr("SqlStmtSourceType") == "Variable" || strings.HasPrefix(sqlSource, "User::") {
					if v, ok := variables[sqlSource]; ok && v != "" {
						sqlQuery = v
					} else {
						sqlQuery = sqlSource
					}
				} else {
					sqlQuery = sqlSource
				}

				if sqlQuery != "" {
					desc = fmt.Sprintf("SQL: %s", summarizeQuery(sqlQuery))
					// Check for stored procedure
					if proc := extractStoredProcedure(sqlQuery); proc != "" {
						stepType = "StoredProcedure"
						targetProc = proc
						targetAction = "EXEC"
					} else if tbl, act := extractTableAndAction(sqlQuery); tbl != "" {
						stepType = "DatabaseQuery"
						targetTable = tbl
						targetAction = act
					} else {
						stepType = "DatabaseQuery"
					}
				}
			}
		}

		// B. Pipeline / Data Flow Task
		if strings.Contains(strings.ToLower(creationName), "pipeline") {
			stepType = "WorkflowStep"
			pipeNode := execNode.FindDescendant("pipeline")
			if pipeNode != nil {
				sources, dests, transforms := parseDataFlowComponents(pipeNode, connections)
				var details []string
				if len(sources) > 0 {
					details = append(details, fmt.Sprintf("Extract: %s", strings.Join(sources, ", ")))
				}
				if len(transforms) > 0 {
					details = append(details, fmt.Sprintf("Transform: %s", strings.Join(transforms, ", ")))
				}
				if len(dests) > 0 {
					details = append(details, fmt.Sprintf("Load: %s", strings.Join(dests, ", ")))
				}
				if len(details) > 0 {
					desc = strings.Join(details, " | ")
				} else {
					desc = "Data Flow Task"
				}

				// If single primary destination table
				if len(dests) > 0 {
					targetTable = dests[0]
					targetAction = "LOAD"
				}
			}
		}

		// C. Script Task
		if strings.Contains(strings.ToLower(creationName), "scripttask") {
			stepType = "Script"
			desc = "C# / VB.NET Script Task"
		}

		// D. Containers (Sequence, ForLoop, ForeachLoop)
		isContainer := strings.Contains(strings.ToLower(creationName), "sequence") ||
			strings.Contains(strings.ToLower(creationName), "forloop") ||
			strings.Contains(strings.ToLower(creationName), "foreachloop")

		st := model.Step{
			ID:          stepID,
			SwimlaneID:  laneID,
			Name:        name,
			Description: desc,
			Type:        stepType,
			Language:    "ssis",
			SourceFile:  path,
			LineNumber:  execNode.Line,
		}

		result.Steps = append(result.Steps, st)
		result.ASTNodes = append(result.ASTNodes, &model.ASTNode{
			ID:   stepID,
			Type: stepType,
			Name: name,
			Data: map[string]any{
				"file":          path,
				"creation_name": creationName,
				"line":          execNode.Line,
			},
		})

		stepIDByRef[refID] = stepID
		stepIDByName[name] = stepID
		controlFlowStepIDs = append(controlFlowStepIDs, stepID)

		// Create target database steps & links if discovered
		if targetProc != "" {
			if targetDB == "" {
				targetDB = extractDBFromProc(targetProc)
				if targetDB == "" {
					targetDB = "TRAIN"
				}
			}
			dbLane := ensureDBLane(targetDB)
			procKey := fmt.Sprintf("proc:%s:%s", targetDB, targetProc)
			procStepID, exists := createdDBSteps[procKey]
			if !exists {
				procStepID = fmt.Sprintf("proc_%s_%s", targetDB, sanitizeID(targetProc))
				createdDBSteps[procKey] = procStepID
				result.Steps = append(result.Steps, model.Step{
					ID:          procStepID,
					SwimlaneID:  dbLane,
					Name:        targetProc,
					Description: fmt.Sprintf("Stored Procedure %s on %s", targetProc, targetDB),
					Type:        "StoredProcedure",
					Language:    "sql",
					SourceFile:  path,
					LineNumber:  execNode.Line,
				})
			}
			result.Links = append(result.Links, model.Link{
				ID:             fmt.Sprintf("%s->%s:exec", stepID, procStepID),
				SourceStepID:   stepID,
				TargetStepID:   procStepID,
				Label:          targetAction,
				IsCrossService: true,
			})
		} else if targetTable != "" {
			if targetDB == "" {
				targetDB = extractDBFromTable(targetTable)
				if targetDB == "" {
					targetDB = "Database"
				}
			}
			dbLane := ensureDBLane(targetDB)
			tblKey := fmt.Sprintf("tbl:%s:%s", targetDB, targetTable)
			tblStepID, exists := createdDBSteps[tblKey]
			if !exists {
				tblStepID = fmt.Sprintf("tbl_%s_%s", targetDB, sanitizeID(targetTable))
				createdDBSteps[tblKey] = tblStepID
				result.Steps = append(result.Steps, model.Step{
					ID:          tblStepID,
					SwimlaneID:  dbLane,
					Name:        targetTable,
					Description: fmt.Sprintf("Table %s on %s", targetTable, targetDB),
					Type:        "DatabaseTable",
					Language:    "sql",
					SourceFile:  path,
					LineNumber:  execNode.Line,
				})
			}
			result.Links = append(result.Links, model.Link{
				ID:             fmt.Sprintf("%s->%s:db", stepID, tblStepID),
				SourceStepID:   stepID,
				TargetStepID:   tblStepID,
				Label:          targetAction,
				IsCrossService: true,
			})
		}

		// Recurse into container children
		if isContainer {
			if execsNode := execNode.FindChild("Executables"); execsNode != nil {
				for _, child := range execsNode.FindChildren("Executable") {
					processExecutable(child, parentLane)
				}
			}
		}
	}

	// Process package executables
	if execsNode := rootNode.FindChild("Executables"); execsNode != nil {
		for _, child := range execsNode.FindChildren("Executable") {
			processExecutable(child, pkgLaneID)
		}
	} else {
		// Fallback: search all descendant Executables
		for _, child := range rootNode.FindDescendants("Executable") {
			processExecutable(child, pkgLaneID)
		}
	}

	// 5. Parse Precedence Constraints
	var constraintCount int
	for _, pcsNode := range rootNode.FindDescendants("PrecedenceConstraints") {
		for _, pc := range pcsNode.FindChildren("PrecedenceConstraint") {
			fromRef := pc.Attr("From")
			toRef := pc.Attr("To")

			srcStep := stepIDByRef[fromRef]
			if srcStep == "" {
				srcStep = stepIDByName[fromRef]
			}
			if srcStep == "" {
				srcStep = stepIDByName[extractBaseName(fromRef)]
			}

			tgtStep := stepIDByRef[toRef]
			if tgtStep == "" {
				tgtStep = stepIDByName[toRef]
			}
			if tgtStep == "" {
				tgtStep = stepIDByName[extractBaseName(toRef)]
			}

			if srcStep != "" && tgtStep != "" {
				constraintCount++
				label, condition := formatPrecedenceCondition(pc, variables)
				result.Links = append(result.Links, model.Link{
					ID:           fmt.Sprintf("%s->%s:pc_%d", srcStep, tgtStep, constraintCount),
					SourceStepID: srcStep,
					TargetStepID: tgtStep,
					Label:        label,
					Condition:    condition,
				})
			}
		}
	}

	// 6. Sequential linking fallback if multiple control flow steps have no precedence constraints
	if constraintCount == 0 && len(controlFlowStepIDs) > 1 {
		for i := 0; i < len(controlFlowStepIDs)-1; i++ {
			result.Links = append(result.Links, model.Link{
				ID:           fmt.Sprintf("%s->%s:seq", controlFlowStepIDs[i], controlFlowStepIDs[i+1]),
				SourceStepID: controlFlowStepIDs[i],
				TargetStepID: controlFlowStepIDs[i+1],
				Label:        "",
			})
		}
	}

	return result, nil
}

// -----------------------------------------------------------------------------
// XML Parsing Helper
// -----------------------------------------------------------------------------

func parseDTSXTree(content []byte) (*dtsxNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var root *dtsxNode
	var stack []*dtsxNode

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			offset := int(decoder.InputOffset())
			if offset > len(content) {
				offset = len(content)
			}
			lineNum := bytes.Count(content[:offset], []byte{'\n'}) + 1

			attrs := make(map[string]string)
			for _, a := range t.Attr {
				attrs[strings.ToLower(a.Name.Local)] = a.Value
				if a.Name.Space != "" {
					attrs[strings.ToLower(fmt.Sprintf("%s:%s", a.Name.Space, a.Name.Local))] = a.Value
				}
			}

			node := &dtsxNode{
				Tag:   strings.ToLower(t.Name.Local),
				Attrs: attrs,
				Line:  lineNum,
			}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			} else if root == nil {
				root = node
			}
			stack = append(stack, node)

		case xml.CharData:
			if len(stack) > 0 {
				str := strings.TrimSpace(string(t))
				if str != "" {
					stack[len(stack)-1].Content += str
				}
			}

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if root == nil {
		return nil, fmt.Errorf("no root XML element found")
	}
	return root, nil
}

// -----------------------------------------------------------------------------
// Metadata Extraction Helpers
// -----------------------------------------------------------------------------

func parseConnectionManager(cmNode *dtsxNode, variables map[string]string) *connInfo {
	id := cmNode.Attr("DTSID")
	refID := cmNode.Attr("refId")
	name := cmNode.Attr("ObjectName")
	creationName := cmNode.Attr("CreationName")

	connStr := ""
	// Check PropertyExpression for ConnectionString
	for _, pe := range cmNode.FindDescendants("PropertyExpression") {
		if strings.EqualFold(pe.Attr("Name"), "ConnectionString") {
			expr := pe.Content
			if val, ok := variables[expr]; ok {
				connStr = val
			} else {
				connStr = expr
			}
		}
	}

	// Check ObjectData
	if connStr == "" {
		for _, child := range cmNode.FindDescendants("ConnectionManager") {
			if cs := child.Attr("ConnectionString"); cs != "" {
				connStr = cs
				break
			}
		}
	}

	database := extractParam(connStr, "Initial Catalog")
	if database == "" {
		database = extractParam(connStr, "Database")
	}
	server := extractParam(connStr, "Data Source")
	if server == "" {
		server = extractParam(connStr, "Server")
	}

	resource := database
	if resource == "" {
		resource = name
	}

	return &connInfo{
		ID:               id,
		RefID:            refID,
		Name:             name,
		CreationName:     creationName,
		ConnectionString: connStr,
		Database:         database,
		Server:           server,
		Resource:         resource,
	}
}

func extractParam(connStr, param string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(param) + `\s*=\s*([^;]+)`)
	m := re.FindStringSubmatch(connStr)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func classifySSISTask(creationName string) string {
	lower := strings.ToLower(creationName)
	switch {
	case strings.Contains(lower, "executesqltask"):
		return "DatabaseQuery"
	case strings.Contains(lower, "pipeline"):
		return "WorkflowStep"
	case strings.Contains(lower, "scripttask"):
		return "Script"
	case strings.Contains(lower, "executepackagetask"):
		return "Endpoint"
	case strings.Contains(lower, "filesystemtask"):
		return "Task"
	case strings.Contains(lower, "sendmailtask"):
		return "Endpoint"
	case strings.Contains(lower, "sequence") || strings.Contains(lower, "forloop") || strings.Contains(lower, "foreachloop"):
		return "WorkflowStep"
	default:
		return "WorkflowStep"
	}
}

func cleanSQLIdentifier(s string) string {
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	return strings.TrimSpace(s)
}

func extractStoredProcedure(query string) string {
	re := regexp.MustCompile(`(?i)^\s*(?:EXEC|EXECUTE)\s+([\[\]a-zA-Z0-9_\.]+)`)
	m := re.FindStringSubmatch(query)
	if len(m) > 1 {
		return cleanSQLIdentifier(m[1])
	}
	return ""
}

func extractTableAndAction(query string) (string, string) {
	re := regexp.MustCompile(`(?i)\b(INSERT\s+INTO|UPDATE|DELETE\s+FROM|TRUNCATE\s+TABLE|FROM|JOIN)\s+([\[\]\w\.]+)`)
	matches := re.FindAllStringSubmatch(query, -1)
	for _, m := range matches {
		action := strings.ToUpper(strings.TrimSpace(m[1]))
		tbl := strings.TrimSpace(m[2])
		if strings.EqualFold(tbl, "OPENJSON") || strings.EqualFold(tbl, "VALUES") || strings.EqualFold(tbl, "SELECT") {
			continue
		}
		tbl = cleanSQLIdentifier(tbl)
		if tbl != "" {
			return tbl, action
		}
	}
	return "", ""
}

func extractDBFromProc(proc string) string {
	parts := strings.Split(proc, ".")
	if len(parts) >= 3 {
		return parts[0]
	}
	return ""
}

func extractDBFromTable(table string) string {
	parts := strings.Split(table, ".")
	if len(parts) >= 3 {
		return parts[0]
	}
	return ""
}

func summarizeQuery(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func extractBaseName(ref string) string {
	if idx := strings.LastIndex(ref, `\`); idx != -1 {
		return ref[idx+1:]
	}
	return ref
}

func formatPrecedenceCondition(pc *dtsxNode, variables map[string]string) (string, string) {
	val := pc.Attr("Value")
	evalOp := pc.Attr("EvalOp")
	expr := strings.TrimSpace(pc.Attr("Expression"))

	varVarRe := regexp.MustCompile(`@\[(?:User::|System::)?([a-zA-Z0-9_]+)\]`)
	cleanExpr := varVarRe.ReplaceAllString(expr, "$1")

	label := ""
	condition := ""

	if expr != "" && (evalOp == "1" || evalOp == "3" || evalOp == "4") {
		condition = cleanExpr
		label = ""
	} else if val == "1" {
		label = "On Failure"
	} else if val == "2" {
		label = "On Completion"
	} else {
		label = ""
	}

	return label, condition
}

func parseDataFlowComponents(pipeNode *dtsxNode, connections map[string]*connInfo) (sources, dests, transforms []string) {
	compsNode := pipeNode.FindChild("components")
	if compsNode == nil {
		return
	}

	for _, comp := range compsNode.FindChildren("component") {
		name := comp.Attr("name")
		classID := strings.ToLower(comp.Attr("componentClassID"))
		openRowset := ""
		for _, prop := range comp.FindDescendants("property") {
			if strings.EqualFold(prop.Attr("name"), "OpenRowset") && prop.Content != "" {
				openRowset = cleanSQLIdentifier(prop.Content)
			}
		}

		switch {
		case strings.Contains(classID, "oledbsource") || strings.Contains(classID, "flatfilesource") || strings.Contains(classID, "excelsource"):
			target := openRowset
			if target == "" {
				target = name
			}
			sources = append(sources, target)
		case strings.Contains(classID, "oledbdestination") || strings.Contains(classID, "flatfiledestination") || strings.Contains(classID, "exceldestination") || strings.Contains(classID, "sqlserverdestination"):
			target := openRowset
			if target == "" {
				target = name
			}
			dests = append(dests, target)
		default:
			transforms = append(transforms, name)
		}
	}
	return
}
