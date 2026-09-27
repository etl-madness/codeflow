package python

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

// Analyzer implements static analysis for Python files using tree-sitter.
type Analyzer struct {
	parser *sitter.Parser
}

// New creates a new Python Analyzer.
func New() *Analyzer {
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())
	return &Analyzer{
		parser: p,
	}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "python"
}

// CanAnalyze returns true for .py extension.
func (a *Analyzer) CanAnalyze(ext string) bool {
	return strings.ToLower(ext) == ".py"
}

var (
	routePattern  = regexp.MustCompile(`\(['"]([^'"]+)['"]`)
	methodPattern = regexp.MustCompile(`(?i)\.(get|post|put|delete|patch|options|head|route)\b`)
)

// AnalyzeFile parses Python files and extracts endpoints, functions, and call chains.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	tree, err := a.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("python tree-sitter parse error in %s: %w", path, err)
	}

	fileName := filepath.Base(path)
	moduleName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	swimlaneID := fmt.Sprintf("py:%s", moduleName)

	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          swimlaneID,
		Name:        fmt.Sprintf("Python Module (%s)", moduleName),
		Description: fmt.Sprintf("Python file %s", fileName),
	})

	funcMap := make(map[string]string) // funcName -> stepID

	// First pass: extract functions and endpoints
	var findFunctions func(n *sitter.Node)
	findFunctions = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		var fnDef *sitter.Node
		var decorators []string

		if nodeType == "decorated_definition" {
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child.Type() == "decorator" {
					decorators = append(decorators, child.Content(content))
				} else if child.Type() == "function_definition" {
					fnDef = child
				}
			}
		} else if nodeType == "function_definition" {
			fnDef = n
		}

		if fnDef != nil {
			nameNode := fnDef.ChildByFieldName("name")
			fnName := "unknown"
			if nameNode != nil {
				fnName = nameNode.Content(content)
			}

			startPoint := fnDef.StartPoint()
			line := int(startPoint.Row) + 1
			stepID := fmt.Sprintf("%s:%s:%d", path, fnName, line)

			stepType := "Function"
			stepDesc := fmt.Sprintf("Python function %s", fnName)

			var route, method string
			for _, dec := range decorators {
				r, m := extractPythonRoute(dec)
				if r != "" || m != "" {
					route = r
					method = m
					stepType = "Endpoint"
					stepDesc = fmt.Sprintf("API Endpoint [%s] %s -> %s", method, route, fnName)
					break
				}
			}

			step := model.Step{
				ID:          stepID,
				SwimlaneID:  swimlaneID,
				Name:        fnName,
				Description: stepDesc,
				Type:        stepType,
				Language:    "python",
				SourceFile:  path,
				LineNumber:  line,
				Metadata: map[string]any{
					"module": moduleName,
				},
			}
			if route != "" {
				step.Metadata["route"] = route
			}
			if method != "" {
				step.Metadata["method"] = method
			}

			result.Steps = append(result.Steps, step)
			funcMap[fnName] = stepID

			// Search inside function body for calls
			bodyNode := fnDef.ChildByFieldName("body")
			if bodyNode != nil {
				a.extractCalls(path, content, bodyNode, swimlaneID, stepID, funcMap, result)
			}
			return
		}

		for i := 0; i < int(n.NamedChildCount()); i++ {
			findFunctions(n.NamedChild(i))
		}
	}

	findFunctions(tree.RootNode())

	return result, nil
}

func (a *Analyzer) extractCalls(path string, content []byte, body *sitter.Node, swimlaneID string, callerStepID string, funcMap map[string]string, result *analyzer.FileAnalysisResult) {
	var walkBody func(n *sitter.Node)
	walkBody = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "call" {
			fnNode := n.ChildByFieldName("function")
			if fnNode != nil {
				callText := fnNode.Content(content)
				line := int(n.StartPoint().Row) + 1

				// Check for DB calls (e.g. session.query, db.execute, cursor.execute)
				if isPythonDBCall(callText) {
					dbStepID := fmt.Sprintf("%s:db:%d", path, line)
					dbStep := model.Step{
						ID:          dbStepID,
						SwimlaneID:  swimlaneID,
						Name:        fmt.Sprintf("DB %s", callText),
						Description: fmt.Sprintf("Database operation %s", callText),
						Type:        "DatabaseQuery",
						Language:    "python",
						SourceFile:  path,
						LineNumber:  line,
						Metadata: map[string]any{
							"call": callText,
						},
					}
					result.Steps = append(result.Steps, dbStep)
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", callerStepID, dbStepID),
						SourceStepID: callerStepID,
						TargetStepID: dbStepID,
						Label:        "executes db query",
					})
				} else if targetID, ok := funcMap[callText]; ok && targetID != callerStepID {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s:%d", callerStepID, targetID, line),
						SourceStepID: callerStepID,
						TargetStepID: targetID,
						Label:        "calls",
					})
				}
			}
		}

		for i := 0; i < int(n.NamedChildCount()); i++ {
			walkBody(n.NamedChild(i))
		}
	}

	walkBody(body)
}

func extractPythonRoute(decorator string) (route string, method string) {
	// Look for @app.get(...), @router.post(...), @app.route(...)
	matches := methodPattern.FindStringSubmatch(decorator)
	if len(matches) > 1 {
		rawVerb := strings.ToUpper(matches[1])
		if rawVerb == "ROUTE" {
			method = "ANY"
		} else {
			method = rawVerb
		}
	}

	routeMatches := routePattern.FindStringSubmatch(decorator)
	if len(routeMatches) > 1 {
		route = routeMatches[1]
	}

	return route, method
}

func isPythonDBCall(callText string) bool {
	dbKeywords := []string{
		".execute", ".query", ".filter", ".add", ".commit", "session.query",
		"cursor.execute", "db.session", "connection.execute",
	}
	for _, kw := range dbKeywords {
		if strings.Contains(callText, kw) {
			return true
		}
	}
	return false
}
