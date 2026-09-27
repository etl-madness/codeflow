package mermaid

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codeflow/pkg/model"
)

// DiagramType represents the supported Mermaid diagram format.
type DiagramType string

const (
	TypeFlowchartTD DiagramType = "td"
	TypeFlowchartLR DiagramType = "lr"
	TypeJourney     DiagramType = "journey"     // Universal Journey Flowchart (flowchart LR with milestone stages)
	TypeSequence    DiagramType = "sequence"    // sequenceDiagram
	TypeERDiagram   DiagramType = "er"          // erDiagram
	TypeState       DiagramType = "state"       // stateDiagram-v2
	TypeRawJourney  DiagramType = "raw-journey" // Native 'journey' keyword
)

// Exporter converts a ProcessModel into Mermaid diagrams.
type Exporter struct {
	NoTruncate bool
}

// New creates a new Mermaid Exporter.
func New() *Exporter {
	return &Exporter{}
}

// Export writes the default top-down flowchart (.mmd) to outputPath.
func (e *Exporter) Export(pm *model.ProcessModel, outputPath string) error {
	return e.ExportFlowchart(pm, outputPath, "TD")
}

// ExportFlowchart writes a flowchart (TD or LR) diagram to outputPath.
func (e *Exporter) ExportFlowchart(pm *model.ProcessModel, outputPath, direction string) error {
	content := e.GenerateFlowchart(pm, direction)
	return writeFile(outputPath, content)
}

// ExportJourney writes a universal journey flowchart (flowchart LR) to outputPath.
func (e *Exporter) ExportJourney(pm *model.ProcessModel, outputPath string) error {
	content := e.GenerateJourney(pm)
	return writeFile(outputPath, content)
}

// ExportSequence writes a sequence diagram (sequenceDiagram) to outputPath.
func (e *Exporter) ExportSequence(pm *model.ProcessModel, outputPath string) error {
	content := e.GenerateSequenceDiagram(pm)
	return writeFile(outputPath, content)
}

// ExportERDiagram writes an Entity-Relationship diagram (erDiagram) to outputPath.
func (e *Exporter) ExportERDiagram(pm *model.ProcessModel, outputPath string) error {
	content := e.GenerateERDiagram(pm)
	return writeFile(outputPath, content)
}

// ExportStateDiagram writes a state diagram (stateDiagram-v2) to outputPath.
func (e *Exporter) ExportStateDiagram(pm *model.ProcessModel, outputPath string) error {
	content := e.GenerateStateDiagram(pm)
	return writeFile(outputPath, content)
}

// ExportMarkdown writes the specified diagram wrapped inside a standard markdown code block.
func (e *Exporter) ExportMarkdown(pm *model.ProcessModel, outputPath string, dType DiagramType) error {
	var diagramContent string
	switch dType {
	case TypeFlowchartLR:
		diagramContent = e.GenerateFlowchart(pm, "LR")
	case TypeJourney:
		diagramContent = e.GenerateJourney(pm)
	case TypeSequence:
		diagramContent = e.GenerateSequenceDiagram(pm)
	case TypeERDiagram:
		diagramContent = e.GenerateERDiagram(pm)
	case TypeState:
		diagramContent = e.GenerateStateDiagram(pm)
	case TypeRawJourney:
		diagramContent = e.GenerateRawJourney(pm)
	default:
		diagramContent = e.GenerateFlowchart(pm, "TD")
	}

	title := pm.Name
	if title == "" {
		title = "Architecture Workflow"
	}
	mdContent := fmt.Sprintf("# %s\n\n```mermaid\n%s```\n", title, diagramContent)
	return writeFile(outputPath, mdContent)
}

// Generate returns the default flowchart TD representation.
func (e *Exporter) Generate(pm *model.ProcessModel) string {
	return e.GenerateFlowchart(pm, "TD")
}

