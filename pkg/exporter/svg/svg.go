package svg

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"

	"codeflow/pkg/model"
)

// Exporter converts a ProcessModel into SVG vector diagrams.
type Exporter struct {
	NoTruncate bool
}

// New creates a new SVG Exporter.
func New() *Exporter {
	return &Exporter{}
}

// Export writes the ProcessModel as a Flowchart TD SVG file to outputPath.
func (e *Exporter) Export(pm *model.ProcessModel, outputPath string) error {
	return e.ExportFlowchart(pm, outputPath, "TD")
}

// ExportFlowchart writes a Flowchart (TD or LR) SVG to outputPath.
func (e *Exporter) ExportFlowchart(pm *model.ProcessModel, outputPath string, direction string) error {
	return writeSVGFile(outputPath, e.GenerateFlowchart(pm, direction))
}

// ExportJourney writes a Customer/Data Journey SVG to outputPath.
func (e *Exporter) ExportJourney(pm *model.ProcessModel, outputPath string) error {
	return writeSVGFile(outputPath, e.GenerateJourney(pm))
}

// ExportSequenceDiagram writes a Lifeline Sequence Diagram SVG to outputPath.
func (e *Exporter) ExportSequenceDiagram(pm *model.ProcessModel, outputPath string) error {
	return writeSVGFile(outputPath, e.GenerateSequenceDiagram(pm))
}

// ExportStateDiagram writes a UML State Machine SVG to outputPath.
func (e *Exporter) ExportStateDiagram(pm *model.ProcessModel, outputPath string) error {
	return writeSVGFile(outputPath, e.GenerateStateDiagram(pm))
}

// ExportERDiagram writes an Entity-Relationship SVG to outputPath.
func (e *Exporter) ExportERDiagram(pm *model.ProcessModel, outputPath string) error {
	return writeSVGFile(outputPath, e.GenerateERDiagram(pm))
}

func writeSVGFile(outputPath, content string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write SVG file %s: %w", outputPath, err)
	}
	return nil
}

type point struct {
	x, y float64
}

type nodeBox struct {
	x, y, w, h float64
}

// Generate renders the default SVG (Flowchart TD).
func (e *Exporter) Generate(pm *model.ProcessModel) string {
	return e.GenerateFlowchartTD(pm)
}

// GenerateFlowchart renders either a TD (top-to-bottom) or LR (left-to-right) flowchart SVG.
func (e *Exporter) GenerateFlowchart(pm *model.ProcessModel, direction string) string {
	if strings.ToUpper(strings.TrimSpace(direction)) == "LR" {
		return e.GenerateFlowchartLR(pm)
	}
	return e.GenerateFlowchartTD(pm)
}

// -------------------------------------------------------------------------
// 1. FLOWCHART TD (Top-to-Bottom Swimlane Columns)
// -------------------------------------------------------------------------

func (e *Exporter) GenerateFlowchartTD(pm *model.ProcessModel) string {
	maxTitleLen, _, maxLaneLen := measureModelText(pm)

	headerHeight := 45.0
	nodeHeight := 70.0
	nodeGap := 35.0
	margin := 40.0
	colGap := 60.0

	nodeW := 230.0
	if e.NoTruncate {
		nodeW = math.Max(260.0, float64(maxTitleLen)*7.8+30.0)
	} else {
		nodeW = math.Max(230.0, math.Min(360.0, float64(maxTitleLen)*7.5+30.0))
	}
	colWidth := math.Max(nodeW+30.0, float64(maxLaneLen)*8.0+40.0)

	lanes, laneSteps := collectLanesAndSteps(pm)

	descWrapLimit := int((nodeW - 20.0) / 6.5)
	if descWrapLimit < 35 {
		descWrapLimit = 35
	}

	// Calculate maximum lane height
	maxLaneHeight := 0.0
	for _, lane := range lanes {
		steps := laneSteps[lane.ID]
		h := headerHeight + 20.0
		for _, s := range steps {
			var descLines []string
			if s.Description != "" {
				desc := s.Description
				if !e.NoTruncate && len(desc) > descWrapLimit*2 {
					desc = e.truncate(desc, descWrapLimit*2)
				}
				descLines = wrapText(desc, descWrapLimit)
			}
			cardH := nodeHeight
			if len(descLines) > 1 {
				cardH += float64(len(descLines)-1) * 16.0
			}
			h += cardH + nodeGap
		}
		h += 20.0
		if h > maxLaneHeight {
			maxLaneHeight = h
		}
	}
	laneHeight := math.Max(headerHeight+100.0, maxLaneHeight)
	totalWidth := margin*2 + float64(len(lanes))*colWidth + float64(len(lanes)-1)*colGap
	totalHeight := margin*2 + laneHeight

	nodePositions := make(map[string]nodeBox)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="100%%" height="100%%">`+"\n", totalWidth, totalHeight))
	sb.WriteString(commonSVGDefs())
	sb.WriteString(commonSVGStyles())
	sb.WriteString(`<rect width="100%" height="100%" fill="#F9FAFB" />` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="28" class="title">%s (Flowchart TD)</text>`+"\n", margin, html.EscapeString(pm.Name)))

	// Draw Swimlane Columns
	for i, lane := range lanes {
		laneX := margin + float64(i)*(colWidth+colGap)
		laneY := margin + 20.0

		// Lane Container
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="8" fill="#F3F4F6" stroke="#E5E7EB" stroke-width="1.5" />`+"\n",
			laneX, laneY, colWidth, laneHeight))

		// Lane Header
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="8" fill="#E5E7EB" />`+"\n",
			laneX, laneY, colWidth, headerHeight))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="lane-title" text-anchor="middle">%s</text>`+"\n",
			laneX+colWidth/2, laneY+27, html.EscapeString(lane.Name)))

		// Draw Steps in this lane
		steps := laneSteps[lane.ID]
		curY := laneY + headerHeight + 20.0
		for _, step := range steps {
			nodeX := laneX + 15.0
			nodeY := curY

			var descLines []string
			if step.Description != "" {
				desc := step.Description
				if !e.NoTruncate && len(desc) > descWrapLimit*2 {
					desc = e.truncate(desc, descWrapLimit*2)
				}
				descLines = wrapText(desc, descWrapLimit)
			}

			cardH := nodeHeight
			if len(descLines) > 1 {
				cardH += float64(len(descLines)-1) * 16.0
			}

			nodePositions[step.ID] = nodeBox{x: nodeX, y: nodeY, w: nodeW, h: cardH}
			bgFill, strokeColor, badgeColor := getNodeColors(step.Type)

			// Step Card
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="6" fill="%s" stroke="%s" stroke-width="1.5" filter="url(#shadow)" />`+"\n",
				nodeX, nodeY, nodeW, cardH, bgFill, strokeColor))

			// Type Badge
			badgeText := step.Type
			if badgeText == "" {
				badgeText = "Step"
			}
			badgeWidth := float64(len(badgeText)*7 + 14)
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="15" rx="3" fill="%s" opacity="0.15" />`+"\n",
				nodeX+10, nodeY+8, badgeWidth, badgeColor))
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-badge" fill="%s">%s</text>`+"\n",
				nodeX+15, nodeY+19, badgeColor, html.EscapeString(badgeText)))

			// Step Name
			nameText := step.Name
			if !e.NoTruncate {
				maxChars := int((nodeW - 20.0) / 7.5)
				nameText = e.truncate(nameText, maxChars)
			}
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-title">%s</text>`+"\n",
				nodeX+10, nodeY+42, html.EscapeString(nameText)))

			// Description
			for lineIdx, line := range descLines {
				sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-desc">%s</text>`+"\n",
					nodeX+10, nodeY+58+float64(lineIdx)*16.0, html.EscapeString(line)))
			}

			curY += cardH + nodeGap
		}
	}

	// Draw Links
	drawLinks(&sb, pm.Links, nodePositions, false)

	sb.WriteString("</svg>\n")
	return sb.String()
}

