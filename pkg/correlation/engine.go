package correlation

import (
	"fmt"
	"regexp"
	"strings"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

// Engine correlates steps across multi-language components and unifies them into a canonical ProcessModel.
type Engine struct{}

// New creates a new Correlation Engine.
func New() *Engine {
	return &Engine{}
}

// Correlate takes file analysis results and produces a correlated ProcessModel.
func (e *Engine) Correlate(modelID, modelName string, results []*analyzer.FileAnalysisResult) *model.ProcessModel {
	pm := model.NewProcessModel(modelID, modelName)

	// 1. Merge swimlanes, steps, and links
	for _, res := range results {
		for _, sl := range res.Swimlanes {
			pm.AddSwimlane(sl)
		}
		for _, step := range res.Steps {
			pm.AddStep(step)
		}
		for _, link := range res.Links {
			pm.AddLink(link)
		}
	}

	// 2. Perform cross-service correlation:
	// a) Route correlation (HTTP endpoints matching calls or client routes)
	e.correlateRoutes(pm)

	// b) Database correlation (Code DB queries matching SQL table operations or stored procedures)
	e.correlateDatabases(pm)

	// c) Cross-package and cross-service call correlation
	e.correlateCalls(pm)

	return pm
}

func (e *Engine) correlateRoutes(pm *model.ProcessModel) {
	// Index endpoints by route and method
	type endpointInfo struct {
		Step   *model.Step
		Route  string
		Method string
	}
	var endpoints []endpointInfo

	for i := range pm.Steps {
		s := &pm.Steps[i]
		if s.Type == "Endpoint" {
			route := ""
			method := ""
			if r, ok := s.Metadata["route"].(string); ok {
				route = normalizeRoute(r)
			}
			if m, ok := s.Metadata["method"].(string); ok {
				method = strings.ToUpper(m)
			}
			if route != "" {
				endpoints = append(endpoints, endpointInfo{
					Step:   s,
					Route:  route,
					Method: method,
				})
			}
		}
	}

	// Look for caller steps (e.g., UI components or functions that reference the route)
	for i := range pm.Steps {
		caller := &pm.Steps[i]
		callerText := strings.ToLower(caller.Name + " " + caller.Description)

		for _, ep := range endpoints {
			if ep.Step.ID == caller.ID || ep.Step.SwimlaneID == caller.SwimlaneID {
				continue
			}

			// If caller text contains the route
			if strings.Contains(callerText, strings.ToLower(ep.Route)) && ep.Route != "/" {
				linkID := fmt.Sprintf("%s->%s:cross_route", caller.ID, ep.Step.ID)
				pm.AddLink(model.Link{
					ID:             linkID,
					SourceStepID:   caller.ID,
					TargetStepID:   ep.Step.ID,
					Label:          fmt.Sprintf("HTTP %s %s", ep.Method, ep.Route),
					IsCrossService: true,
				})
			}
		}
	}
}

func (e *Engine) correlateDatabases(pm *model.ProcessModel) {
	// Index SQL & Liquibase tables and stored procedures
	sqlTables := make(map[string]*model.Step)
	sqlProcs := make(map[string]*model.Step)

	for i := range pm.Steps {
		s := &pm.Steps[i]
		if s.Language == "sql" || s.Language == "liquibase" || s.Type == "DatabaseTable" {
			if tbl, ok := s.Metadata["table"].(string); ok && tbl != "" {
				sqlTables[strings.ToLower(tbl)] = s
			} else if s.Type == "DatabaseTable" && s.Name != "" {
				sqlTables[strings.ToLower(s.Name)] = s
			}
			if proc, ok := s.Metadata["procedure"].(string); ok && proc != "" {
				sqlProcs[strings.ToLower(proc)] = s
			}
		}
	}

	// Match cross-file foreign keys between tables
	for i := range pm.Steps {
		s := &pm.Steps[i]
		if s.Type != "DatabaseTable" {
			continue
		}
		fksRaw, ok := s.Metadata["foreign_keys"].([]map[string]string)
		if !ok || len(fksRaw) == 0 {
			continue
		}
		for _, fk := range fksRaw {
			refTblName := strings.ToLower(fk["referenced_table"])
			if targetStep, exists := sqlTables[refTblName]; exists && targetStep.ID != s.ID {
				label := "references"
				if cName := fk["constraint_name"]; cName != "" {
					label = fmt.Sprintf("references (%s)", cName)
				}
				linkID := fmt.Sprintf("%s->%s:fk", s.ID, targetStep.ID)
				pm.AddLink(model.Link{
					ID:             linkID,
					SourceStepID:   s.ID,
					TargetStepID:   targetStep.ID,
					Label:          label,
					IsCrossService: targetStep.SwimlaneID != s.SwimlaneID,
				})
			}
		}
	}

	// Match code steps that query databases
	for i := range pm.Steps {
		s := &pm.Steps[i]
		if s.Language != "sql" && s.Language != "liquibase" && (s.Type == "DatabaseQuery" || strings.Contains(s.Description, "Database") || strings.Contains(s.Description, "EF")) {
			rawQuery := ""
			if q, ok := s.Metadata["query"].(string); ok {
				rawQuery = q
			}
			if rawQuery == "" {
				if r, ok := s.Metadata["raw"].(string); ok {
					rawQuery = r
				}
			}
			if rawQuery == "" {
				rawQuery = s.Description + " " + s.Name
			}

			lowerQuery := strings.ToLower(rawQuery)

			// Match tables
			for tblName, targetStep := range sqlTables {
				if matchWord(lowerQuery, tblName) {
					linkID := fmt.Sprintf("%s->%s:db_table", s.ID, targetStep.ID)
					pm.AddLink(model.Link{
						ID:             linkID,
						SourceStepID:   s.ID,
						TargetStepID:   targetStep.ID,
						Label:          fmt.Sprintf("accesses table %s", tblName),
						IsCrossService: true,
					})
				}
			}

			// Match stored procedures
			for procName, targetStep := range sqlProcs {
				if matchWord(lowerQuery, procName) {
					linkID := fmt.Sprintf("%s->%s:db_proc", s.ID, targetStep.ID)
					pm.AddLink(model.Link{
						ID:             linkID,
						SourceStepID:   s.ID,
						TargetStepID:   targetStep.ID,
						Label:          fmt.Sprintf("executes proc %s", procName),
						IsCrossService: true,
					})
				}
			}
		}
	}
}

func (e *Engine) correlateCalls(pm *model.ProcessModel) {
	// Index steps by lowercase lookup keys
	stepByExact := make(map[string]*model.Step)
	stepByPkg := make(map[string]*model.Step)
	stepByShort := make(map[string]*model.Step)

	for i := range pm.Steps {
		s := &pm.Steps[i]
		lowerName := strings.ToLower(s.Name)
		stepByExact[lowerName] = s

		if pkg, ok := s.Metadata["package"].(string); ok && pkg != "" {
			stepByPkg[strings.ToLower(fmt.Sprintf("%s.%s", pkg, s.Name))] = s
			if fn, ok := s.Metadata["func"].(string); ok && fn != "" {
				stepByPkg[strings.ToLower(fmt.Sprintf("%s.%s", pkg, fn))] = s
			}
		}
		if fn, ok := s.Metadata["func"].(string); ok && fn != "" {
			stepByShort[strings.ToLower(fn)] = s
		}
	}

	existingLinks := make(map[string]bool)
	for _, l := range pm.Links {
		existingLinks[fmt.Sprintf("%s->%s", l.SourceStepID, l.TargetStepID)] = true
	}

	for i := range pm.Steps {
		caller := &pm.Steps[i]
		callsRaw, ok := caller.Metadata["calls"]
		if !ok {
			continue
		}

		var calls []string
		switch c := callsRaw.(type) {
		case []string:
			calls = c
		case []any:
			for _, item := range c {
				if str, ok := item.(string); ok {
					calls = append(calls, str)
				}
			}
		}

		for _, call := range calls {
			lowerCall := strings.ToLower(call)
			var target *model.Step

			if s, ok := stepByPkg[lowerCall]; ok {
				target = s
			} else if s, ok := stepByExact[lowerCall]; ok {
				target = s
			} else if s, ok := stepByShort[lowerCall]; ok {
				target = s
			}

			if target != nil && target.ID != caller.ID {
				key := fmt.Sprintf("%s->%s", caller.ID, target.ID)
				if !existingLinks[key] {
					existingLinks[key] = true
					pm.AddLink(model.Link{
						ID:             fmt.Sprintf("%s->%s:call", caller.ID, target.ID),
						SourceStepID:   caller.ID,
						TargetStepID:   target.ID,
						Label:          "calls",
						IsCrossService: caller.SwimlaneID != target.SwimlaneID,
					})
				}
			}
		}
	}
}

func normalizeRoute(r string) string {
	r = strings.TrimSpace(r)
	r = strings.Trim(r, "\"'")
	// Normalize path parameters like {id} or :id or [controller]
	re := regexp.MustCompile(`\{[^}]+\}|:[a-zA-Z0-9_]+`)
	return re.ReplaceAllString(r, "*")
}

func matchWord(text, word string) bool {
	if word == "" {
		return false
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	return re.MatchString(text)
}
