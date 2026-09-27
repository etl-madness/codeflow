package excel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"codeflow/pkg/model"
)

// Exporter converts a ProcessModel into Visio Data Visualizer compatible workbooks.
type Exporter struct{}

// New creates a new Excel Exporter.
func New() *Exporter {
	return &Exporter{}
}

// Export creates a standard single-sheet Visio Data Visualizer compatible Excel file at outputPath.
func (e *Exporter) Export(pm *model.ProcessModel, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Process Map"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}

	if sheetName != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}
	f.SetActiveSheet(index)

	// Microsoft Visio Data Visualizer Standard Schema Headers
	headers := []string{
		"Process Step ID",
		"Step Description",
		"Next Step ID",
		"Connector Label",
		"Step Type",
		"Owner / Function",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
			Size:  11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#2F5597"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create header style: %w", err)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
		_ = f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	outgoingLinks := make(map[string][]model.Link)
	for _, link := range pm.Links {
		outgoingLinks[link.SourceStepID] = append(outgoingLinks[link.SourceStepID], link)
	}

	rowStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
	})

	for rowIdx, step := range pm.Steps {
		row := rowIdx + 2

		var nextIDs []string
		var labels []string
		for _, l := range outgoingLinks[step.ID] {
			nextIDs = append(nextIDs, l.TargetStepID)
			label := l.Label
			if l.Condition != "" {
				if label != "" {
					label = fmt.Sprintf("[%s] %s", l.Condition, label)
				} else {
					label = fmt.Sprintf("[%s]", l.Condition)
				}
			}
			labels = append(labels, label)
		}

		nextStepsStr := strings.Join(nextIDs, ", ")
		connectorLabelStr := strings.Join(labels, ", ")

		owner := "General"
		if lane := pm.FindSwimlane(step.SwimlaneID); lane != nil {
			owner = lane.Name
		}

		desc := step.Description
		if desc == "" {
			desc = step.Name
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), step.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), desc)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), nextStepsStr)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), connectorLabelStr)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), step.Type)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), owner)

		for c := 1; c <= 6; c++ {
			cell, _ := excelize.CoordinatesToCellName(c, row)
			_ = f.SetCellStyle(sheetName, cell, cell, rowStyle)
		}
	}

	_ = f.SetColWidth(sheetName, "A", "A", 25)
	_ = f.SetColWidth(sheetName, "B", "B", 35)
	_ = f.SetColWidth(sheetName, "C", "C", 25)
	_ = f.SetColWidth(sheetName, "D", "D", 20)
	_ = f.SetColWidth(sheetName, "E", "E", 18)
	_ = f.SetColWidth(sheetName, "F", "F", 25)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("failed to save excel file %s: %w", outputPath, err)
	}

	return nil
}

// ExportCustomWorkbook creates an interactive, multi-sheet Excel Data Visualizer custom workbook.
// This workbook provides an in-Excel visual process navigator and complete dependency traceability
// that functions without requiring the retired Microsoft Visio Data Visualizer add-in.
func (e *Exporter) ExportCustomWorkbook(pm *model.ProcessModel, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	// 1. Dashboard & KPI Sheet
	if err := e.buildDashboardSheet(f, pm); err != nil {
		return err
	}

	// 2. Interactive Visual Process Map Sheet (In-Excel Swimlane Layout)
	if err := e.buildVisualMapSheet(f, pm); err != nil {
		return err
	}

	// 3. Data Visualizer Schema Sheet (Standard Table)
	if err := e.buildSchemaSheet(f, pm); err != nil {
		return err
	}

	// 4. Traceability & Dependencies Matrix Sheet
	if err := e.buildTraceabilitySheet(f, pm); err != nil {
		return err
	}

	// 5. Swimlanes & Services Sheet
	if err := e.buildServicesSheet(f, pm); err != nil {
		return err
	}

	_ = f.DeleteSheet("Sheet1")

	// Set Dashboard as active sheet
	dashIdx, _ := f.GetSheetIndex("Dashboard")
	f.SetActiveSheet(dashIdx)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("failed to save custom workbook %s: %w", outputPath, err)
	}

	return nil
}

