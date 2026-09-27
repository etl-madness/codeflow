package flowxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/analyzer/liquibase"
	"codeflow/pkg/analyzer/ssis"
	"codeflow/pkg/model"
)

// Analyzer implements static analysis for workflow XML and SSIS packages.
type Analyzer struct{}

// New creates a new Flow XML Analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "flowxml"
}

// CanAnalyze returns true for .xml and .dtsx extensions.
func (a *Analyzer) CanAnalyze(ext string) bool {
	l := strings.ToLower(ext)
	return l == ".xml" || l == ".dtsx"
}

// Custom Workflow XML structures
type CustomWorkflowXML struct {
	XMLName     xml.Name              `xml:"workflow"`
	ID          string                `xml:"id,attr"`
	Name        string                `xml:"name,attr"`
	Swimlane    string                `xml:"swimlane,attr"`
	Steps       []CustomStepXML       `xml:"step"`
	Transitions []CustomTransitionXML `xml:"transition"`
}

type CustomStepXML struct {
	ID          string `xml:"id,attr"`
	Name        string `xml:"name,attr"`
	Type        string `xml:"type,attr"`
	Swimlane    string `xml:"swimlane,attr"`
	Line        int    `xml:"line,attr"`
	Description string `xml:"description"`
	Action      string `xml:"action"`
}

type CustomTransitionXML struct {
	ID        string `xml:"id,attr"`
	From      string `xml:"from,attr"`
	To        string `xml:"to,attr"`
	Label     string `xml:"label,attr"`
	Condition string `xml:"condition,attr"`
}

// BPMN / Generic Process XML structures
type ProcessXML struct {
	XMLName       xml.Name          `xml:"process"`
	ID            string            `xml:"id,attr"`
	Name          string            `xml:"name,attr"`
	Tasks         []GenericTaskXML  `xml:"task"`
	ServiceTasks  []GenericTaskXML  `xml:"serviceTask"`
	SequenceFlows []SequenceFlowXML `xml:"sequenceFlow"`
}

