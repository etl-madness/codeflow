package json

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"codeflow/pkg/model"
)

// Exporter exports ProcessModel to JSON.
type Exporter struct{}

// New creates a new JSON Exporter.
func New() *Exporter {
	return &Exporter{}
}

// Export serializes the ProcessModel to the specified file path in JSON format.
func (e *Exporter) Export(pm *model.ProcessModel, outputPath string) error {
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ProcessModel to JSON: %w", err)
	}

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON output to %s: %w", outputPath, err)
	}

	return nil
}