func (e *Exporter) buildDashboardSheet(f *excelize.File, pm *model.ProcessModel) error {
	sheet := "Dashboard"
	_, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}

	// Title Banner
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16, Color: "#FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	_ = f.MergeCell(sheet, "A1", "H2")
	_ = f.SetCellValue(sheet, "A1", fmt.Sprintf("CodeFlow - Excel Data Visualizer: %s", pm.Name))
	_ = f.SetCellStyle(sheet, "A1", "H2", titleStyle)

	// KPI Cards
	crossLinks := 0
	for _, l := range pm.Links {
		if l.IsCrossService {
			crossLinks++
		}
	}

	cardHeaders := []string{"Total Steps", "Swimlanes / Services", "Cross-Service Links", "Total Connections"}
	cardValues := []int{len(pm.Steps), len(pm.Swimlanes), crossLinks, len(pm.Links)}
	cardColors := []string{"#D9E1F2", "#E2EFDA", "#FCE4D6", "#FFF2CC"}

	for i := 0; i < 4; i++ {
		startCol, _ := excelize.CoordinatesToCellName(i*2+1, 4)
		endCol, _ := excelize.CoordinatesToCellName(i*2+2, 4)
		valCol, _ := excelize.CoordinatesToCellName(i*2+1, 5)
		valEndCol, _ := excelize.CoordinatesToCellName(i*2+2, 5)

		_ = f.MergeCell(sheet, startCol, endCol)
		_ = f.SetCellValue(sheet, startCol, cardHeaders[i])

		_ = f.MergeCell(sheet, valCol, valEndCol)
		_ = f.SetCellValue(sheet, valCol, cardValues[i])

		cHeaderStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 10, Color: "#595959"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{cardColors[i]}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		cValStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 18, Color: "#262626"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{cardColors[i]}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})

		_ = f.SetCellStyle(sheet, startCol, endCol, cHeaderStyle)
		_ = f.SetCellStyle(sheet, valCol, valEndCol, cValStyle)
	}

	// Instructions Section
	infoStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10, Color: "#333333"},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	_ = f.MergeCell(sheet, "A7", "H8")
	instructions := "Welcome to your Custom Data Visualizer Workbook! This workbook provides full architectural visualization and traceability:\n" +
		"• 'Visual Process Map' renders an in-Excel graphical swimlane board.\n" +
		"• 'Process Steps' provides the complete Data Visualizer schema table with sorting and filtering.\n" +
		"• 'Traceability Matrix' details all cross-service and internal service dependencies."
	_ = f.SetCellValue(sheet, "A7", instructions)
	_ = f.SetCellStyle(sheet, "A7", "H8", infoStyle)

	// Step Types Breakdown Table
	typeCounts := make(map[string]int)
	langCounts := make(map[string]int)
	for _, s := range pm.Steps {
		typeCounts[s.Type]++
		lang := s.Language
		if lang == "" {
			lang = "other"
		}
		langCounts[lang]++
	}

	subHeadStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#333f48"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// Table 1: Step Types
	_ = f.MergeCell(sheet, "A10", "D10")
	_ = f.SetCellValue(sheet, "A10", "Step Type Breakdown")
	_ = f.SetCellStyle(sheet, "A10", "D10", subHeadStyle)

	_ = f.SetCellValue(sheet, "A11", "Step Type")
	_ = f.SetCellValue(sheet, "D11", "Count")
	row := 12
	for tName, count := range typeCounts {
		_ = f.MergeCell(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row))
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), tName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), count)
		row++
	}

	// Table 2: Languages
	_ = f.MergeCell(sheet, "F10", "H10")
	_ = f.SetCellValue(sheet, "F10", "Source Language Breakdown")
	_ = f.SetCellStyle(sheet, "F10", "H10", subHeadStyle)

	_ = f.SetCellValue(sheet, "F11", "Language")
	_ = f.SetCellValue(sheet, "H11", "Count")
	lrow := 12
	for lName, count := range langCounts {
		_ = f.MergeCell(sheet, fmt.Sprintf("F%d", lrow), fmt.Sprintf("G%d", lrow))
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", lrow), strings.ToUpper(lName))
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", lrow), count)
		lrow++
	}

	_ = f.SetColWidth(sheet, "A", "H", 15)
	return nil
}

