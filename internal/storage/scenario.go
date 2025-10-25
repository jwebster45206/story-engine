package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// Scenario operations (filesystem-backed)

func (r *RedisStorage) ListScenarios(ctx context.Context) (map[string]string, error) {
	scenariosDir := filepath.Join(r.dataDir, "scenarios")
	scenarios := make(map[string]string)

	err := filepath.WalkDir(scenariosDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		file, err := os.ReadFile(path)
		if err != nil {
			r.logger.Warn("Failed to read scenario file", "path", path, "error", err)
			return nil
		}

		var s scenario.Scenario
		if err := json.Unmarshal(file, &s); err != nil {
			r.logger.Warn("Failed to unmarshal scenario file", "path", path, "error", err)
			return nil
		}

		filename := filepath.Base(path)
		scenarios[s.Name] = filename
		return nil
	})

	if err != nil {
		r.logger.Error("Failed to walk scenarios directory", "error", err)
		return nil, fmt.Errorf("failed to list scenarios: %w", err)
	}

	return scenarios, nil
}

func (r *RedisStorage) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
	path := filepath.Join(r.dataDir, "scenarios", filename)
	r.logger.Debug("Loading scenario", "filename", filename, "full_path", path, "dataDir", r.dataDir)

	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.Error("Scenario file not found", "path", path, "error", err)
			return nil, fmt.Errorf("scenario not found: %s", filename)
		}
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	var s scenario.Scenario
	if err := json.Unmarshal(file, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scenario: %w", err)
	}

	return &s, nil
}
