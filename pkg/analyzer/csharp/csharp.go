package csharp

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

// Analyzer implements static analysis for C# and Razor/Blazor source files using tree-sitter.
type Analyzer struct {
	parser *sitter.Parser
}

// New creates a new C# Analyzer.
func New() *Analyzer {
	p := sitter.NewParser()
	p.SetLanguage(csharp.GetLanguage())
	return &Analyzer{
		parser: p,
	}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "csharp"
}

// CanAnalyze returns true for .cs and .razor extensions.
func (a *Analyzer) CanAnalyze(ext string) bool {
	l := strings.ToLower(ext)
	return l == ".cs" || l == ".razor"
}

var (
	razorPageRegex   = regexp.MustCompile(`@page\s+"([^"]+)"`)
	razorInjectRegex = regexp.MustCompile(`@inject\s+(\S+)\s+(\S+)`)
	razorCodeRegex   = regexp.MustCompile(`(?s)@code\s*\{(.*)\}`)
)

// AnalyzeFile parses C# or Razor/Blazor files.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	ext := strings.ToLower(filepath.Ext(path))

	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	if ext == ".razor" {
		a.analyzeRazor(path, content, result)
	} else {
		if err := a.analyzeCSharp(path, content, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (a *Analyzer) analyzeRazor(path string, content []byte, result *analyzer.FileAnalysisResult) {
	src := string(content)
	fileName := filepath.Base(path)
	componentName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	swimlaneID := fmt.Sprintf("razor:%s", componentName)
	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          swimlaneID,
		Name:        fmt.Sprintf("Blazor Component (%s)", componentName),
		Description: fmt.Sprintf("Razor/Blazor page in %s", fileName),
	})

	// Check @page route
	pageMatches := razorPageRegex.FindStringSubmatch(src)
	route := ""
	if len(pageMatches) > 1 {
		route = pageMatches[1]
	}

	stepID := fmt.Sprintf("%s:%s", path, componentName)
	componentStep := model.Step{
		ID:          stepID,
		SwimlaneID:  swimlaneID,
		Name:        componentName,
		Description: fmt.Sprintf("Razor Component %s", componentName),
		Type:        "UIComponent",
		Language:    "csharp",
		SourceFile:  path,
		LineNumber:  1,
		Metadata:    make(map[string]any),
	}

	if route != "" {
		componentStep.Type = "Endpoint"
		componentStep.Description = fmt.Sprintf("Blazor Route %s (%s)", route, componentName)
		componentStep.Metadata["route"] = route
	}
	result.Steps = append(result.Steps, componentStep)

	// Check @code block for C# methods
	codeMatches := razorCodeRegex.FindStringSubmatch(src)
	if len(codeMatches) > 1 {
		csharpCode := []byte(codeMatches[1])
		tree, err := a.parser.ParseCtx(context.Background(), nil, csharpCode)
		if err == nil && tree != nil {
			a.extractFromNode(path, csharpCode, tree.RootNode(), swimlaneID, stepID, result)
		}
	}
}

func (a *Analyzer) analyzeCSharp(path string, content []byte, result *analyzer.FileAnalysisResult) error {
	tree, err := a.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return fmt.Errorf("csharp tree-sitter parse error in %s: %w", path, err)
	}

	fileName := filepath.Base(path)
	defaultSwimlaneID := fmt.Sprintf("cs:%s", fileName)
	swimlane := model.Swimlane{
		ID:          defaultSwimlaneID,
		Name:        fmt.Sprintf("C# Class (%s)", strings.TrimSuffix(fileName, filepath.Ext(fileName))),
		Description: fmt.Sprintf("C# file %s", fileName),
	}
	result.Swimlanes = append(result.Swimlanes, swimlane)

	a.extractFromNode(path, content, tree.RootNode(), defaultSwimlaneID, "", result)
	return nil
}

