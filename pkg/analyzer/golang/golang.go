package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"codeflow/pkg/analyzer"
	"codeflow/pkg/model"
)

// Analyzer implements static analysis for Go source files.
type Analyzer struct{}

// New creates a new Go Analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Language returns the language identifier.
func (a *Analyzer) Language() string {
	return "go"
}

// CanAnalyze returns true if the extension is .go.
func (a *Analyzer) CanAnalyze(ext string) bool {
	return strings.ToLower(ext) == ".go"
}

// AnalyzeFile parses Go source code and extracts endpoints, DB queries, gRPC registrations, and function calls.
func (a *Analyzer) AnalyzeFile(path string, content []byte) (*analyzer.FileAnalysisResult, error) {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("go parser error in %s: %w", path, err)
	}

	result := &analyzer.FileAnalysisResult{
		Steps:     make([]model.Step, 0),
		Links:     make([]model.Link, 0),
		Swimlanes: make([]model.Swimlane, 0),
		ASTNodes:  make([]*model.ASTNode, 0),
	}

	pkgName := fileNode.Name.Name
	swimlaneID := fmt.Sprintf("go:%s", pkgName)
	result.Swimlanes = append(result.Swimlanes, model.Swimlane{
		ID:          swimlaneID,
		Name:        fmt.Sprintf("Go Service (%s)", pkgName),
		Description: fmt.Sprintf("Go package %s in %s", pkgName, filepath.Base(path)),
	})

	// Track functions declared in this file
	funcSteps := make(map[string]string) // funcName or Recv.funcName -> stepID
	stepCalls := make(map[string][]string) // stepID -> list of callee names

	// Zeroeth pass: extract struct declarations for data modeling and ER diagrams
	for _, decl := range fileNode.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, isStruct := typeSpec.Type.(*ast.StructType)
			if !isStruct {
				continue
			}

			structName := typeSpec.Name.Name
			line := fset.Position(typeSpec.Pos()).Line
			stepID := fmt.Sprintf("%s:struct:%s:%d", path, structName, line)

			desc := fmt.Sprintf("Go Struct %s in %s", structName, pkgName)
			if typeSpec.Doc != nil {
				if d := strings.TrimSpace(typeSpec.Doc.Text()); d != "" {
					desc = d
				}
			} else if genDecl.Doc != nil {
				if d := strings.TrimSpace(genDecl.Doc.Text()); d != "" {
					desc = d
				}
			}

			var cols []map[string]string
			if structType.Fields != nil {
				for _, field := range structType.Fields.List {
					fieldType := exprToString(field.Type)
					for _, name := range field.Names {
						key := ""
						if strings.EqualFold(name.Name, "id") || strings.EqualFold(name.Name, structName+"id") {
							key = "PK"
						}
						cols = append(cols, map[string]string{
							"name": name.Name,
							"type": fieldType,
							"key":  key,
						})
					}
					if len(field.Names) == 0 {
						cols = append(cols, map[string]string{
							"name": fieldType,
							"type": "embedded",
						})
					}
				}
			}

			result.Steps = append(result.Steps, model.Step{
				ID:          stepID,
				SwimlaneID:  swimlaneID,
				Name:        structName,
				Description: desc,
				Type:        "DatabaseTable", // Enables ER diagram representation andVisio table mapping
				Language:    "go",
				SourceFile:  path,
				LineNumber:  line,
				Metadata: map[string]any{
					"package":   pkgName,
					"table":     structName,
					"columns":   cols,
					"is_struct": true,
				},
			})
		}
	}

	// First pass: extract function and method declarations
	for _, decl := range fileNode.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcName := funcDecl.Name.Name
		line := fset.Position(funcDecl.Pos()).Line

		recvType := ""
		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			recvType = extractReceiverType(funcDecl.Recv.List[0].Type)
		}

		displayName := funcName
		if recvType != "" {
			displayName = fmt.Sprintf("%s.%s", recvType, funcName)
		}

		stepID := fmt.Sprintf("%s:%s:%d", path, displayName, line)

		stepType := "Function"
		stepDesc := fmt.Sprintf("Function %s", displayName)
		if funcDecl.Doc != nil {
			if d := strings.TrimSpace(funcDecl.Doc.Text()); d != "" {
				stepDesc = d
			}
		}

		// Check if it's a gRPC handler or HTTP handler
		if isGRPCHandler(funcDecl) {
			stepType = "Endpoint"
			if funcDecl.Doc == nil {
				stepDesc = fmt.Sprintf("gRPC Service Handler %s", displayName)
			}
		} else if isHTTPHandlerFunc(funcDecl) {
			stepType = "Endpoint"
			if funcDecl.Doc == nil {
				stepDesc = fmt.Sprintf("HTTP Handler %s", displayName)
			}
		}

		funcStep := model.Step{
			ID:          stepID,
			SwimlaneID:  swimlaneID,
			Name:        displayName,
			Description: stepDesc,
			Type:        stepType,
			Language:    "go",
			SourceFile:  path,
			LineNumber:  line,
			Metadata: map[string]any{
				"package":  pkgName,
				"func":     funcName,
				"receiver": recvType,
			},
		}

		result.Steps = append(result.Steps, funcStep)
		funcSteps[funcName] = stepID
		funcSteps[displayName] = stepID

		result.ASTNodes = append(result.ASTNodes, &model.ASTNode{
			ID:   stepID,
			Type: "FuncDecl",
			Name: displayName,
		})
	}

	// Second pass: inspect AST calls inside functions (HTTP routes, DB queries, gRPC registrations, function calls)
	for _, decl := range fileNode.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcName := funcDecl.Name.Name
		recvType := ""
		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			recvType = extractReceiverType(funcDecl.Recv.List[0].Type)
		}
		displayName := funcName
		if recvType != "" {
			displayName = fmt.Sprintf("%s.%s", recvType, funcName)
		}
		parentStepID := funcSteps[displayName]
		if parentStepID == "" {
			parentStepID = funcSteps[funcName]
		}

		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			callLine := fset.Position(callExpr.Pos()).Line

			// 1. Check for HTTP route definitions (e.g., r.GET("/path", handler), http.HandleFunc("/path", handler))
			if route, method, handlerName, isRoute := extractHTTPRoute(callExpr); isRoute {
				routeStepID := fmt.Sprintf("%s:route:%s:%d", path, strings.Trim(route, "\""), callLine)
				routeStep := model.Step{
					ID:          routeStepID,
					SwimlaneID:  swimlaneID,
					Name:        fmt.Sprintf("%s %s", method, strings.Trim(route, "\"")),
					Description: fmt.Sprintf("HTTP Endpoint %s %s -> %s", method, strings.Trim(route, "\""), handlerName),
					Type:        "Endpoint",
					Language:    "go",
					SourceFile:  path,
					LineNumber:  callLine,
					Metadata: map[string]any{
						"route":   strings.Trim(route, "\""),
						"method":  method,
						"handler": handlerName,
					},
				}
				result.Steps = append(result.Steps, routeStep)

				// Link route to parent setup step or to handler step if handler step exists
				if parentStepID != "" {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", parentStepID, routeStepID),
						SourceStepID: parentStepID,
						TargetStepID: routeStepID,
						Label:        "registers route",
					})
				}
				if handlerName != "" {
					if targetID, ok := funcSteps[handlerName]; ok {
						result.Links = append(result.Links, model.Link{
							ID:           fmt.Sprintf("%s->%s", routeStepID, targetID),
							SourceStepID: routeStepID,
							TargetStepID: targetID,
							Label:        "dispatches to",
						})
					}
				}
				return true
			}

			// 2. Check for gRPC server registration (e.g., pb.RegisterXServer(s, srv))
			if grpcService, isGRPC := extractGRPCRegistration(callExpr); isGRPC {
				grpcStepID := fmt.Sprintf("%s:grpc:%s:%d", path, grpcService, callLine)
				grpcStep := model.Step{
					ID:          grpcStepID,
					SwimlaneID:  swimlaneID,
					Name:        fmt.Sprintf("Register gRPC %s", grpcService),
					Description: fmt.Sprintf("gRPC Server Registration for %s", grpcService),
					Type:        "Endpoint",
					Language:    "go",
					SourceFile:  path,
					LineNumber:  callLine,
					Metadata: map[string]any{
						"grpc_service": grpcService,
					},
				}
				result.Steps = append(result.Steps, grpcStep)
				if parentStepID != "" {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", parentStepID, grpcStepID),
						SourceStepID: parentStepID,
						TargetStepID: grpcStepID,
						Label:        "registers gRPC",
					})
				}
				return true
			}

			// 3. Check for DB queries (e.g. db.Query, db.Exec, gorm.Find, gorm.Create)
			if query, op, isDB := extractDBQuery(callExpr); isDB {
				dbStepID := fmt.Sprintf("%s:db:%d", path, callLine)
				dbStep := model.Step{
					ID:          dbStepID,
					SwimlaneID:  swimlaneID,
					Name:        fmt.Sprintf("DB %s", op),
					Description: fmt.Sprintf("Database operation: %s (%s)", op, query),
					Type:        "DatabaseQuery",
					Language:    "go",
					SourceFile:  path,
					LineNumber:  callLine,
					Metadata: map[string]any{
						"operation": op,
						"query":     query,
					},
				}
				result.Steps = append(result.Steps, dbStep)
				if parentStepID != "" {
					result.Links = append(result.Links, model.Link{
						ID:           fmt.Sprintf("%s->%s", parentStepID, dbStepID),
						SourceStepID: parentStepID,
						TargetStepID: dbStepID,
						Label:        "executes query",
					})
				}
				return true
			}

			// 4. Function / method call
			if pkg, fn, ok := extractQualifiedCall(callExpr); ok {
				if parentStepID != "" {
					stepCalls[parentStepID] = append(stepCalls[parentStepID], fmt.Sprintf("%s.%s", pkg, fn))
					if targetID, exists := funcSteps[fn]; exists && targetID != parentStepID {
						result.Links = append(result.Links, model.Link{
							ID:           fmt.Sprintf("%s->%s:%d", parentStepID, targetID, callLine),
							SourceStepID: parentStepID,
							TargetStepID: targetID,
							Label:        "calls",
						})
					}
				}
			} else if calleeName, ok := extractSimpleCallName(callExpr); ok {
				if parentStepID != "" {
					stepCalls[parentStepID] = append(stepCalls[parentStepID], calleeName)
					if targetID, exists := funcSteps[calleeName]; exists && targetID != parentStepID {
						result.Links = append(result.Links, model.Link{
							ID:           fmt.Sprintf("%s->%s:%d", parentStepID, targetID, callLine),
							SourceStepID: parentStepID,
							TargetStepID: targetID,
							Label:        "calls",
						})
					}
				}
			}

			return true
		})
	}

	// Attach outgoing calls to step metadata for cross-package correlation
	for i := range result.Steps {
		s := &result.Steps[i]
		if calls, ok := stepCalls[s.ID]; ok && len(calls) > 0 {
			s.Metadata["calls"] = calls
		}
	}

	return result, nil
}