// -------------------------------------------------------------------------
// 2. FLOWCHART LR (Horizontal Swimlane Bands)
// -------------------------------------------------------------------------

func (e *Exporter) GenerateFlowchartLR(pm *model.ProcessModel) string {
	maxTitleLen, maxDescLen, maxLaneLen := measureModelText(pm)

	headerWidth := math.Max(190.0, float64(maxLaneLen)*8.0+30.0)
	nodeWidth := 200.0
	if e.NoTruncate {
		nodeWidth = math.Max(240.0, math.Max(float64(maxTitleLen)*7.8+30.0, math.Min(340.0, float64(maxDescLen)*5.5+30.0)))
	} else {
		nodeWidth = math.Max(200.0, math.Min(300.0, float64(maxTitleLen)*7.5+30.0))
	}
	nodeHeight := 65.0
	nodeGapX := 50.0
	rowGap := 25.0
	margin := 40.0

	descWrapLimit := int((nodeWidth - 20.0) / 6.5)
	if descWrapLimit < 28 {
		descWrapLimit = 28
	}

	lanes, laneSteps := collectLanesAndSteps(pm)

	maxSteps := 1
	for _, l := range lanes {
		if c := len(laneSteps[l.ID]); c > maxSteps {
			maxSteps = c
		}
	}

	rowWidth := headerWidth + 30.0 + float64(maxSteps)*(nodeWidth+nodeGapX)
	totalWidth := margin*2 + rowWidth

	// Calculate dynamic row height for each lane
	laneRowHeights := make(map[string]float64)
	totalRowsHeight := 0.0
	for _, lane := range lanes {
		steps := laneSteps[lane.ID]
		maxCardH := nodeHeight
		for _, s := range steps {
			var descLines []string
			if s.Description != "" {
				desc := s.Description
				if !e.NoTruncate && len(desc) > descWrapLimit*2 {
					desc = e.truncate(desc, descWrapLimit*2)
				}
				descLines = wrapText(desc, descWrapLimit)
			}
			cardH := nodeHeight
			if len(descLines) > 1 {
				cardH += float64(len(descLines)-1) * 16.0
			}
			if cardH > maxCardH {
				maxCardH = cardH
			}
		}
		rH := math.Max(105.0, maxCardH+40.0)
		laneRowHeights[lane.ID] = rH
		totalRowsHeight += rH
	}

	totalHeight := margin*2 + totalRowsHeight + float64(len(lanes)-1)*rowGap + 20.0

	nodePositions := make(map[string]nodeBox)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="100%%" height="100%%">`+"\n", totalWidth, totalHeight))
	sb.WriteString(commonSVGDefs())
	sb.WriteString(commonSVGStyles())
	sb.WriteString(`<rect width="100%" height="100%" fill="#F9FAFB" />` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="28" class="title">%s (Flowchart LR)</text>`+"\n", margin, html.EscapeString(pm.Name)))

	// Draw Horizontal Swimlane Rows
	curLaneY := margin + 20.0
	for _, lane := range lanes {
		laneY := curLaneY
		laneRowH := laneRowHeights[lane.ID]
		curLaneY += laneRowH + rowGap
		laneX := margin

		// Row container
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="8" fill="#F3F4F6" stroke="#E5E7EB" stroke-width="1.5" />`+"\n",
			laneX, laneY, rowWidth, laneRowH))

		// Left Header Band
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="8" fill="#E5E7EB" />`+"\n",
			laneX, laneY, headerWidth, laneRowH))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="lane-title" text-anchor="middle">%s</text>`+"\n",
			laneX+headerWidth/2, laneY+laneRowH/2+5, html.EscapeString(e.truncate(lane.Name, int((headerWidth-20)/8.0)))))

		// Draw Steps horizontally across this row
		steps := laneSteps[lane.ID]
		for stepIdx, step := range steps {
			var descLines []string
			if step.Description != "" {
				desc := step.Description
				if !e.NoTruncate && len(desc) > descWrapLimit*2 {
					desc = e.truncate(desc, descWrapLimit*2)
				}
				descLines = wrapText(desc, descWrapLimit)
			}

			cardH := nodeHeight
			if len(descLines) > 1 {
				cardH += float64(len(descLines)-1) * 16.0
			}

			nodeX := laneX + headerWidth + 25.0 + float64(stepIdx)*(nodeWidth+nodeGapX)
			nodeY := laneY + (laneRowH-cardH)/2

			nodePositions[step.ID] = nodeBox{x: nodeX, y: nodeY, w: nodeWidth, h: cardH}
			bgFill, strokeColor, badgeColor := getNodeColors(step.Type)

			// Step Card
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="6" fill="%s" stroke="%s" stroke-width="1.5" filter="url(#shadow)" />`+"\n",
				nodeX, nodeY, nodeWidth, cardH, bgFill, strokeColor))

			// Badge
			badgeText := step.Type
			if badgeText == "" {
				badgeText = "Step"
			}
			badgeWidth := float64(len(badgeText)*6 + 14)
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="14" rx="3" fill="%s" opacity="0.15" />`+"\n",
				nodeX+8, nodeY+7, badgeWidth, badgeColor))
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-badge" fill="%s">%s</text>`+"\n",
				nodeX+12, nodeY+17, badgeColor, html.EscapeString(badgeText)))

			// Title
			nameText := step.Name
			if !e.NoTruncate {
				maxChars := int((nodeWidth - 16.0) / 7.5)
				nameText = e.truncate(nameText, maxChars)
			}
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-title">%s</text>`+"\n",
				nodeX+8, nodeY+36, html.EscapeString(nameText)))

			// Multi-line Description
			for lineIdx, line := range descLines {
				sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-desc">%s</text>`+"\n",
					nodeX+8, nodeY+52+float64(lineIdx)*15.0, html.EscapeString(line)))
			}
		}
	}

	// Draw Links (Horizontal Flow)
	drawLinks(&sb, pm.Links, nodePositions, true)

	sb.WriteString("</svg>\n")
	return sb.String()
}