// GenerateFlowchart returns a Mermaid flowchart (TD or LR) styled to match modern vector graphics.
// It maps node IDs to compact short IDs to stay within Mermaid's character limit.
func (e *Exporter) GenerateFlowchart(pm *model.ProcessModel, direction string) string {
	var sb strings.Builder

	dir := strings.ToUpper(strings.TrimSpace(direction))
	if dir != "LR" {
		dir = "TD"
	}

	sb.WriteString(themeInitFlowchart())
	sb.WriteString(fmt.Sprintf("flowchart %s\n", dir))
	sb.WriteString(flowchartClassDefs() + "\n")

	stepIDMap := make(map[string]string)
	getShortID := func(rawID string) string {
		if sid, ok := stepIDMap[rawID]; ok {
			return sid
		}
		sid := fmt.Sprintf("s%d", len(stepIDMap)+1)
		stepIDMap[rawID] = sid
		return sid
	}

	for _, step := range pm.Steps {
		_ = getShortID(step.ID)
	}

	stepsByLane := make(map[string][]model.Step)
	for _, step := range pm.Steps {
		stepsByLane[step.SwimlaneID] = append(stepsByLane[step.SwimlaneID], step)
	}

	var laneStyles []string
	for _, lane := range pm.Swimlanes {
		steps := stepsByLane[lane.ID]
		if len(steps) == 0 {
			continue
		}

		laneSafeID := sanitizeID("lane_" + lane.ID)
		sb.WriteString(fmt.Sprintf("    subgraph %s [\"%s\"]\n", laneSafeID, escapeLabel(lane.Name)))

		for _, step := range steps {
			stepSafeID := getShortID(step.ID)
			nodeShape := e.formatNode(stepSafeID, step.Name, step.Description, step.Type)
			sb.WriteString(fmt.Sprintf("        %s\n", nodeShape))
		}

		sb.WriteString("    end\n\n")
		laneStyles = append(laneStyles, fmt.Sprintf("    style %s fill:#F8FAFC,stroke:#E2E8F0,stroke-width:1.5px,rx:8px,ry:8px;", laneSafeID))
	}

	var unassigned []model.Step
	for _, step := range pm.Steps {
		if pm.FindSwimlane(step.SwimlaneID) == nil {
			unassigned = append(unassigned, step)
		}
	}
	if len(unassigned) > 0 {
		sb.WriteString("    subgraph lane_general [\"General Process\"]\n")
		for _, step := range unassigned {
			stepSafeID := getShortID(step.ID)
			nodeShape := e.formatNode(stepSafeID, step.Name, step.Description, step.Type)
			sb.WriteString(fmt.Sprintf("        %s\n", nodeShape))
		}
		sb.WriteString("    end\n\n")
		laneStyles = append(laneStyles, "    style lane_general fill:#F8FAFC,stroke:#E2E8F0,stroke-width:1.5px,rx:8px,ry:8px;")
	}

	seenLinks := make(map[string]bool)
	for _, link := range pm.Links {
		srcID := getShortID(link.SourceStepID)
		tgtID := getShortID(link.TargetStepID)
		label := escapeLabel(link.Label)
		if link.Condition != "" {
			if label != "" && !strings.Contains(label, link.Condition) {
				label = fmt.Sprintf("[%s] %s", link.Condition, label)
			} else if label == "" {
				label = fmt.Sprintf("[%s]", link.Condition)
			}
		}

		arrow := "-->"
		if link.IsCrossService {
			arrow = "-.->"
		}

		linkKey := fmt.Sprintf("%s|%s|%s|%s", srcID, arrow, label, tgtID)
		if seenLinks[linkKey] {
			continue
		}
		seenLinks[linkKey] = true

		if label != "" {
			sb.WriteString(fmt.Sprintf("    %s %s|\"%s\"| %s\n", srcID, arrow, label, tgtID))
		} else {
			sb.WriteString(fmt.Sprintf("    %s %s %s\n", srcID, arrow, tgtID))
		}
	}

	if len(laneStyles) > 0 {
		sb.WriteString("\n")
		for _, ls := range laneStyles {
			sb.WriteString(ls + "\n")
		}
	}

	return sb.String()
}