func (a *Analyzer) extractFromNode(path string, content []byte, root *sitter.Node, currentSwimlaneID string, parentStepID string, result *analyzer.FileAnalysisResult) {
	if root == nil {
		return
	}

	// Traverse tree to find classes and methods
	var walk func(n *sitter.Node, currentClass string)
	walk = func(n *sitter.Node, currentClass string) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Class declaration
		if nodeType == "class_declaration" || nodeType == "record_declaration" {
			nameNode := n.ChildByFieldName("name")
			className := currentClass
			if nameNode != nil {
				className = nameNode.Content(content)
				swimlaneID := fmt.Sprintf("cs:%s", className)
				result.Swimlanes = append(result.Swimlanes, model.Swimlane{
					ID:          swimlaneID,
					Name:        fmt.Sprintf("C# Class (%s)", className),
					Description: fmt.Sprintf("Class %s in %s", className, filepath.Base(path)),
				})
				currentSwimlaneID = swimlaneID
			}

			for i := 0; i < int(n.NamedChildCount()); i++ {
				walk(n.NamedChild(i), className)
			}
			return
		}

		// Method declaration
		if nodeType == "method_declaration" {
			nameNode := n.ChildByFieldName("name")
			methodName := "unknown"
			if nameNode != nil {
				methodName = nameNode.Content(content)
			}

			startPoint := n.StartPoint()
			line := int(startPoint.Row) + 1
			methodStepID := fmt.Sprintf("%s:%s:%d", path, methodName, line)

			// Analyze attributes (e.g., [HttpGet], [HttpPost], [Route])
			httpRoute := ""
			httpMethod := ""
			isAsync := false

			// Check modifiers (async)
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "modifier" && child.Content(content) == "async" {
					isAsync = true
				}
				if child.Type() == "attribute_list" {
					route, verb := extractCSharpRouteAttribute(child, content)
					if verb != "" {
						httpMethod = verb
					}
					if route != "" {
						httpRoute = route
					}
				}
			}

			stepType := "Function"
			desc := fmt.Sprintf("Method %s", methodName)
			if httpMethod != "" || httpRoute != "" {
				stepType = "Endpoint"
				desc = fmt.Sprintf("Controller Endpoint [%s] %s -> %s", httpMethod, httpRoute, methodName)
			} else if isAsync {
				stepType = "Task"
				desc = fmt.Sprintf("Async Task %s", methodName)
			}

			step := model.Step{
				ID:          methodStepID,
				SwimlaneID:  currentSwimlaneID,
				Name:        methodName,
				Description: desc,
				Type:        stepType,
				Language:    "csharp",
				SourceFile:  path,
				LineNumber:  line,
				Metadata: map[string]any{
					"class": currentClass,
				},
			}
			if httpRoute != "" {
				step.Metadata["route"] = httpRoute
			}
			if httpMethod != "" {
				step.Metadata["method"] = httpMethod
			}
			result.Steps = append(result.Steps, step)

			if parentStepID != "" {
				result.Links = append(result.Links, model.Link{
					ID:           fmt.Sprintf("%s->%s", parentStepID, methodStepID),
					SourceStepID: parentStepID,
					TargetStepID: methodStepID,
					Label:        "contains",
				})
			}

			// Traverse method body for EF operations and invocations
			body := n.ChildByFieldName("body")
			if body == nil {
				// Expression-bodied method?
				body = n.ChildByFieldName("expression_body")
			}
			if body != nil {
				a.extractMethodInvocations(path, content, body, currentSwimlaneID, methodStepID, result)
			}
			return
		}

		for i := 0; i < int(n.NamedChildCount()); i++ {
			walk(n.NamedChild(i), currentClass)
		}
	}

	walk(root, "")
}

func (a *Analyzer) extractMethodInvocations(path string, content []byte, body *sitter.Node, swimlaneID string, methodStepID string, result *analyzer.FileAnalysisResult) {
	var walkBody func(n *sitter.Node)
	walkBody = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "invocation_expression" {
			callText := n.Content(content)
			line := int(n.StartPoint().Row) + 1

			// Check for Entity Framework operations
			if isEFOperation(callText) {
				efStepID := fmt.Sprintf("%s:ef:%d", path, line)
				efOp := cleanEFName(callText)
				efStep := model.Step{
					ID:          efStepID,
					SwimlaneID:  swimlaneID,
					Name:        fmt.Sprintf("EF: %s", efOp),
					Description: fmt.Sprintf("Entity Framework Operation %s", callText),
					Type:        "DatabaseQuery",
					Language:    "csharp",
					SourceFile:  path,
					LineNumber:  line,
					Metadata: map[string]any{
						"operation": efOp,
						"raw":       callText,
					},
				}
				result.Steps = append(result.Steps, efStep)
				result.Links = append(result.Links, model.Link{
					ID:           fmt.Sprintf("%s->%s", methodStepID, efStepID),
					SourceStepID: methodStepID,
					TargetStepID: efStepID,
					Label:        "executes EF query",
				})
			}
		}

		for i := 0; i < int(n.NamedChildCount()); i++ {
			walkBody(n.NamedChild(i))
		}
	}
	walkBody(body)
}

func extractCSharpRouteAttribute(attrList *sitter.Node, content []byte) (route string, httpMethod string) {
	text := attrList.Content(content)
	verbs := []string{"HttpGet", "HttpPost", "HttpPut", "HttpDelete", "HttpPatch", "Route"}
	for _, verb := range verbs {
		if strings.Contains(text, verb) {
			if verb != "Route" {
				httpMethod = strings.ToUpper(strings.TrimPrefix(verb, "Http"))
			}
			// Extract route inside ("...")
			re := regexp.MustCompile(`\(\s*"([^"]*)"\s*\)`)
			matches := re.FindStringSubmatch(text)
			if len(matches) > 1 {
				route = matches[1]
			}
			return route, httpMethod
		}
	}
	return "", ""
}

func isEFOperation(callText string) bool {
	efKeywords := []string{
		"SaveChangesAsync", "SaveChanges", ".AddAsync(", ".Add(",
		".FirstOrDefaultAsync(", ".FirstOrDefault(", ".ToListAsync(",
		".ToList(", ".Where(", ".FromSqlRaw(", ".ExecuteSqlRaw(",
	}
	for _, kw := range efKeywords {
		if strings.Contains(callText, kw) {
			return true
		}
	}
	return false
}

func cleanEFName(callText string) string {
	parts := strings.Split(callText, "(")
	if len(parts) > 0 {
		subParts := strings.Split(parts[0], ".")
		if len(subParts) >= 2 {
			return subParts[len(subParts)-2] + "." + subParts[len(subParts)-1]
		}
		return subParts[len(subParts)-1]
	}
	return callText
}