// -------------------------------------------------------------------------
// 3. JOURNEY (Multi-Stage Milestone Progress Map)
// -------------------------------------------------------------------------

func (e *Exporter) GenerateJourney(pm *model.ProcessModel) string {
	maxTitleLen, maxDescLen, maxLaneLen := measureModelText(pm)

	cardWidth := 240.0
	if e.NoTruncate {
		cardWidth = math.Max(260.0, math.Max(float64(maxTitleLen)*7.8+30.0, math.Min(340.0, float64(maxDescLen)*5.5+30.0)))
	} else {
		cardWidth = math.Max(240.0, math.Min(340.0, float64(maxTitleLen)*7.5+30.0))
	}
	stageWidth := math.Max(cardWidth+30.0, float64(maxLaneLen)*8.0+40.0)
	cardW := stageWidth - 30.0
	stageGap := 65.0
	headerHeight := 75.0
	cardHeight := 70.0
	cardGap := 15.0
	margin := 40.0

	descWrapLimit := int((cardW - 24.0) / 6.5)
	if descWrapLimit < 30 {
		descWrapLimit = 30
	}

	lanes, laneSteps := collectLanesAndSteps(pm)

	// Calculate maximum stage height across all stages
	maxStageHeight := headerHeight + 100.0
	for _, lane := range lanes {
		steps := laneSteps[lane.ID]
		h := headerHeight + 15.0
		for _, s := range steps {
			var descLines []string
			if s.Description != "" {
				desc := s.Description
				if !e.NoTruncate && len(desc) > descWrapLimit*2 {
					desc = e.truncate(desc, descWrapLimit*2)
				}
				descLines = wrapText(desc, descWrapLimit)
			}
			cH := cardHeight
			if len(descLines) > 1 {
				cH += float64(len(descLines)-1) * 15.0
			}
			h += cH + cardGap
		}
		h += 15.0
		if h > maxStageHeight {
			maxStageHeight = h
		}
	}

	stageHeight := maxStageHeight
	totalWidth := margin*2 + float64(len(lanes))*stageWidth + float64(len(lanes)-1)*stageGap
	timelineHeight := 60.0
	totalHeight := margin*2 + stageHeight + timelineHeight + 20.0

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="100%%" height="100%%">`+"\n", totalWidth, totalHeight))
	sb.WriteString(commonSVGDefs())
	sb.WriteString(commonSVGStyles())
	sb.WriteString(`<rect width="100%" height="100%" fill="#F8FAFC" />` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="28" class="title">%s (User / Data Journey)</text>`+"\n", margin, html.EscapeString(pm.Name)))

	// Draw Stages
	for i, lane := range lanes {
		stageX := margin + float64(i)*(stageWidth+stageGap)
		stageY := margin + 25.0

		// Stage Outer Container
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="12" fill="#FFFFFF" stroke="#E2E8F0" stroke-width="1.8" filter="url(#shadow)" />`+"\n",
			stageX, stageY, stageWidth, stageHeight))

		// Stage Header
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="12" fill="#EEF2FF" />`+"\n",
			stageX, stageY, stageWidth, headerHeight))
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="12" fill="#EEF2FF" />`+"\n",
			stageX, stageY+headerHeight-12, stageWidth)) // flatten bottom radius of header

		// Stage Number Pill
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="68" height="20" rx="10" fill="#4F46E5" />`+"\n",
			stageX+15, stageY+14))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="11" font-weight="bold" fill="#FFFFFF">STAGE %d</text>`+"\n",
			stageX+26, stageY+28, i+1))

		// Stage Title & Count
		steps := laneSteps[lane.ID]
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="lane-title" font-size="14">%s</text>`+"\n",
			stageX+15, stageY+52, html.EscapeString(e.truncate(lane.Name, int((stageWidth-30)/8.0)))))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-desc" font-size="11">%d milestones in stage</text>`+"\n",
			stageX+stageWidth-15, stageY+28, len(steps)))

		// Step Cards
		curCardY := stageY + headerHeight + 15.0
		for stepIdx, step := range steps {
			var descLines []string
			if step.Description != "" {
				desc := step.Description
				if !e.NoTruncate && len(desc) > descWrapLimit*2 {
					desc = e.truncate(desc, descWrapLimit*2)
				}
				descLines = wrapText(desc, descWrapLimit)
			}
			cH := cardHeight
			if len(descLines) > 1 {
				cH += float64(len(descLines)-1) * 15.0
			}

			cardX := stageX + 15.0
			cardY := curCardY

			_, strokeColor, badgeColor := getNodeColors(step.Type)

			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="6" fill="#F8FAFC" stroke="#E2E8F0" stroke-width="1.2" />`+"\n",
				cardX, cardY, cardW, cH))
			// Left accent stripe
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="4" height="%.0f" rx="2" fill="%s" />`+"\n",
				cardX, cardY, cH, strokeColor))

			// Milestone index badge
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="10" font-weight="bold" fill="%s">#%d  [%s]</text>`+"\n",
				cardX+12, cardY+18, badgeColor, stepIdx+1, html.EscapeString(step.Type)))

			// Name and Description
			nameText := step.Name
			if !e.NoTruncate {
				maxChars := int((cardW - 24.0) / 7.5)
				nameText = e.truncate(nameText, maxChars)
			}
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-title" font-size="12">%s</text>`+"\n",
				cardX+12, cardY+38, html.EscapeString(nameText)))

			for lineIdx, line := range descLines {
				sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-desc">%s</text>`+"\n",
					cardX+12, cardY+54+float64(lineIdx)*15.0, html.EscapeString(line)))
			}

			curCardY += cH + cardGap
		}

		// Milestone Arrow between stages
		if i < len(lanes)-1 {
			arrowX := stageX + stageWidth + 8.0
			arrowY := stageY + headerHeight/2
			arrowW := stageGap - 16.0

			sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#6366F1" stroke-width="3" stroke-linecap="round" marker-end="url(#milestone-arrow)" />`+"\n",
				arrowX, arrowY, arrowX+arrowW, arrowY))
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="34" height="16" rx="4" fill="#EEF2FF" />`+"\n",
				arrowX+arrowW/2-17, arrowY-20))
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="10" font-weight="bold" fill="#4F46E5" text-anchor="middle">==&gt;</text>`+"\n",
				arrowX+arrowW/2, arrowY-8))
		}
	}

	// Bottom Journey Timeline Progress Track
	trackY := margin + stageHeight + 40.0
	sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#CBD5E1" stroke-width="4" stroke-linecap="round" />`+"\n",
		margin+stageWidth/2, trackY, totalWidth-margin-stageWidth/2, trackY))

	for i, lane := range lanes {
		dotX := margin + float64(i)*(stageWidth+stageGap) + stageWidth/2
		sb.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="9" fill="#4F46E5" stroke="#FFFFFF" stroke-width="3" />`+"\n",
			dotX, trackY))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="11" font-weight="600" fill="#334155" text-anchor="middle">Stage %d: %s</text>`+"\n",
			dotX, trackY+22, i+1, html.EscapeString(e.truncate(lane.Name, int((stageWidth-20)/8.0)))))
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// -------------------------------------------------------------------------
// 4. SEQUENCE DIAGRAM (Lifelines & Numbered Messages)
// -------------------------------------------------------------------------