// GenerateJourney outputs a Universal Journey Flowchart using the standard 'flowchart LR' header.
// This is 100% compatible with all Markdown previewers and GitHub/IDE renderers.
func (e *Exporter) GenerateJourney(pm *model.ProcessModel) string {
	var sb strings.Builder
	sb.WriteString(`%%{init: {"theme": "base", "themeVariables": {"fontFamily": "-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif", "fontSize": "13px", "primaryColor": "#FFFFFF", "primaryBorderColor": "#CBD5E1", "primaryTextColor": "#0F172A", "lineColor": "#2563EB", "clusterBkg": "#F8FAFC", "clusterBorder": "#CBD5E1", "edgeLabelBackground": "#FFFFFF"}, "maxTextSize": 1000000}}%%` + "\n")
	sb.WriteString("flowchart LR\n")
	sb.WriteString(flowchartClassDefs() + "\n")

	stepsByLane := make(map[string][]model.Step)
	for _, step := range pm.Steps {
		stepsByLane[step.SwimlaneID] = append(stepsByLane[step.SwimlaneID], step)
	}

	stepIDMap := make(map[string]string)
	getShortID := func(rawID string) string {
		if sid, ok := stepIDMap[rawID]; ok {
			return sid
		}
		sid := fmt.Sprintf("j%d", len(stepIDMap)+1)
		stepIDMap[rawID] = sid
		return sid
	}

	stageColors := []string{"#3B82F6", "#10B981", "#8B5CF6", "#F59E0B", "#EC4899", "#06B6D4"}
	var activeLanes []string
	var stageStyles []string

	for idx, lane := range pm.Swimlanes {
		steps := stepsByLane[lane.ID]
		if len(steps) == 0 {
			continue
		}

		stageID := fmt.Sprintf("stage_%d", idx+1)
		activeLanes = append(activeLanes, stageID)

		sb.WriteString(fmt.Sprintf("    subgraph %s [\"Stage %d: %s\"]\n", stageID, idx+1, escapeLabel(lane.Name)))
		sb.WriteString("        direction TB\n")

		// List milestone steps (up to 12 per lane to keep journey clear and fast-rendering)
		limit := 12
		if len(steps) < limit {
			limit = len(steps)
		}
		for i := 0; i < limit; i++ {
			step := steps[i]
			sid := getShortID(step.ID)
			nodeShape := e.formatNode(sid, step.Name, step.Description, step.Type)
			sb.WriteString(fmt.Sprintf("        %s\n", nodeShape))
		}
		if len(steps) > limit {
			moreID := fmt.Sprintf("%s_more", stageID)
			sb.WriteString(fmt.Sprintf("        %s[\"+ %d more steps...\"]\n", moreID, len(steps)-limit))
		}

		sb.WriteString("    end\n\n")
		color := stageColors[idx%len(stageColors)]
		stageStyles = append(stageStyles, fmt.Sprintf("    style %s fill:#F8FAFC,stroke:%s,stroke-width:2px,rx:8px,ry:8px;", stageID, color))
	}

	// Connect stages sequentially with thick milestone arrows
	for i := 0; i < len(activeLanes)-1; i++ {
		sb.WriteString(fmt.Sprintf("    %s ==> %s\n", activeLanes[i], activeLanes[i+1]))
	}

	if len(stageStyles) > 0 {
		sb.WriteString("\n")
		for _, ss := range stageStyles {
			sb.WriteString(ss + "\n")
		}
	}

	return sb.String()
}