func (e *Exporter) buildVisualMapSheet(f *excelize.File, pm *model.ProcessModel) error {
	sheet := "Visual Process Map"
	_, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: "#FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#2F5597"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	_ = f.MergeCell(sheet, "A1", "F1")
	_ = f.SetCellValue(sheet, "A1", "Visual Process Map - Architecture Swimlanes")
	_ = f.SetCellStyle(sheet, "A1", "F1", titleStyle)

	stepsByLane := make(map[string][]model.Step)
	for _, s := range pm.Steps {
		stepsByLane[s.SwimlaneID] = append(stepsByLane[s.SwimlaneID], s)
	}

	outgoingMap := make(map[string][]string)
	for _, l := range pm.Links {
		desc := l.TargetStepID
		if l.Label != "" {
			desc = fmt.Sprintf("%s (%s)", l.TargetStepID, l.Label)
		}
		outgoingMap[l.SourceStepID] = append(outgoingMap[l.SourceStepID], desc)
	}

	currentRow := 3

	laneHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "#1F4E79"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "#2F5597", Style: 2},
			{Type: "top", Color: "#2F5597", Style: 1},
			{Type: "bottom", Color: "#2F5597", Style: 1},
			{Type: "right", Color: "#2F5597", Style: 1},
		},
		Alignment: &excelize.Alignment{Vertical: "center", Indent: 1},
	})

	cardHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 9, Color: "#595959"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#F2F2F2"}, Pattern: 1},
	})

	for _, lane := range pm.Swimlanes {
		steps := stepsByLane[lane.ID]
		if len(steps) == 0 {
			continue
		}

		_ = f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("F%d", currentRow))
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("SWIMLANE: %s (%d steps)", lane.Name, len(steps)))
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("F%d", currentRow), laneHeaderStyle)
		currentRow++

		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Step ID / Name")
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), "Type")
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", currentRow), "Description")
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", currentRow), "Source Code")
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", currentRow), "Next Connections")
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", currentRow), "Cross-Service")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("F%d", currentRow), cardHeaderStyle)
		currentRow++

		for _, step := range steps {
			bgColor := "#FFFFFF"
			switch step.Type {
			case "Endpoint":
				bgColor = "#EDF2F8"
			case "DatabaseQuery", "TableOperation":
				bgColor = "#EAF4EA"
			case "StoredProcedure", "View":
				bgColor = "#F4EEF8"
			case "Task", "WorkflowStep":
				bgColor = "#FEF8EC"
			}

			stepStyle, _ := f.NewStyle(&excelize.Style{
				Font:      &excelize.Font{Size: 9},
				Fill:      excelize.Fill{Type: "pattern", Color: []string{bgColor}, Pattern: 1},
				Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
				Border: []excelize.Border{
					{Type: "bottom", Color: "#E0E0E0", Style: 1},
				},
			})

			nextSteps := strings.Join(outgoingMap[step.ID], "; ")
			hasCross := "No"
			for _, l := range pm.Links {
				if l.SourceStepID == step.ID && l.IsCrossService {
					hasCross = "YES"
					break
				}
			}

			srcRef := fmt.Sprintf("%s:%d", filepath.Base(step.SourceFile), step.LineNumber)
			if step.SourceFile == "" {
				srcRef = "-"
			}

			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), step.Name)
			f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), step.Type)
			f.SetCellValue(sheet, fmt.Sprintf("C%d", currentRow), step.Description)
			f.SetCellValue(sheet, fmt.Sprintf("D%d", currentRow), srcRef)
			f.SetCellValue(sheet, fmt.Sprintf("E%d", currentRow), nextSteps)
			f.SetCellValue(sheet, fmt.Sprintf("F%d", currentRow), hasCross)

			for c := 1; c <= 6; c++ {
				cell, _ := excelize.CoordinatesToCellName(c, currentRow)
				_ = f.SetCellStyle(sheet, cell, cell, stepStyle)
			}
			currentRow++
		}
		currentRow++ // Space between lanes
	}

	_ = f.SetColWidth(sheet, "A", "A", 28)
	_ = f.SetColWidth(sheet, "B", "B", 18)
	_ = f.SetColWidth(sheet, "C", "C", 40)
	_ = f.SetColWidth(sheet, "D", "D", 20)
	_ = f.SetColWidth(sheet, "E", "E", 35)
	_ = f.SetColWidth(sheet, "F", "F", 15)

	return nil
}

