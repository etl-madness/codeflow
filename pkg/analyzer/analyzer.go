package analyzer

import (
	"codeflow/pkg/model"
)

// FileAnalysisResult contains the extracted model elements from analyzing a single file.
type FileAnalysisResult struct {
	Steps     []model.Step
	Links     []model.Link
	Swimlanes []model.Swimlane
	ASTNodes  []*model.ASTNode
}

// Analyzer defines the contract for language-specific static analysis.
type Analyzer interface {
	Language() string
	CanAnalyze(extension string) bool
	AnalyzeFile(path string, content []byte) (*FileAnalysisResult, error)
}