// GenerateSequenceDiagram outputs a Mermaid sequenceDiagram showing interactions across swimlanes.
func (e *Exporter) GenerateSequenceDiagram(pm *model.ProcessModel) string {
	var sb strings.Builder
	sb.WriteString(`%%{init: {"theme": "base", "themeVariables": {"fontFamily": "-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif", "actorBkg": "#FFFFFF", "actorBorder": "#3B82F6", "actorTextColor": "#0F172A", "actorLineColor": "#94A3B8", "signalColor": "#475569", "signalTextColor": "#0F172A", "labelBoxBkgColor": "#F8FAFC", "labelBoxBorderColor": "#E2E8F0", "activationBorderColor": "#2563EB", "activationBkgColor": "#EFF6FF"}}}%%` + "\n")
	sb.WriteString("sequenceDiagram\n")
	sb.WriteString("    autonumber\n")

	laneAlias := make(map[string]string)
	for idx, lane := range pm.Swimlanes {
		alias := fmt.Sprintf("P%d", idx+1)
		laneAlias[lane.ID] = alias
		sb.WriteString(fmt.Sprintf("    participant %s as %s\n", alias, escapeLabel(lane.Name)))
	}

	seenInteractions := make(map[string]bool)
	interactionCount := 0

	for _, link := range pm.Links {
		srcStep := pm.FindStep(link.SourceStepID)
		tgtStep := pm.FindStep(link.TargetStepID)
		if srcStep == nil || tgtStep == nil {
			continue
		}

		srcLane := laneAlias[srcStep.SwimlaneID]
		tgtLane := laneAlias[tgtStep.SwimlaneID]
		if srcLane == "" || tgtLane == "" || srcLane == tgtLane {
			continue
		}

		label := sanitizeEntityLabel(link.Label)
		if label == "" {
			label = sanitizeEntityLabel(tgtStep.Name)
		}
		if label == "" {
			label = "calls"
		}

		key := fmt.Sprintf("%s->%s:%s", srcLane, tgtLane, label)
		if seenInteractions[key] {
			continue
		}
		seenInteractions[key] = true
		interactionCount++

		arrow := "->>"
		if link.IsCrossService {
			arrow = "-->>"
		}

		sb.WriteString(fmt.Sprintf("    %s%s%s: %s\n", srcLane, arrow, tgtLane, label))
		if interactionCount >= 40 {
			break
		}
	}

	if interactionCount == 0 && len(pm.Swimlanes) > 1 {
		for i := 0; i < len(pm.Swimlanes)-1; i++ {
			l1 := laneAlias[pm.Swimlanes[i].ID]
			l2 := laneAlias[pm.Swimlanes[i+1].ID]
			sb.WriteString(fmt.Sprintf("    %s->>%s: invokes\n", l1, l2))
		}
	}

	return sb.String()
}

// GenerateStateDiagram outputs a Mermaid stateDiagram-v2 showing process state progression.
func (e *Exporter) GenerateStateDiagram(pm *model.ProcessModel) string {
	var sb strings.Builder
	sb.WriteString(`%%{init: {"theme": "base", "themeVariables": {"fontFamily": "-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif", "stateBkg": "#FFFFFF", "stateBorder": "#3B82F6", "stateLabelColor": "#0F172A", "labelColor": "#0F172A", "transitionColor": "#64748B", "transitionLabelColor": "#334155"}}}%%` + "\n")
	sb.WriteString("stateDiagram-v2\n")
	sb.WriteString("    [*] --> Start\n")

	stepsByLane := make(map[string][]model.Step)
	for _, step := range pm.Steps {
		stepsByLane[step.SwimlaneID] = append(stepsByLane[step.SwimlaneID], step)
	}

	for idx, lane := range pm.Swimlanes {
		stateID := fmt.Sprintf("State_%d", idx+1)
		sb.WriteString(fmt.Sprintf("    state \"%s\" as %s\n", escapeLabel(lane.Name), stateID))
		steps := stepsByLane[lane.ID]
		for stepIdx, step := range steps {
			if stepIdx >= 4 {
				sb.WriteString(fmt.Sprintf("    %s: ... +%d more steps\n", stateID, len(steps)-4))
				break
			}
			prefix := "do /"
			if stepIdx == 0 {
				prefix = "entry /"
			} else if stepIdx == len(steps)-1 {
				prefix = "exit /"
			}
			sb.WriteString(fmt.Sprintf("    %s: %s %s\n", stateID, prefix, escapeLabel(step.Name)))
		}
	}

	if len(pm.Swimlanes) > 0 {
		sb.WriteString("    Start --> State_1\n")
		for i := 0; i < len(pm.Swimlanes)-1; i++ {
			sb.WriteString(fmt.Sprintf("    State_%d --> State_%d: [next]\n", i+1, i+2))
		}
		sb.WriteString(fmt.Sprintf("    State_%d --> [*]\n", len(pm.Swimlanes)))
	} else {
		sb.WriteString("    Start --> [*]\n")
	}

	return sb.String()
}