func (e *Exporter) GenerateSequenceDiagram(pm *model.ProcessModel) string {
	maxTitleLen, _, maxLaneLen := measureModelText(pm)

	pWidth := 200.0
	if e.NoTruncate {
		pWidth = math.Max(220.0, float64(maxLaneLen)*8.0+40.0)
	} else {
		pWidth = math.Max(200.0, math.Min(320.0, float64(maxLaneLen)*7.8+30.0))
	}
	pGap := math.Max(80.0, pWidth/2.5)
	margin := 50.0
	stepGapY := 60.0
	pHeaderHeight := 45.0

	lanes, _ := collectLanesAndSteps(pm)

	numInteractions := len(pm.Links)
	if numInteractions == 0 {
		numInteractions = len(pm.Steps)
	}
	if numInteractions == 0 {
		numInteractions = 1
	}

	lifelineHeight := float64(numInteractions)*stepGapY + 80.0
	totalWidth := margin*2 + float64(len(lanes))*pWidth + float64(len(lanes)-1)*pGap
	totalHeight := margin*2 + pHeaderHeight*2 + lifelineHeight + 40.0

	// Map lane ID to lifeline center X
	laneCenters := make(map[string]float64)
	stepToLane := make(map[string]string)
	stepNames := make(map[string]string)
	for _, s := range pm.Steps {
		stepToLane[s.ID] = s.SwimlaneID
		stepNames[s.ID] = s.Name
	}

	for i, l := range lanes {
		laneCenters[l.ID] = margin + float64(i)*(pWidth+pGap) + pWidth/2
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="100%%" height="100%%">`+"\n", totalWidth, totalHeight))
	sb.WriteString(commonSVGDefs())
	sb.WriteString(commonSVGStyles())
	sb.WriteString(`<rect width="100%" height="100%" fill="#F8FAFC" />` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="28" class="title">%s (Sequence Diagram)</text>`+"\n", margin, html.EscapeString(pm.Name)))

	topY := margin + 25.0
	bottomLifelineY := topY + pHeaderHeight + lifelineHeight

	// Draw Participants & Lifelines
	for i, lane := range lanes {
		pX := margin + float64(i)*(pWidth+pGap)
		pCenterX := laneCenters[lane.ID]

		// Vertical Dashed Lifeline
		sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#94A3B8" stroke-dasharray="6,4" stroke-width="1.8" />`+"\n",
			pCenterX, topY+pHeaderHeight, pCenterX, bottomLifelineY))

		// Top Participant Box
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="8" fill="#1E293B" stroke="#0F172A" filter="url(#shadow)" />`+"\n",
			pX, topY, pWidth, pHeaderHeight))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="13" font-weight="600" fill="#FFFFFF" text-anchor="middle">%s</text>`+"\n",
			pCenterX, topY+28, html.EscapeString(e.truncate(lane.Name, int((pWidth-20)/8.0)))))

		// Bottom Participant Box
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="8" fill="#1E293B" stroke="#0F172A" filter="url(#shadow)" />`+"\n",
			pX, bottomLifelineY, pWidth, pHeaderHeight))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="13" font-weight="600" fill="#FFFFFF" text-anchor="middle">%s</text>`+"\n",
			pCenterX, bottomLifelineY+28, html.EscapeString(e.truncate(lane.Name, int((pWidth-20)/8.0)))))
	}

	// Draw Messages along the timeline
	if len(pm.Links) > 0 {
		for linkIdx, link := range pm.Links {
			msgY := topY + pHeaderHeight + 40.0 + float64(linkIdx)*stepGapY
			srcLane := stepToLane[link.SourceStepID]
			tgtLane := stepToLane[link.TargetStepID]

			srcX := laneCenters[srcLane]
			tgtX := laneCenters[tgtLane]
			if srcX == 0 {
				srcX = margin + pWidth/2
			}
			if tgtX == 0 {
				tgtX = margin + pWidth/2
			}

			msgLabel := link.Label
			if msgLabel == "" {
				msgLabel = stepNames[link.TargetStepID]
			}
			if msgLabel == "" {
				msgLabel = fmt.Sprintf("Call %s", link.TargetStepID)
			}

			maxMsgChars := 28
			if e.NoTruncate {
				maxMsgChars = max(40, maxTitleLen)
			}
			msgToShow := e.truncate(msgLabel, maxMsgChars)

			if math.Abs(srcX-tgtX) < 10 {
				// Self Call / Loop
				rectWidth := float64(len(msgToShow)*6 + 28)
				sb.WriteString(fmt.Sprintf(`<path d="M %.0f %.0f L %.0f %.0f L %.0f %.0f L %.0f %.0f" fill="none" stroke="#2563EB" stroke-width="1.8" marker-end="url(#seq-arrow)" />`+"\n",
					srcX, msgY, srcX+45, msgY, srcX+45, msgY+24, srcX, msgY+24))
				sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="16" rx="3" fill="#EFF6FF" opacity="0.95" />`+"\n",
					srcX+52, msgY+4, rectWidth))
				sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="edge-label" font-weight="600" fill="#1D4ED8">%d. %s</text>`+"\n",
					srcX+56, msgY+16, linkIdx+1, html.EscapeString(msgToShow)))
			} else {
				// Call from srcX to tgtX
				strokeColor := "#2563EB"
				marker := "url(#seq-arrow)"
				dash := ""
				if link.IsCrossService {
					strokeColor = "#DC2626"
					marker = "url(#cross-arrow)"
					dash = `stroke-dasharray="5,4"`
				}

				// Activation Bar on target lifeline
				sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="12" height="26" rx="2" fill="#BFDBFE" stroke="#3B82F6" stroke-width="1" />`+"\n",
					tgtX-6, msgY-8))

				// Message Arrow
				sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="2" marker-end="%s" %s />`+"\n",
					srcX, msgY, tgtX, msgY, strokeColor, marker, dash))

				// Numbered Message Card
				midX := (srcX + tgtX) / 2
				cardWidth := float64(len(msgToShow)*6 + 32)
				sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="18" rx="4" fill="#FFFFFF" stroke="#E2E8F0" filter="url(#shadow)" />`+"\n",
					midX-cardWidth/2, msgY-20, cardWidth))
				sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="11" font-weight="600" fill="#1E293B" text-anchor="middle">%d. %s</text>`+"\n",
					midX, msgY-7, linkIdx+1, html.EscapeString(msgToShow)))
			}
		}
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// -------------------------------------------------------------------------
// 5. STATE DIAGRAM (UML State Machine)
// -------------------------------------------------------------------------