func (e *Exporter) buildSchemaSheet(f *excelize.File, pm *model.ProcessModel) error {
	sheet := "Process Steps"
	_, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}

	headers := []string{
		"Process Step ID",
		"Step Description",
		"Next Step ID",
		"Connector Label",
		"Step Type",
		"Owner / Function",
		"Language",
		"Source File",
		"Line",
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2F5597"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	outgoingLinks := make(map[string][]model.Link)
	for _, link := range pm.Links {
		outgoingLinks[link.SourceStepID] = append(outgoingLinks[link.SourceStepID], link)
	}

	rowStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
	})

	for rowIdx, step := range pm.Steps {
		row := rowIdx + 2

		var nextIDs []string
		var labels []string
		for _, l := range outgoingLinks[step.ID] {
			nextIDs = append(nextIDs, l.TargetStepID)
			lbl := l.Label
			if l.Condition != "" {
				lbl = fmt.Sprintf("[%s] %s", l.Condition, lbl)
			}
			labels = append(labels, lbl)
		}

		owner := "General"
		if lane := pm.FindSwimlane(step.SwimlaneID); lane != nil {
			owner = lane.Name
		}

		desc := step.Description
		if desc == "" {
			desc = step.Name
		}

		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), step.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), desc)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), strings.Join(nextIDs, ", "))
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), strings.Join(labels, ", "))
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), step.Type)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), owner)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), strings.ToUpper(step.Language))
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), step.SourceFile)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), step.LineNumber)

		for c := 1; c <= len(headers); c++ {
			cell, _ := excelize.CoordinatesToCellName(c, row)
			_ = f.SetCellStyle(sheet, cell, cell, rowStyle)
		}
	}

	_ = f.SetColWidth(sheet, "A", "A", 28)
	_ = f.SetColWidth(sheet, "B", "B", 38)
	_ = f.SetColWidth(sheet, "C", "C", 25)
	_ = f.SetColWidth(sheet, "D", "D", 20)
	_ = f.SetColWidth(sheet, "E", "E", 18)
	_ = f.SetColWidth(sheet, "F", "F", 25)
	_ = f.SetColWidth(sheet, "G", "G", 12)
	_ = f.SetColWidth(sheet, "H", "H", 30)
	_ = f.SetColWidth(sheet, "I", "I", 10)

	return nil
}