// GenerateERDiagram returns a Mermaid Entity-Relationship diagram (erDiagram).
func (e *Exporter) GenerateERDiagram(pm *model.ProcessModel) string {
	var sb strings.Builder
	sb.WriteString(`%%{init: {"theme": "base", "themeVariables": {"fontFamily": "-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif", "primaryColor": "#FFFFFF", "primaryBorderColor": "#3B82F6", "primaryTextColor": "#0F172A", "lineColor": "#64748B", "textColor": "#0F172A"}}}%%` + "\n")
	sb.WriteString("erDiagram\n")

	type Entity struct {
		Name    string
		Columns []map[string]string
	}
	entities := make(map[string]*Entity)

	for _, step := range pm.Steps {
		tableName := ""
		if tbl, ok := step.Metadata["table"].(string); ok && tbl != "" {
			tableName = tbl
		} else if step.Type == "TableOperation" || step.Type == "DatabaseQuery" {
			tableName = cleanEntityName(step.Name)
		}

		if tableName != "" {
			cleanName := sanitizeEntityIdentifier(tableName)
			if cleanName != "" {
				if _, ok := entities[cleanName]; !ok {
					entities[cleanName] = &Entity{Name: cleanName}
				}
				if colsRaw, ok := step.Metadata["columns"].([]map[string]string); ok && len(colsRaw) > 0 {
					entities[cleanName].Columns = colsRaw
				}
			}
		}
	}

	serviceEntities := make(map[string]string)
	for _, lane := range pm.Swimlanes {
		laneEntity := sanitizeEntityIdentifier(lane.Name)
		if laneEntity != "" {
			serviceEntities[lane.ID] = laneEntity
		}
	}

	for _, ent := range entities {
		if len(ent.Columns) > 0 {
			sb.WriteString(fmt.Sprintf("    %s {\n", ent.Name))
			for _, col := range ent.Columns {
				colType := sanitizeAttributeType(col["type"])
				colName := sanitizeEntityIdentifier(col["name"])
				key := col["key"]
				if key != "" {
					sb.WriteString(fmt.Sprintf("        %s %s %s\n", colType, colName, key))
				} else {
					sb.WriteString(fmt.Sprintf("        %s %s\n", colType, colName))
				}
			}
			sb.WriteString("    }\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("    %s {\n        string id PK\n    }\n\n", ent.Name))
		}
	}

	seenRels := make(map[string]bool)
	for _, link := range pm.Links {
		srcStep := pm.FindStep(link.SourceStepID)
		tgtStep := pm.FindStep(link.TargetStepID)
		if srcStep == nil || tgtStep == nil {
			continue
		}

		srcEnt := getStepEntityName(srcStep, serviceEntities)
		tgtEnt := getStepEntityName(tgtStep, serviceEntities)

		if srcEnt != "" && tgtEnt != "" && srcEnt != tgtEnt {
			relKey := fmt.Sprintf("%s->%s", srcEnt, tgtEnt)
			if !seenRels[relKey] {
				seenRels[relKey] = true
				label := sanitizeEntityLabel(link.Label)
				if label == "" {
					label = "queries"
				}
				sb.WriteString(fmt.Sprintf("    %s ||--o{ %s : \"%s\"\n", srcEnt, tgtEnt, label))
			}
		}
	}

	if len(seenRels) == 0 {
		for _, step := range pm.Steps {
			if tbl, ok := step.Metadata["table"].(string); ok && tbl != "" {
				tblEnt := sanitizeEntityIdentifier(tbl)
				if svcEnt, ok := serviceEntities[step.SwimlaneID]; ok && svcEnt != tblEnt {
					relKey := fmt.Sprintf("%s->%s", svcEnt, tblEnt)
					if !seenRels[relKey] {
						seenRels[relKey] = true
						sb.WriteString(fmt.Sprintf("    %s ||--o{ %s : \"manages\"\n", svcEnt, tblEnt))
					}
				}
			}
		}
	}

	return sb.String()
}

// GenerateRawJourney outputs native Mermaid 'journey' syntax (for engines supporting the keyword).
func (e *Exporter) GenerateRawJourney(pm *model.ProcessModel) string {
	var sb strings.Builder
	sb.WriteString("journey\n")

	title := pm.Name
	if title == "" {
		title = "System Execution Journey"
	}
	sb.WriteString(fmt.Sprintf("    title %s\n", cleanJourneyText(title)))

	stepsByLane := make(map[string][]model.Step)
	for _, step := range pm.Steps {
		stepsByLane[step.SwimlaneID] = append(stepsByLane[step.SwimlaneID], step)
	}

	for _, lane := range pm.Swimlanes {
		steps := stepsByLane[lane.ID]
		if len(steps) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("    section %s\n", cleanJourneyText(lane.Name)))

		seenNames := make(map[string]bool)
		for _, step := range steps {
			name := cleanJourneyText(step.Name)
			if name == "" || seenNames[name] {
				continue
			}
			seenNames[name] = true

			score := 4
			switch step.Type {
			case "Endpoint":
				score = 5
			case "DatabaseQuery", "TableOperation":
				score = 4
			default:
				score = 3
			}

			actor := cleanJourneyText(lane.Name)
			if actor == "" {
				actor = "System"
			}

			sb.WriteString(fmt.Sprintf("        %s: %d: %s\n", name, score, actor))
		}
	}

	return sb.String()
}