func (e *Exporter) GenerateStateDiagram(pm *model.ProcessModel) string {
	maxTitleLen, _, maxLaneLen := measureModelText(pm)

	stateWidth := 270.0
	if e.NoTruncate {
		stateWidth = math.Max(280.0, math.Max(float64(maxLaneLen)*8.0+40.0, float64(maxTitleLen)*7.8+50.0))
	} else {
		stateWidth = math.Max(270.0, math.Min(380.0, math.Max(float64(maxLaneLen)*7.8+30.0, float64(maxTitleLen)*7.5+30.0)))
	}
	stateHeight := 140.0
	stateGap := 70.0
	margin := 50.0

	lanes, laneSteps := collectLanesAndSteps(pm)

	totalWidth := margin*2 + 80.0 + float64(len(lanes))*stateWidth + float64(len(lanes))*stateGap + 80.0
	totalHeight := margin*2 + stateHeight + 80.0

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="100%%" height="100%%">`+"\n", totalWidth, totalHeight))
	sb.WriteString(commonSVGDefs())
	sb.WriteString(commonSVGStyles())
	sb.WriteString(`<rect width="100%" height="100%" fill="#F8FAFC" />` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="28" class="title">%s (State Diagram)</text>`+"\n", margin, html.EscapeString(pm.Name)))

	centerY := margin + 30.0 + stateHeight/2

	// Initial State [*]
	startX := margin + 25.0
	sb.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="14" fill="#1F2937" filter="url(#shadow)" />`+"\n", startX, centerY))
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-badge" fill="#64748B" text-anchor="middle">[*] Start</text>`+"\n", startX, centerY+28))

	// Arrow from Start to First State
	firstStateX := margin + 90.0
	firstStateY := margin + 30.0
	sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#4B5563" stroke-width="2" marker-end="url(#arrow)" />`+"\n",
		startX+14, centerY, firstStateX, centerY))

	// Draw State Containers
	for i, lane := range lanes {
		sX := margin + 90.0 + float64(i)*(stateWidth+stateGap)
		sY := firstStateY

		// State Rounded Outer Card
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="16" fill="#FFFFFF" stroke="#64748B" stroke-width="2" filter="url(#shadow)" />`+"\n",
			sX, sY, stateWidth, stateHeight))

		// State Header
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="34" rx="16" fill="#F1F5F9" />`+"\n",
			sX, sY, stateWidth))
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="10" fill="#F1F5F9" />`+"\n",
			sX, sY+24, stateWidth))
		sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#CBD5E1" stroke-width="1.5" />`+"\n",
			sX, sY+34, sX+stateWidth, sY+34))

		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="12" font-weight="bold" fill="#1E293B" text-anchor="middle">%s</text>`+"\n",
			sX+stateWidth/2, sY+22, html.EscapeString(e.truncate(lane.Name, int((stateWidth-30)/8.0)))))

		// Internal Activities inside the State
		steps := laneSteps[lane.ID]
		for stepIdx, step := range steps {
			if stepIdx >= 4 {
				sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="10" fill="#94A3B8">... +%d more</text>`+"\n",
					sX+16, sY+52+float64(stepIdx)*18, len(steps)-4))
				break
			}
			prefix := "do /"
			if stepIdx == 0 {
				prefix = "entry /"
			} else if stepIdx == len(steps)-1 {
				prefix = "exit /"
			}
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="10" fill="#475569"><tspan font-weight="600" fill="#2563EB">%s</tspan> %s</text>`+"\n",
				sX+16, sY+52+float64(stepIdx)*18, prefix, html.EscapeString(e.truncate(step.Name, int((stateWidth-45)/7.5)))))
		}

		// Transition Arrow to next state or end
		nextX := sX + stateWidth + stateGap
		if i < len(lanes)-1 {
			sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#4B5563" stroke-width="2" marker-end="url(#arrow)" />`+"\n",
				sX+stateWidth, centerY, nextX, centerY))
			// Transition pill badge
			midX := sX + stateWidth + stateGap/2
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="54" height="18" rx="9" fill="#EEF2FF" stroke="#C7D2FE" />`+"\n",
				midX-27, centerY-22))
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="9" font-weight="bold" fill="#4F46E5" text-anchor="middle">[next]</text>`+"\n",
				midX, centerY-10))
		} else {
			// Transition to End Node [*]
			endNodeX := sX + stateWidth + 50.0
			sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#4B5563" stroke-width="2" marker-end="url(#arrow)" />`+"\n",
				sX+stateWidth, centerY, endNodeX-18, centerY))

			// Bullseye End State
			sb.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="16" fill="none" stroke="#1F2937" stroke-width="2.5" />`+"\n",
				endNodeX, centerY))
			sb.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="10" fill="#1F2937" />`+"\n",
				endNodeX, centerY))
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="node-badge" fill="#64748B" text-anchor="middle">[*] End</text>`+"\n",
				endNodeX, centerY+30))
		}
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// -------------------------------------------------------------------------
// 6. ER DIAGRAM (Entity-Relationship Layout)
// -------------------------------------------------------------------------

func (e *Exporter) GenerateERDiagram(pm *model.ProcessModel) string {
	maxTitleLen, _, _ := measureModelText(pm)

	boxWidth := 250.0
	if e.NoTruncate {
		boxWidth = math.Max(260.0, float64(maxTitleLen)*8.0+40.0)
	} else {
		boxWidth = math.Max(250.0, math.Min(360.0, float64(maxTitleLen)*7.5+30.0))
	}
	boxHeight := 130.0
	boxGapX := 80.0
	boxGapY := 60.0
	margin := 50.0

	// Collect entities: prioritize database tables & distinct steps
	type erEntity struct {
		ID         string
		Name       string
		Kind       string
		Attributes []string
	}

	var entities []erEntity
	seenEntities := make(map[string]bool)

	// Check if there are explicit tables or struct models
	hasExplicitEntities := false
	for _, s := range pm.Steps {
		if s.Type == "DatabaseTable" || s.Metadata["table"] != nil || s.Metadata["columns"] != nil {
			hasExplicitEntities = true
			break
		}
	}

	for _, s := range pm.Steps {
		if hasExplicitEntities && s.Type != "DatabaseTable" && s.Metadata["table"] == nil && s.Metadata["columns"] == nil {
			continue
		}

		entName := s.Name
		if tbl, ok := s.Metadata["table"].(string); ok && tbl != "" {
			entName = tbl
		}
		if seenEntities[entName] {
			continue
		}
		seenEntities[entName] = true

		kind := "TABLE"
		if isStruct, ok := s.Metadata["is_struct"].(bool); ok && isStruct {
			kind = "STRUCT"
		} else if s.Type != "DatabaseTable" && s.Metadata["table"] == nil {
			kind = "ENTITY"
		}

		var attrs []string
		if colsRaw, ok := s.Metadata["columns"].([]map[string]string); ok && len(colsRaw) > 0 {
			for _, col := range colsRaw {
				prefix := "    "
				if col["key"] == "PK" {
					prefix = "PK  "
				}
				attrs = append(attrs, fmt.Sprintf("%s%s : %s", prefix, col["name"], col["type"]))
			}
		} else {
			attrs = []string{
				"PK  id : bigint",
				"    created_at : datetime",
			}
			if s.Type == "DatabaseTable" || s.Type == "TableOperation" || strings.Contains(s.Name, ".") {
				attrs = append(attrs, "    status : varchar(50)", "    payload : nvarchar(max)")
			} else {
				attrs = append(attrs, fmt.Sprintf("    type : %s", s.Type), "    status : varchar(20)")
			}
		}

		entities = append(entities, erEntity{
			ID:         s.ID,
			Name:       entName,
			Kind:       kind,
			Attributes: attrs,
		})
	}

	if len(entities) == 0 {
		entities = append(entities, erEntity{
			ID:         "process",
			Name:       pm.Name,
			Kind:       "PROCESS",
			Attributes: []string{"PK id : uuid", "   name : text"},
		})
	}

	maxAttrs := 4
	for _, ent := range entities {
		if len(ent.Attributes) > maxAttrs {
			maxAttrs = len(ent.Attributes)
		}
	}
	boxHeight = math.Max(130.0, float64(maxAttrs)*18.0+50.0)

	// Layout in 2 columns if many entities
	cols := 2
	if len(entities) <= 3 {
		cols = len(entities)
	}
	if cols == 0 {
		cols = 1
	}
	rows := (len(entities) + cols - 1) / cols

	totalWidth := margin*2 + float64(cols)*boxWidth + float64(cols-1)*boxGapX
	totalHeight := margin*2 + float64(rows)*boxHeight + float64(rows-1)*boxGapY + 30.0

	entPositions := make(map[string]point)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="100%%" height="100%%">`+"\n", totalWidth, totalHeight))
	sb.WriteString(commonSVGDefs())
	sb.WriteString(commonSVGStyles())
	sb.WriteString(`<rect width="100%" height="100%" fill="#F8FAFC" />` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="28" class="title">%s (ER Diagram)</text>`+"\n", margin, html.EscapeString(pm.Name)))

	// Draw Entities
	for idx, ent := range entities {
		col := idx % cols
		row := idx / cols

		bX := margin + float64(col)*(boxWidth+boxGapX)
		bY := margin + 30.0 + float64(row)*(boxHeight+boxGapY)

		entPositions[ent.ID] = point{x: bX + boxWidth/2, y: bY + boxHeight/2}

		// Entity Card
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="6" fill="#FFFFFF" stroke="#059669" stroke-width="1.8" filter="url(#shadow)" />`+"\n",
			bX, bY, boxWidth, boxHeight))

		// Entity Header
		headerFill := "#059669"
		if ent.Kind == "ENTITY" {
			headerFill = "#2563EB"
		}
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="30" rx="6" fill="%s" />`+"\n",
			bX, bY, boxWidth, headerFill))
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="8" fill="%s" />`+"\n",
			bX, bY+22, boxWidth, headerFill))

		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="11" font-weight="bold" fill="#FFFFFF">[%s]  %s</text>`+"\n",
			bX+10, bY+20, ent.Kind, html.EscapeString(e.truncate(ent.Name, int((boxWidth-40)/8.0)))))

		// Attribute rows
		for aIdx, attr := range ent.Attributes {
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="'Consolas', monospace" font-size="10" fill="#334155">%s</text>`+"\n",
				bX+10, bY+50+float64(aIdx)*18, html.EscapeString(attr)))
		}
	}

	// Draw Relationships from links
	for _, l := range pm.Links {
		p1, ok1 := entPositions[l.SourceStepID]
		p2, ok2 := entPositions[l.TargetStepID]
		if !ok1 || !ok2 || l.SourceStepID == l.TargetStepID {
			continue
		}

		sb.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#059669" stroke-width="1.8" stroke-dasharray="4,3" marker-end="url(#arrow)" />`+"\n",
			p1.x, p1.y, p2.x, p2.y))

		// Relationship label
		lbl := l.Label
		if lbl == "" {
			lbl = "relates to"
		}
		midX := (p1.x + p2.x) / 2
		midY := (p1.y + p2.y) / 2
		sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="15" rx="3" fill="#ECFDF5" stroke="#A7F3D0" />`+"\n",
			midX-float64(len(lbl)*3+6), midY-8, float64(len(lbl)*6+12)))
		sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" font-family="-apple-system, sans-serif" font-size="9" font-weight="bold" fill="#047857" text-anchor="middle">%s</text>`+"\n",
			midX, midY+3, html.EscapeString(lbl)))
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// -------------------------------------------------------------------------
// Helpers: SVG Defs, Styles, Lanes, Colors & Links
// -------------------------------------------------------------------------