type GenericTaskXML struct {
	ID   string `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

type SequenceFlowXML struct {
	ID        string `xml:"id,attr"`
	SourceRef string `xml:"sourceRef,attr"`
	TargetRef string `xml:"targetRef,attr"`
	Name      string `xml:"name,attr"`
}

// AnalyzeFile unmarshals workflow XML definitions into canonical ProcessModel elements.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	fileName := filepath.Base(path)

	// 0. Check for SSIS (.dtsx or DTS:Executable)
	if strings.HasSuffix(strings.ToLower(path), ".dtsx") || bytes.Contains(content, []byte("<DTS:Executable")) || bytes.Contains(content, []byte("www.microsoft.com/SqlServer/Dts")) {
		return ssis.New().AnalyzeFile(path, content)
	}

	// 0b. Check for Liquibase XML (<databaseChangeLog)
	if bytes.Contains(content, []byte("<databaseChangeLog")) {
		return liquibase.New().AnalyzeFile(path, content)
	}

	// 1. Try Go-Flow / ETL Madness Pipeline XML format (<pipeline ...>)
	if a.parsePipelineXML(path, content, fileName, result) {
		return result, nil
	}

	// 2. Try Custom Workflow XML format
	var customWF CustomWorkflowXML
	if err := xml.Unmarshal(content, &customWF); err == nil && (len(customWF.Steps) > 0 || customWF.Name != "") {
		wfName := customWF.Name
		if wfName == "" {
			wfName = customWF.ID
		}
		if wfName == "" {
			wfName = fileName
		}

		defaultSwimlaneID := fmt.Sprintf("xml:%s", customWF.ID)
		if customWF.ID == "" {
			defaultSwimlaneID = fmt.Sprintf("xml:%s", fileName)
		}

		result.Swimlanes = append(result.Swimlanes, model.Swimlane{
			ID:          defaultSwimlaneID,
			Name:        wfName,
			Description: fmt.Sprintf("Workflow %s from %s", wfName, fileName),
		})

		stepIDMap := make(map[string]string)
		for idx, st := range customWF.Steps {
			stID := st.ID
			if stID == "" {
				stID = fmt.Sprintf("%s:step:%d", path, idx+1)
			}
			stepIDMap[st.ID] = stID

			stepLane := defaultSwimlaneID
			if st.Swimlane != "" {
				laneID := fmt.Sprintf("xml:%s", strings.ToLower(st.Swimlane))
				result.Swimlanes = append(result.Swimlanes, model.Swimlane{
					ID:          laneID,
					Name:        st.Swimlane,
					Description: fmt.Sprintf("Swimlane %s", st.Swimlane),
				})
				stepLane = laneID
			}

			stepType := st.Type
			if stepType == "" {
				stepType = "WorkflowStep"
			}

			desc := st.Description
			if desc == "" {
				desc = st.Action
			}
			if desc == "" {
				desc = fmt.Sprintf("Workflow step %s", st.Name)
			}

			line := st.Line
			if line <= 0 {
				line = idx + 1
			}

			result.Steps = append(result.Steps, model.Step{
				ID:          stID,
				SwimlaneID:  stepLane,
				Name:        st.Name,
				Description: desc,
				Type:        stepType,
				Language:    "flowxml",
				SourceFile:  path,
				LineNumber:  line,
			})
		}

		for idx, tr := range customWF.Transitions {
			src := stepIDMap[tr.From]
			if src == "" {
				src = tr.From
			}
			tgt := stepIDMap[tr.To]
			if tgt == "" {
				tgt = tr.To
			}

			linkID := tr.ID
			if linkID == "" {
				linkID = fmt.Sprintf("%s->%s:%d", src, tgt, idx)
			}

			result.Links = append(result.Links, model.Link{
				ID:           linkID,
				SourceStepID: src,
				TargetStepID: tgt,
				Label:        tr.Label,
				Condition:    tr.Condition,
			})
		}
		return result, nil
	}

	// 2. Try Generic BPMN / Process XML
	var proc ProcessXML
	if err := xml.Unmarshal(content, &proc); err == nil && (len(proc.Tasks) > 0 || len(proc.ServiceTasks) > 0) {
		procName := proc.Name
		if procName == "" {
			procName = proc.ID
		}
		laneID := fmt.Sprintf("xml:%s", proc.ID)
		result.Swimlanes = append(result.Swimlanes, model.Swimlane{
			ID:   laneID,
			Name: procName,
		})

		allTasks := append(proc.Tasks, proc.ServiceTasks...)
		for idx, task := range allTasks {
			result.Steps = append(result.Steps, model.Step{
				ID:          task.ID,
				SwimlaneID:  laneID,
				Name:        task.Name,
				Description: fmt.Sprintf("Task %s", task.Name),
				Type:        "WorkflowStep",
				Language:    "flowxml",
				SourceFile:  path,
				LineNumber:  idx + 1,
			})
		}

		for _, flow := range proc.SequenceFlows {
			result.Links = append(result.Links, model.Link{
				ID:           flow.ID,
				SourceStepID: flow.SourceRef,
				TargetStepID: flow.TargetRef,
				Label:        flow.Name,
			})
		}
		return result, nil
	}

	// 3. Fallback: Parse SSIS or other XML nodes iteratively
	a.parseGenericOrSSISXML(path, content, fileName, result)
	return result, nil
}

func (a *Analyzer) parseGenericOrSSISXML(path string, content []byte, fileName string, result *analyzer.FileAnalysisResult) {
	decoder := xml.NewDecoder(strings.NewReader(string(content)))
	swimlaneID := fmt.Sprintf("xml:%s", fileName)
	packageName := fmt.Sprintf("SSIS / Flow Package (%s)", fileName)

	var stepCounter int
	depth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			depth++
			local := t.Name.Local
			// Look for SSIS Executable or generic Task/Step
			if strings.EqualFold(local, "Executable") || strings.EqualFold(local, "Task") || strings.EqualFold(local, "Step") {
				name := ""
				stepType := "WorkflowStep"
				for _, attr := range t.Attr {
					if strings.EqualFold(attr.Name.Local, "ObjectName") || strings.EqualFold(attr.Name.Local, "Name") {
						name = attr.Value
					}
					if strings.EqualFold(attr.Name.Local, "CreationName") {
						stepType = attr.Value
					}
				}

				// If depth == 1, this is the root package itself
				if depth == 1 {
					if name != "" {
						packageName = name
					}
					continue
				}

				stepCounter++
				if name == "" {
					name = fmt.Sprintf("Step %d", stepCounter)
				}
				stepID := fmt.Sprintf("%s:ssis:%d", path, stepCounter)
				result.Steps = append(result.Steps, model.Step{
					ID:          stepID,
					SwimlaneID:  swimlaneID,
					Name:        name,
					Description: fmt.Sprintf("%s: %s", stepType, name),
					Type:        "WorkflowStep",
					Language:    "flowxml",
					SourceFile:  path,
					LineNumber:  stepCounter,
				})
			}

		case xml.EndElement:
			depth--
		}
	}

	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          swimlaneID,
		Name:        packageName,
		Description: fmt.Sprintf("Workflow package %s", fileName),
	})
}

// -------------------------------------------------------------------------
// Go-Flow / ETL Madness Pipeline XML Parser (<pipeline ...>)
// -------------------------------------------------------------------------

type pipelineNode struct {
	Tag         string
	ID          string
	DB          string
	TargetDB    string
	Language    string
	Var         string
	Equals      string
	OutputVar   string
	Desc        string
	Message     string
	OnFailure   string
	URL         string
	Method      string
	TargetTable string
	SourceTable string
	Content     string
	LineNumber  int
	Children    []*pipelineNode
	ThenBranch  []*pipelineNode
	ElseBranch  []*pipelineNode
}

func (a *Analyzer) parsePipelineXML(path string, content []byte, fileName string, result *analyzer.FileAnalysisResult) bool {
	if !bytes.Contains(content, []byte("<pipeline")) {
		return false
	}

	decoder := xml.NewDecoder(bytes.NewReader(content))
	var rootElem *xml.StartElement
	for {
		tok, err := decoder.Token()
		if err != nil {
			return false
		}
		if se, ok := tok.(xml.StartElement); ok {
			if strings.EqualFold(se.Name.Local, "pipeline") {
				rootElem = &se
				break
			}
			return false
		}
	}

	pipelineDesc := ""
	for _, attr := range rootElem.Attr {
		if strings.EqualFold(attr.Name.Local, "description") {
			pipelineDesc = attr.Value
		}
	}

	pipelineName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	variables := make(map[string]string)
	databases := make(map[string]string)
	var preflightNodes []*pipelineNode
	var flowNodes []*pipelineNode

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			tag := strings.ToLower(t.Name.Local)
			switch tag {
			case "variables":
				readPipelineVariables(decoder, variables)
			case "databases":
				readPipelineDatabases(decoder, databases)
			case "preflight":
				preflightNodes = readPipelineChildNodes(decoder, "preflight", content)
			case "flow":
				flowNodes = readPipelineChildNodes(decoder, "flow", content)
			default:
				_ = decoder.Skip()
			}
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "pipeline") {
				break
			}
		}
	}

	if len(preflightNodes) == 0 && len(flowNodes) == 0 {
		return false
	}

	var preflightLaneID string
	var flowLaneID string

	if len(preflightNodes) > 0 {
		preflightLaneID = fmt.Sprintf("pipeline:%s:preflight", pipelineName)
		result.Swimlanes = append(result.Swimlanes, model.Swimlane{
			ID:          preflightLaneID,
			Name:        fmt.Sprintf("Pipeline Preflight (%s)", pipelineName),
			Description: "Preflight validation and checks",
		})
	}

	if len(flowNodes) > 0 {
		flowLaneID = fmt.Sprintf("pipeline:%s:flow", pipelineName)
		laneName := fmt.Sprintf("Pipeline Flow (%s)", pipelineName)
		if len(preflightNodes) == 0 {
			laneName = fmt.Sprintf("Pipeline (%s)", pipelineName)
		}
		result.Swimlanes = append(result.Swimlanes, model.Swimlane{
			ID:          flowLaneID,
			Name:        laneName,
			Description: pipelineDesc,
		})
	}

	createdLanes := make(map[string]bool)
	ensureDBLane := func(dbName string) string {
		laneID := fmt.Sprintf("db:%s", dbName)
		if !createdLanes[laneID] {
			createdLanes[laneID] = true
			desc := databases[dbName]
			if desc == "" {
				desc = fmt.Sprintf("Database %s", dbName)
			}
			result.Swimlanes = append(result.Swimlanes, model.Swimlane{
				ID:          laneID,
				Name:        fmt.Sprintf("Database (%s)", dbName),
				Description: desc,
			})
		}
		return laneID
	}

	varProducers := make(map[string]string)
	createdTables := make(map[string]string)

	extractTableFromSQL := func(query string, defaultTable string) (string, string) {
		if defaultTable != "" {
			t := defaultTable
			for vName, vVal := range variables {
				t = strings.ReplaceAll(t, "{{"+vName+"}}", vVal)
			}
			return strings.Trim(t, "[]"), "WRITES"
		}
		re := regexp.MustCompile(`(?i)\b(INSERT\s+INTO|UPDATE|DELETE\s+FROM|TRUNCATE\s+TABLE|FROM|JOIN)\s+([\[\]\{\}\w\.]+)`)
		matches := re.FindAllStringSubmatch(query, -1)
		for _, m := range matches {
			action := strings.ToUpper(strings.TrimSpace(m[1]))
			tbl := strings.TrimSpace(m[2])
			if strings.EqualFold(tbl, "OPENJSON") || strings.EqualFold(tbl, "VALUES") || strings.EqualFold(tbl, "SELECT") {
				continue
			}
			for vName, vVal := range variables {
				tbl = strings.ReplaceAll(tbl, "{{"+vName+"}}", vVal)
			}
			tbl = strings.Trim(tbl, "[]{}")
			if tbl != "" {
				return tbl, action
			}
		}
		return "", ""
	}

	var stepSeq int
	createStep := func(n *pipelineNode, laneID string) model.Step {
		stepSeq++
		sID := n.ID
		if sID == "" {
			sID = fmt.Sprintf("%s_%s_%d", pipelineName, n.Tag, stepSeq)
		}

		sName := n.ID
		if sName == "" {
			if n.Desc != "" {
				sName = n.Desc
			} else {
				sName = fmt.Sprintf("%s_%d", strings.ToUpper(n.Tag), stepSeq)
			}
		}

		sType := "WorkflowStep"
		desc := n.Desc

		switch n.Tag {
		case "sql":
			sType = "DatabaseQuery"
			if desc == "" {
				desc = fmt.Sprintf("SQL query on %s", n.DB)
			}
		case "sql_bulk":
			sType = "BulkCopy"
			if desc == "" {
				desc = fmt.Sprintf("Bulk copy to %s", n.TargetTable)
			}
		case "script":
			sType = "Script"
			if desc == "" {
				desc = fmt.Sprintf("Execute %s script", n.Language)
			}
		case "assert":
			sType = "Assertion"
			if desc == "" {
				desc = fmt.Sprintf("Assert %s == %s", n.Var, n.Equals)
			}
		case "http", "http_client":
			sType = "HTTPClient"
			if desc == "" {
				desc = fmt.Sprintf("HTTP %s %s", n.Method, n.URL)
			}
		case "if":
			sType = "Condition"
			if desc == "" {
				desc = fmt.Sprintf("Check if %s == %s", n.Var, n.Equals)
			}
		case "parallel":
			sType = "Parallel"
			if desc == "" {
				desc = "Execute tasks in parallel"
			}
		case "foreach", "loop":
			sType = "Loop"
			if desc == "" {
				desc = fmt.Sprintf("Loop over %s", n.Var)
			}
		case "while":
			sType = "Loop"
			if desc == "" {
				desc = fmt.Sprintf("While %s == %s", n.Var, n.Equals)
			}
		case "group":
			sType = "Group"
			if desc == "" {
				desc = "Step group"
			}
		case "file_save", "file_read":
			sType = "FileIO"
		case "excel_read", "excel_write":
			sType = "ExcelIO"
		case "template", "template_html", "html_template":
			sType = "Template"
		case "kv", "kv_bulk":
			sType = "KeyValue"
		}

		st := model.Step{
			ID:          sID,
			SwimlaneID:  laneID,
			Name:        sName,
			Description: desc,
			Type:        sType,
			Language:    "flowxml",
			SourceFile:  path,
			LineNumber:  n.LineNumber,
		}

		result.Steps = append(result.Steps, st)
		result.ASTNodes = append(result.ASTNodes, &model.ASTNode{
			ID:   sID,
			Type: sType,
			Name: sName,
			Data: map[string]any{
				"file":        path,
				"line_number": n.LineNumber,
			},
		})

		if n.OutputVar != "" {
			varProducers[n.OutputVar] = sID
		}

		if (n.Tag == "sql" || n.Tag == "sql_bulk") && n.DB != "" {
			tbl, action := extractTableFromSQL(n.Content, n.TargetTable)
			if tbl != "" {
				dbLane := ensureDBLane(n.DB)
				tblKey := fmt.Sprintf("%s:%s", n.DB, tbl)
				tableStepID, exists := createdTables[tblKey]
				if !exists {
					tableStepID = fmt.Sprintf("tbl_%s_%s", n.DB, strings.ReplaceAll(tbl, ".", "_"))
					createdTables[tblKey] = tableStepID
					result.Steps = append(result.Steps, model.Step{
						ID:          tableStepID,
						SwimlaneID:  dbLane,
						Name:        tbl,
						Description: fmt.Sprintf("Table %s in %s", tbl, n.DB),
						Type:        "DatabaseTable",
						Language:    "sql",
						SourceFile:  path,
						LineNumber:  n.LineNumber,
					})
				}
				result.Links = append(result.Links, model.Link{
					ID:             fmt.Sprintf("%s->%s", sID, tableStepID),
					SourceStepID:   sID,
					TargetStepID:   tableStepID,
					Label:          action,
					IsCrossService: true,
				})
			}
		}

		return st
	}

	var linkNodes func(nodes []*pipelineNode, laneID string, prevSteps []string) []string
	linkNodes = func(nodes []*pipelineNode, laneID string, prevSteps []string) []string {
		currPrevs := prevSteps

		for _, node := range nodes {
			if node.Tag == "parallel" {
				parStep := createStep(node, laneID)
				for _, p := range currPrevs {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", p, parStep.ID),
						SourceStepID: p,
						TargetStepID: parStep.ID,
						Label:        "parallel",
					})
				}

				var parExits []string
				for _, child := range node.Children {
					childExits := linkNodes([]*pipelineNode{child}, laneID, []string{parStep.ID})
					parExits = append(parExits, childExits...)
				}
				if len(parExits) > 0 {
					currPrevs = parExits
				} else {
					currPrevs = []string{parStep.ID}
				}
				continue
			}

			if node.Tag == "if" {
				ifStep := createStep(node, laneID)
				for _, p := range currPrevs {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", p, ifStep.ID),
						SourceStepID: p,
						TargetStepID: ifStep.ID,
					})
				}

				var ifExits []string
				if len(node.ThenBranch) > 0 {
					firstThen := node.ThenBranch[0]
					thenStep := createStep(firstThen, laneID)
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s:then", ifStep.ID, thenStep.ID),
						SourceStepID: ifStep.ID,
						TargetStepID: thenStep.ID,
						Label:        "true",
					})
					thenRemainingExits := linkNodes(node.ThenBranch[1:], laneID, []string{thenStep.ID})
					ifExits = append(ifExits, thenRemainingExits...)
				}

				if len(node.ElseBranch) > 0 {
					firstElse := node.ElseBranch[0]
					elseStep := createStep(firstElse, laneID)
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s:else", ifStep.ID, elseStep.ID),
						SourceStepID: ifStep.ID,
						TargetStepID: elseStep.ID,
						Label:        "false",
					})
					elseRemainingExits := linkNodes(node.ElseBranch[1:], laneID, []string{elseStep.ID})
					ifExits = append(ifExits, elseRemainingExits...)
				} else {
					ifExits = append(ifExits, ifStep.ID)
				}

				if len(ifExits) > 0 {
					currPrevs = ifExits
				} else {
					currPrevs = []string{ifStep.ID}
				}
				continue
			}

			if node.Tag == "group" {
				currPrevs = linkNodes(node.Children, laneID, currPrevs)
				continue
			}

			if node.Tag == "foreach" || node.Tag == "while" {
				loopStep := createStep(node, laneID)
				for _, p := range currPrevs {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", p, loopStep.ID),
						SourceStepID: p,
						TargetStepID: loopStep.ID,
					})
				}
				if len(node.Children) > 0 {
					childExits := linkNodes(node.Children, laneID, []string{loopStep.ID})
					for _, ce := range childExits {
						result.Links = append(result.Links, model.Link{
							ID:           fmt.Sprintf("%s->%s:loop", ce, loopStep.ID),
							SourceStepID: ce,
							TargetStepID: loopStep.ID,
							Label:        "next",
						})
					}
				}
				currPrevs = []string{loopStep.ID}
				continue
			}

			st := createStep(node, laneID)
			for _, p := range currPrevs {
				label := ""
				for varName, prodStepID := range varProducers {
					if prodStepID == p {
						if strings.Contains(node.Content, varName) || node.Var == varName {
							label = varName
						}
					}
				}
				result.Links = append(result.Links, model.Link{
					ID:           fmt.Sprintf("%s->%s", p, st.ID),
					SourceStepID: p,
					TargetStepID: st.ID,
					Label:        label,
				})
			}
			currPrevs = []string{st.ID}
		}

		return currPrevs
	}

	var preflightExits []string
	if len(preflightNodes) > 0 {
		preflightExits = linkNodes(preflightNodes, preflightLaneID, nil)
	}

	if len(flowNodes) > 0 {
		linkNodes(flowNodes, flowLaneID, preflightExits)
	}

	return true
}

func readPipelineVariables(decoder *xml.Decoder, variables map[string]string) {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, "variable") {
				varName := ""
				varVal := ""
				for _, attr := range t.Attr {
					if strings.EqualFold(attr.Name.Local, "name") {
						varName = attr.Value
					}
					if strings.EqualFold(attr.Name.Local, "value") {
						varVal = attr.Value
					}
				}
				if varName != "" {
					variables[varName] = varVal
				}
				_ = decoder.Skip()
			}
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "variables") {
				return
			}
		}
	}
}

func readPipelineDatabases(decoder *xml.Decoder, databases map[string]string) {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, "database") {
				dbName := ""
				dbDesc := ""
				for _, attr := range t.Attr {
					if strings.EqualFold(attr.Name.Local, "name") {
						dbName = attr.Value
					}
					if strings.EqualFold(attr.Name.Local, "description") {
						dbDesc = attr.Value
					}
				}
				if dbName != "" {
					databases[dbName] = dbDesc
				}
				_ = decoder.Skip()
			}
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "databases") {
				return
			}
		}
	}
}

func readPipelineChildNodes(decoder *xml.Decoder, parentTag string, content []byte) []*pipelineNode {
	var nodes []*pipelineNode
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nodes
		}
		switch t := tok.(type) {
		case xml.StartElement:
			childNode := readSinglePipelineNode(decoder, t, content)
			nodes = append(nodes, childNode)
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, parentTag) {
				return nodes
			}
		}
	}
}

func readSinglePipelineNode(decoder *xml.Decoder, startElem xml.StartElement, content []byte) *pipelineNode {
	offset := int(decoder.InputOffset())
	if offset > len(content) {
		offset = len(content)
	}
	lineNum := bytes.Count(content[:offset], []byte{'\n'}) + 1

	node := &pipelineNode{
		Tag:        strings.ToLower(startElem.Name.Local),
		LineNumber: lineNum,
	}

	for _, attr := range startElem.Attr {
		k := strings.ToLower(attr.Name.Local)
		v := strings.TrimSpace(attr.Value)
		switch k {
		case "id":
			node.ID = v
		case "db":
			node.DB = v
		case "target_db":
			node.TargetDB = v
		case "language", "lang":
			node.Language = v
		case "var", "if_var":
			node.Var = v
		case "equals", "val", "value", "if_val", "if_equals":
			node.Equals = v
		case "output_var", "output_variable":
			node.OutputVar = v
		case "description", "desc":
			node.Desc = v
		case "message", "msg":
			node.Message = v
		case "on_failure":
			node.OnFailure = v
		case "url":
			node.URL = v
		case "method":
			node.Method = v
		case "target_table":
			node.TargetTable = v
		case "source_table":
			node.SourceTable = v
		}
	}

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			childTag := strings.ToLower(t.Name.Local)
			if node.Tag == "if" && childTag == "then" {
				node.ThenBranch = readPipelineChildNodes(decoder, "then", content)
			} else if node.Tag == "if" && childTag == "else" {
				node.ElseBranch = readPipelineChildNodes(decoder, "else", content)
			} else {
				child := readSinglePipelineNode(decoder, t, content)
				node.Children = append(node.Children, child)
			}
		case xml.CharData:
			node.Content += string(t)
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, startElem.Name.Local) {
				node.Content = strings.TrimSpace(node.Content)
				return node
			}
		}
	}

	node.Content = strings.TrimSpace(node.Content)
	return node
}

