package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
)

func (r *RedisStorage) GetMonster(ctx context.Context, templateID string) (*actor.Monster, error) {
	path := filepath.Join(r.dataDir, "monsters", templateID+".json")
	r.logger.Debug("Loading monster template", "templateID", templateID, "full_path", path)

	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.Error("Monster template file not found", "path", path, "error", err)
			return nil, fmt.Errorf("monster template not found: %s", templateID)
		}
		return nil, fmt.Errorf("failed to read monster template file: %w", err)
	}

	var m actor.Monster
	if err := json.Unmarshal(file, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal monster template: %w", err)
	}

	return &m, nil
}

func (r *RedisStorage) ListMonsters(ctx context.Context) (map[string]string, error) {
	monstersDir := filepath.Join(r.dataDir, "monsters")
	monsters := make(map[string]string)

	if _, err := os.Stat(monstersDir); os.IsNotExist(err) {
		r.logger.Debug("Monsters directory does not exist", "path", monstersDir)
		return monsters, nil // Return empty map if directory doesn't exist
	}

	err := filepath.WalkDir(monstersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		file, err := os.ReadFile(path)
		if err != nil {
			r.logger.Warn("Failed to read monster file", "path", path, "error", err)
			return nil
		}

		var m actor.Monster
		if err := json.Unmarshal(file, &m); err != nil {
			r.logger.Warn("Failed to unmarshal monster file", "path", path, "error", err)
			return nil
		}

		templateID := strings.TrimSuffix(filepath.Base(path), ".json")
		monsters[m.Name] = templateID
		return nil
	})

	if err != nil {
		r.logger.Error("Failed to walk monsters directory", "error", err)
		return nil, fmt.Errorf("failed to list monsters: %w", err)
	}

	return monsters, nil
}