func getStepEntityName(step *model.Step, serviceEntities map[string]string) string {
	if tbl, ok := step.Metadata["table"].(string); ok && tbl != "" {
		return sanitizeEntityIdentifier(tbl)
	}
	if step.Type == "TableOperation" || step.Type == "DatabaseQuery" {
		name := cleanEntityName(step.Name)
		if name != "" {
			return sanitizeEntityIdentifier(name)
		}
	}
	if svc, ok := serviceEntities[step.SwimlaneID]; ok {
		return svc
	}
	return ""
}

func cleanEntityName(name string) string {
	clean := strings.TrimPrefix(name, "CREATE TABLE ")
	clean = strings.TrimPrefix(clean, "CREATE ")
	clean = strings.TrimPrefix(clean, "TABLE ")
	clean = strings.TrimPrefix(clean, "UPDATE ")
	clean = strings.TrimPrefix(clean, "INSERT INTO ")
	clean = strings.TrimPrefix(clean, "SELECT FROM ")
	clean = strings.TrimPrefix(clean, "DELETE FROM ")
	parts := strings.Fields(clean)
	if len(parts) > 0 {
		return parts[0]
	}
	return clean
}

var entityIdentRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeEntityIdentifier(s string) string {
	s = strings.TrimSpace(s)
	s = entityIdentRegex.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9') {
		s = "E_" + s
	}
	return s
}

func sanitizeAttributeType(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return "string"
	}
	t = entityIdentRegex.ReplaceAllString(t, "_")
	return strings.Trim(t, "_")
}

func sanitizeEntityLabel(l string) string {
	l = strings.ReplaceAll(l, "\"", "")
	l = strings.ReplaceAll(l, "\n", " ")
	return strings.TrimSpace(l)
}

func cleanJourneyText(s string) string {
	s = strings.ReplaceAll(s, ":", " - ")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.Join(strings.Fields(s), " ")
}

func themeInitFlowchart() string {
	return `%%{init: {"theme": "base", "themeVariables": {"fontFamily": "-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif", "fontSize": "13px", "primaryColor": "#FFFFFF", "primaryBorderColor": "#CBD5E1", "primaryTextColor": "#0F172A", "secondaryColor": "#F8FAFC", "tertiaryColor": "#F1F5F9", "lineColor": "#64748B", "textColor": "#0F172A", "mainBkg": "#FFFFFF", "nodeBorder": "#CBD5E1", "clusterBkg": "#F8FAFC", "clusterBorder": "#E2E8F0", "titleColor": "#0F172A", "edgeLabelBackground": "#FFFFFF"}, "maxTextSize": 1000000}}%%` + "\n"
}