func commonSVGDefs() string {
	return `<defs>
  <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
    <path d="M 0 1 L 10 5 L 0 9 z" fill="#4B5563" />
  </marker>
  <marker id="seq-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
    <path d="M 0 1 L 10 5 L 0 9 z" fill="#2563EB" />
  </marker>
  <marker id="cross-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
    <path d="M 0 1 L 10 5 L 0 9 z" fill="#EF4444" />
  </marker>
  <marker id="milestone-arrow" viewBox="0 0 12 12" refX="9" refY="6" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
    <path d="M 0 1 L 12 6 L 0 11 z" fill="#6366F1" />
  </marker>
  <filter id="shadow" x="-5%" y="-5%" width="115%" height="115%">
    <feDropShadow dx="0" dy="2" stdDeviation="3" flood-opacity="0.1" />
  </filter>
</defs>
`
}

func commonSVGStyles() string {
	return `<style>
  .title { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 18px; font-weight: bold; fill: #1F2937; }
  .lane-title { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 14px; font-weight: 600; fill: #374151; }
  .node-title { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 13px; font-weight: 600; fill: #111827; }
  .node-desc { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 11px; fill: #6B7280; }
  .node-badge { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 9px; font-weight: bold; }
  .edge-label { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 10px; fill: #4B5563; }
  .state-label { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 11px; font-weight: 600; }
</style>
`
}

