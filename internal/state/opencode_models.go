package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/model"
)

const openCodeModelsFile = "opencode-models.json"

// LoadOpenCodeModels reads OpenCode model assignments from ~/.cortex-ia/opencode-models.json.
func LoadOpenCodeModels(homeDir string) (model.OpenCodeModelAssignments, error) {
	path := filepath.Join(homeDir, ".cortex-ia", openCodeModelsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var assignments model.OpenCodeModelAssignments
	if err := json.Unmarshal(data, &assignments); err != nil {
		return nil, err
	}
	return assignments, nil
}

// SaveOpenCodeModels writes OpenCode model assignments to ~/.cortex-ia/opencode-models.json.
func SaveOpenCodeModels(homeDir string, assignments model.OpenCodeModelAssignments) error {
	dir := filepath.Join(homeDir, ".cortex-ia")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(assignments, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, openCodeModelsFile), data, 0644)
}