func flowchartClassDefs() string {
	return `    classDef default fill:#FFFFFF,stroke:#CBD5E1,stroke-width:1.5px,color:#0F172A,rx:6px,ry:6px;
    classDef defaultNode fill:#FFFFFF,stroke:#CBD5E1,stroke-width:1.5px,color:#0F172A,rx:6px,ry:6px;
    classDef dbNode fill:#FFFFFF,stroke:#10B981,stroke-width:2px,color:#065F46,rx:6px,ry:6px;
    classDef apiNode fill:#FFFFFF,stroke:#3B82F6,stroke-width:2px,color:#1E40AF,rx:6px,ry:6px;
    classDef scriptNode fill:#FFFFFF,stroke:#6366F1,stroke-width:2px,color:#3730A3,rx:6px,ry:6px;
    classDef procNode fill:#FFFFFF,stroke:#8B5CF6,stroke-width:2px,color:#5B21B6,rx:6px,ry:6px;
    classDef taskNode fill:#FFFFFF,stroke:#F59E0B,stroke-width:2px,color:#92400E,rx:6px,ry:6px;
    classDef assertNode fill:#FFFFFF,stroke:#EC4899,stroke-width:2px,color:#9D174D,rx:6px,ry:6px;`
}

func (e *Exporter) formatNode(id, name, desc, stepType string) string {
	escapedName := escapeNodeText(name)
	badge := formatTypeBadge(stepType)
	class := getClassForType(stepType)

	var labelContent string
	escapedDesc := escapeNodeText(desc)
	hasDesc := escapedDesc != "" && escapedDesc != escapedName && !strings.EqualFold(escapedDesc, badge)

	if badge != "" && hasDesc {
		labelContent = fmt.Sprintf("<b>%s</b><br/><small>%s</small><br/><sub>%s</sub>", escapedName, escapeNodeText(badge), escapedDesc)
	} else if badge != "" {
		labelContent = fmt.Sprintf("<b>%s</b><br/><small>%s</small>", escapedName, escapeNodeText(badge))
	} else if hasDesc {
		labelContent = fmt.Sprintf("<b>%s</b><br/><sub>%s</sub>", escapedName, escapedDesc)
	} else {
		labelContent = fmt.Sprintf("<b>%s</b>", escapedName)
	}

	switch stepType {
	case "Endpoint":
		return fmt.Sprintf("%s([\"%s\"]):::%s", id, labelContent, class)
	case "DatabaseQuery", "TableOperation", "DatabaseTable":
		return fmt.Sprintf("%s[(\"%s\")]:::%s", id, labelContent, class)
	case "View":
		return fmt.Sprintf("%s[[\"%s\"]):::%s", id, labelContent, class)
	default:
		return fmt.Sprintf("%s[\"%s\"]:::%s", id, labelContent, class)
	}
}

func escapeNodeText(text string) string {
	text = strings.ReplaceAll(text, "\"", "'")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "[", "&#91;")
	text = strings.ReplaceAll(text, "]", "&#93;")
	return strings.TrimSpace(text)
}

func getClassForType(stepType string) string {
	switch stepType {
	case "Endpoint":
		return "apiNode"
	case "DatabaseQuery", "TableOperation", "DatabaseTable":
		return "dbNode"
	case "StoredProcedure", "View":
		return "procNode"
	case "Task", "WorkflowStep":
		return "taskNode"
	case "Script":
		return "scriptNode"
	case "Assertion", "Condition":
		return "assertNode"
	default:
		return "defaultNode"
	}
}

func formatTypeBadge(stepType string) string {
	switch stepType {
	case "Endpoint":
		return "API Endpoint"
	case "DatabaseQuery":
		return "SQL Query"
	case "TableOperation":
		return "Table Operation"
	case "DatabaseTable":
		return "Database Table"
	case "StoredProcedure":
		return "Stored Procedure"
	case "View":
		return "Database View"
	case "Script":
		return "Script"
	case "Assertion":
		return "Assertion"
	case "Condition":
		return "Condition"
	case "Task":
		return "Task"
	case "WorkflowStep":
		return "Workflow Step"
	default:
		if stepType != "" {
			return stepType
		}
		return ""
	}
}

var sanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeID(id string) string {
	res := sanitizeRegex.ReplaceAllString(id, "_")
	if len(res) > 0 && (res[0] >= '0' && res[0] <= '9') {
		res = "n_" + res
	}
	return res
}

func escapeLabel(label string) string {
	label = strings.ReplaceAll(label, "\"", "'")
	label = strings.ReplaceAll(label, "\n", " ")
	return strings.TrimSpace(label)
}

func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}