func collectLanesAndSteps(pm *model.ProcessModel) ([]model.Swimlane, map[string][]model.Step) {
	laneSteps := make(map[string][]model.Step)
	for _, step := range pm.Steps {
		laneSteps[step.SwimlaneID] = append(laneSteps[step.SwimlaneID], step)
	}

	var lanes []model.Swimlane
	for _, lane := range pm.Swimlanes {
		if len(laneSteps[lane.ID]) > 0 {
			lanes = append(lanes, lane)
		}
	}

	var unassigned []model.Step
	for _, s := range pm.Steps {
		if pm.FindSwimlane(s.SwimlaneID) == nil {
			unassigned = append(unassigned, s)
		}
	}
	if len(unassigned) > 0 {
		generalLane := model.Swimlane{ID: "general_lane", Name: "General Process"}
		lanes = append(lanes, generalLane)
		laneSteps[generalLane.ID] = unassigned
	}

	if len(lanes) == 0 {
		lanes = append(lanes, model.Swimlane{ID: "default", Name: pm.Name})
	}

	return lanes, laneSteps
}

func drawLinks(sb *strings.Builder, links []model.Link, nodePositions map[string]nodeBox, isHorizontal bool) {
	for _, link := range links {
		srcBox, okSrc := nodePositions[link.SourceStepID]
		tgtBox, okTgt := nodePositions[link.TargetStepID]
		if !okSrc || !okTgt {
			continue
		}

		var start, end point
		var pathD string

		strokeColor := "#4B5563"
		marker := "url(#arrow)"
		dash := ""
		if link.IsCrossService {
			strokeColor = "#EF4444"
			marker = "url(#cross-arrow)"
			dash = `stroke-dasharray="5,4"`
		}

		if isHorizontal {
			// Horizontal flow
			start = point{srcBox.x + srcBox.w, srcBox.y + srcBox.h/2}
			end = point{tgtBox.x, tgtBox.y + tgtBox.h/2}
			if start.x < end.x {
				midX := (start.x + end.x) / 2
				pathD = fmt.Sprintf("M %.0f %.0f C %.0f %.0f, %.0f %.0f, %.0f %.0f",
					start.x, start.y, midX, start.y, midX, end.y, end.x, end.y)
			} else {
				// Loop back
				loopY := math.Max(start.y, end.y) + 40.0
				pathD = fmt.Sprintf("M %.0f %.0f C %.0f %.0f, %.0f %.0f, %.0f %.0f",
					start.x, start.y, start.x+30, loopY, end.x-30, loopY, end.x, end.y)
			}
		} else {
			// Vertical / TD flow
			if math.Abs(srcBox.x-tgtBox.x) < 5 {
				start = point{srcBox.x + srcBox.w/2, srcBox.y + srcBox.h}
				end = point{tgtBox.x + tgtBox.w/2, tgtBox.y}
				pathD = fmt.Sprintf("M %.0f %.0f L %.0f %.0f", start.x, start.y, end.x, end.y)
			} else if srcBox.x < tgtBox.x {
				start = point{srcBox.x + srcBox.w, srcBox.y + srcBox.h/2}
				end = point{tgtBox.x, tgtBox.y + tgtBox.h/2}
				midX := (start.x + end.x) / 2
				pathD = fmt.Sprintf("M %.0f %.0f C %.0f %.0f, %.0f %.0f, %.0f %.0f",
					start.x, start.y, midX, start.y, midX, end.y, end.x, end.y)
			} else {
				start = point{srcBox.x, srcBox.y + srcBox.h/2}
				end = point{tgtBox.x + tgtBox.w, tgtBox.y + tgtBox.h/2}
				midX := (start.x + end.x) / 2
				pathD = fmt.Sprintf("M %.0f %.0f C %.0f %.0f, %.0f %.0f, %.0f %.0f",
					start.x, start.y, midX, start.y, midX, end.y, end.x, end.y)
			}
		}

		sb.WriteString(fmt.Sprintf(`<path d="%s" fill="none" stroke="%s" stroke-width="1.8" marker-end="%s" %s />`+"\n",
			pathD, strokeColor, marker, dash))

		edgeLabel := link.Label
		if link.Condition != "" {
			if edgeLabel != "" && !strings.Contains(edgeLabel, link.Condition) {
				edgeLabel = fmt.Sprintf("[%s] %s", link.Condition, edgeLabel)
			} else if edgeLabel == "" {
				edgeLabel = fmt.Sprintf("[%s]", link.Condition)
			}
		}

		if edgeLabel != "" {
			midX := (start.x + end.x) / 2
			midY := (start.y + end.y) / 2
			lblW := float64(len(edgeLabel)*6 + 10)
			sb.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="%.0f" height="16" rx="3" fill="#FFFFFF" opacity="0.9" />`+"\n",
				midX-lblW/2, midY-10, lblW))
			sb.WriteString(fmt.Sprintf(`<text x="%.0f" y="%.0f" class="edge-label" text-anchor="middle">%s</text>`+"\n",
				midX, midY+2, html.EscapeString(edgeLabel)))
		}
	}
}

func getNodeColors(stepType string) (bg, stroke, badge string) {
	switch stepType {
	case "Endpoint":
		return "#FFFFFF", "#3B82F6", "#2563EB" // Blue
	case "DatabaseQuery", "TableOperation", "DatabaseTable":
		return "#FFFFFF", "#10B981", "#059669" // Green
	case "StoredProcedure", "View":
		return "#FFFFFF", "#8B5CF6", "#7C3AED" // Purple
	case "Task", "WorkflowStep":
		return "#FFFFFF", "#F59E0B", "#D97706" // Amber
	case "Script":
		return "#FFFFFF", "#6366F1", "#4F46E5" // Indigo
	case "Assertion", "Condition":
		return "#FFFFFF", "#EC4899", "#DB2777" // Pink
	default:
		return "#FFFFFF", "#9CA3AF", "#4B5563" // Slate
	}
}

func (e *Exporter) truncate(s string, max int) string {
	if e.NoTruncate {
		return strings.TrimSpace(s)
	}
	s = strings.TrimSpace(s)
	if max > 0 && len(s) > max {
		if max <= 3 {
			return s[:max]
		}
		return s[:max-3] + "..."
	}
	return s
}

func truncateString(s string, max int) string {
	s = strings.TrimSpace(s)
	if max > 0 && len(s) > max {
		if max <= 3 {
			return s[:max]
		}
		return s[:max-3] + "..."
	}
	return s
}

func wrapText(text string, maxCharsPerLine int) []string {
	if maxCharsPerLine <= 0 {
		maxCharsPerLine = 45
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= maxCharsPerLine {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) <= maxCharsPerLine {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	lines = append(lines, cur)
	return lines
}

func measureModelText(pm *model.ProcessModel) (maxTitleLen, maxDescLen, maxLaneLen int) {
	maxTitleLen = 20
	maxDescLen = 24
	maxLaneLen = 20
	for _, step := range pm.Steps {
		if len(step.Name) > maxTitleLen {
			maxTitleLen = len(step.Name)
		}
		if len(step.Description) > maxDescLen {
			maxDescLen = len(step.Description)
		}
	}
	for _, lane := range pm.Swimlanes {
		if len(lane.Name) > maxLaneLen {
			maxLaneLen = len(lane.Name)
		}
	}
	return
}