func (e *Exporter) buildTraceabilitySheet(f *excelize.File, pm *model.ProcessModel) error {
	sheet := "Traceability Matrix"
	_, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}

	headers := []string{
		"Link ID",
		"Source Swimlane",
		"Source Step ID",
		"Source Step Name",
		"Target Swimlane",
		"Target Step ID",
		"Connector Label",
		"Condition",
		"Dependency Scope",
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	crossStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#C00000", Size: 9},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#FCE4D6"}, Pattern: 1},
	})
	internalStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#595959", Size: 9},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	cellStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})

	for idx, link := range pm.Links {
		row := idx + 2

		srcStep := pm.FindStep(link.SourceStepID)
		tgtStep := pm.FindStep(link.TargetStepID)

		srcLaneName := "-"
		srcStepName := "-"
		if srcStep != nil {
			srcStepName = srcStep.Name
			if l := pm.FindSwimlane(srcStep.SwimlaneID); l != nil {
				srcLaneName = l.Name
			}
		}

		tgtLaneName := "-"
		if tgtStep != nil {
			if l := pm.FindSwimlane(tgtStep.SwimlaneID); l != nil {
				tgtLaneName = l.Name
			}
		}

		scope := "INTERNAL"
		if link.IsCrossService {
			scope = "CROSS-SERVICE"
		}

		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), link.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), srcLaneName)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), link.SourceStepID)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), srcStepName)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), tgtLaneName)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), link.TargetStepID)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), link.Label)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), link.Condition)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), scope)

		for c := 1; c <= 8; c++ {
			cName, _ := excelize.CoordinatesToCellName(c, row)
			_ = f.SetCellStyle(sheet, cName, cName, cellStyle)
		}

		scopeCell := fmt.Sprintf("I%d", row)
		if link.IsCrossService {
			_ = f.SetCellStyle(sheet, scopeCell, scopeCell, crossStyle)
		} else {
			_ = f.SetCellStyle(sheet, scopeCell, scopeCell, internalStyle)
		}
	}

	_ = f.SetColWidth(sheet, "A", "A", 25)
	_ = f.SetColWidth(sheet, "B", "B", 25)
	_ = f.SetColWidth(sheet, "C", "C", 25)
	_ = f.SetColWidth(sheet, "D", "D", 22)
	_ = f.SetColWidth(sheet, "E", "E", 25)
	_ = f.SetColWidth(sheet, "F", "F", 25)
	_ = f.SetColWidth(sheet, "G", "G", 22)
	_ = f.SetColWidth(sheet, "H", "H", 18)
	_ = f.SetColWidth(sheet, "I", "I", 18)

	return nil
}

func (e *Exporter) buildServicesSheet(f *excelize.File, pm *model.ProcessModel) error {
	sheet := "Services & Swimlanes"
	_, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}

	headers := []string{
		"Swimlane ID",
		"Service / Component Name",
		"Description",
		"Total Steps",
		"Incoming Calls",
		"Outgoing Calls",
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#385723"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	stepCounts := make(map[string]int)
	incomingMap := make(map[string]int)
	outgoingMap := make(map[string]int)

	stepLaneMap := make(map[string]string)
	for _, s := range pm.Steps {
		stepCounts[s.SwimlaneID]++
		stepLaneMap[s.ID] = s.SwimlaneID
	}

	for _, l := range pm.Links {
		srcLane := stepLaneMap[l.SourceStepID]
		tgtLane := stepLaneMap[l.TargetStepID]
		if srcLane != "" {
			outgoingMap[srcLane]++
		}
		if tgtLane != "" {
			incomingMap[tgtLane]++
		}
	}

	rowStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
	})

	for idx, lane := range pm.Swimlanes {
		row := idx + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), lane.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), lane.Name)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), lane.Description)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), stepCounts[lane.ID])
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), incomingMap[lane.ID])
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), outgoingMap[lane.ID])

		for c := 1; c <= 6; c++ {
			cName, _ := excelize.CoordinatesToCellName(c, row)
			_ = f.SetCellStyle(sheet, cName, cName, rowStyle)
		}
	}

	_ = f.SetColWidth(sheet, "A", "A", 22)
	_ = f.SetColWidth(sheet, "B", "B", 30)
	_ = f.SetColWidth(sheet, "C", "C", 38)
	_ = f.SetColWidth(sheet, "D", "F", 16)

	return nil
}