// isGRPCHandler checks if a function signature matches gRPC handler pattern.
func isGRPCHandler(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 2 {
		return false
	}
	// Typically (ctx context.Context, req *pb.Request) (*pb.Response, error)
	p1 := fmt.Sprintf("%v", fn.Type.Params.List[0].Type)
	return strings.Contains(p1, "context.Context") || strings.Contains(p1, "Context")
}

// isHTTPHandlerFunc checks if a function signature matches http.HandlerFunc.
func isHTTPHandlerFunc(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	params := fn.Type.Params.List
	if len(params) == 2 {
		p0 := fmt.Sprintf("%v", params[0].Type)
		p1 := fmt.Sprintf("%v", params[1].Type)
		if strings.Contains(p0, "ResponseWriter") && strings.Contains(p1, "Request") {
			return true
		}
	} else if len(params) == 1 {
		p0 := fmt.Sprintf("%v", params[0].Type)
		if strings.Contains(p0, "Context") || strings.Contains(p0, "gin") || strings.Contains(p0, "echo") {
			return true
		}
	}
	return false
}

// extractHTTPRoute detects HTTP route registration calls.
func extractHTTPRoute(call *ast.CallExpr) (route string, method string, handler string, ok bool) {
	sel, okSel := call.Fun.(*ast.SelectorExpr)
	if !okSel {
		return "", "", "", false
	}

	fnName := strings.ToUpper(sel.Sel.Name)
	httpMethods := map[string]string{
		"GET":        "GET",
		"POST":       "POST",
		"PUT":        "PUT",
		"DELETE":     "DELETE",
		"PATCH":      "PATCH",
		"HEAD":       "HEAD",
		"OPTIONS":    "OPTIONS",
		"HANDLEFUNC": "ANY",
		"HANDLE":     "ANY",
	}

	matchedMethod, isHTTP := httpMethods[fnName]
	if !isHTTP {
		return "", "", "", false
	}

	if len(call.Args) >= 1 {
		// First argument is usually the route path
		if lit, okLit := call.Args[0].(*ast.BasicLit); okLit && lit.Kind == token.STRING {
			route = lit.Value
		}
	}

	if len(call.Args) >= 2 {
		// Second argument is usually the handler function
		switch h := call.Args[1].(type) {
		case *ast.Ident:
			handler = h.Name
		case *ast.SelectorExpr:
			handler = h.Sel.Name
		}
	}

	if route != "" {
		return route, matchedMethod, handler, true
	}

	return "", "", "", false
}

