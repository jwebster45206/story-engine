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

func (r *RedisStorage) GetNPC(ctx context.Context, templateID string) (*actor.NPC, error) {
	path := filepath.Join(r.dataDir, "npcs", templateID+".json")
	r.logger.Debug("Loading NPC template", "templateID", templateID, "full_path", path)

	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.Error("NPC template file not found", "path", path, "error", err)
			return nil, fmt.Errorf("npc template not found: %s", templateID)
		}
		return nil, fmt.Errorf("failed to read npc template file: %w", err)
	}

	var n actor.NPC
	if err := json.Unmarshal(file, &n); err != nil {
		return nil, fmt.Errorf("failed to unmarshal npc template: %w", err)
	}

	// Ensure the templateID is set from the filename
	n.TemplateID = templateID

	return &n, nil
}

func (r *RedisStorage) ListNPCs(ctx context.Context) (map[string]string, error) {
	npcsDir := filepath.Join(r.dataDir, "npcs")
	npcs := make(map[string]string)

	if _, err := os.Stat(npcsDir); os.IsNotExist(err) {
		r.logger.Debug("NPCs directory does not exist", "path", npcsDir)
		return npcs, nil // Return empty map if directory doesn't exist
	}

	err := filepath.WalkDir(npcsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		file, err := os.ReadFile(path)
		if err != nil {
			r.logger.Warn("Failed to read npc file", "path", path, "error", err)
			return nil
		}

		var n actor.NPC
		if err := json.Unmarshal(file, &n); err != nil {
			r.logger.Warn("Failed to unmarshal npc file", "path", path, "error", err)
			return nil
		}

		templateID := strings.TrimSuffix(filepath.Base(path), ".json")
		npcs[n.Name] = templateID
		return nil
	})

	if err != nil {
		r.logger.Error("Failed to walk npcs directory", "error", err)
		return nil, fmt.Errorf("failed to list npcs: %w", err)
	}

	return npcs, nil
}