// extractGRPCRegistration detects pb.Register...Server(...)
func extractGRPCRegistration(call *ast.CallExpr) (serviceName string, ok bool) {
	sel, okSel := call.Fun.(*ast.SelectorExpr)
	if !okSel {
		return "", false
	}

	name := sel.Sel.Name
	if strings.HasPrefix(name, "Register") && strings.HasSuffix(name, "Server") {
		service := strings.TrimSuffix(strings.TrimPrefix(name, "Register"), "Server")
		return service, true
	}

	return "", false
}

// extractDBQuery detects database calls like Query, Exec, Find, Create, etc.
func extractDBQuery(call *ast.CallExpr) (query string, op string, ok bool) {
	sel, okSel := call.Fun.(*ast.SelectorExpr)
	if !okSel {
		return "", "", false
	}

	opName := sel.Sel.Name
	dbOps := map[string]bool{
		"Query":    true,
		"QueryRow": true,
		"Exec":     true,
		"ExecRow":  true,
		"Select":   true,
		"Find":     true,
		"Create":   true,
		"Save":     true,
		"Raw":      true,
		"Delete":   true,
	}

	if !dbOps[opName] {
		return "", "", false
	}

	// Try extracting query string if passed as first argument
	if len(call.Args) > 0 {
		if lit, okLit := call.Args[0].(*ast.BasicLit); okLit && lit.Kind == token.STRING {
			query = strings.Trim(lit.Value, "`\"")
		}
	}

	return query, opName, true
}

func extractSimpleCallName(call *ast.CallExpr) (string, bool) {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name, true
	case *ast.SelectorExpr:
		return fn.Sel.Name, true
	default:
		return "", false
	}
}

func extractQualifiedCall(call *ast.CallExpr) (pkgName, funcName string, ok bool) {
	sel, okSel := call.Fun.(*ast.SelectorExpr)
	if !okSel {
		return "", "", false
	}
	if ident, okIdent := sel.X.(*ast.Ident); okIdent {
		return ident.Name, sel.Sel.Name, true
	}
	return "", "", false
}

func extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return extractReceiverType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return extractReceiverType(t.X)
	default:
		return ""
	}
}

func exprToString(expr ast.Expr) string {
	if expr == nil {
		return "any"
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		return "any"
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	default:
		return fmt.Sprintf("%v", expr)
	}
}

