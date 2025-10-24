package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// Narrator operations (filesystem-backed)

func (r *RedisStorage) GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error) {
	if narratorID == "" {
		return nil, nil // No narrator specified
	}

	narratorPath := filepath.Join(r.dataDir, "narrators", narratorID+".json")

	data, err := os.ReadFile(narratorPath)
	if err != nil {
		if os.IsNotExist(err) {
			absPath, _ := filepath.Abs(narratorPath)
			cwd, _ := os.Getwd()
			return nil, fmt.Errorf("narrator not found: %s (tried: %s, cwd: %s)", narratorID, absPath, cwd)
		}
		return nil, fmt.Errorf("failed to read narrator file %s: %w", narratorPath, err)
	}

	var narrator scenario.Narrator
	if err := json.Unmarshal(data, &narrator); err != nil {
		return nil, fmt.Errorf("failed to parse narrator JSON from %s: %w", narratorPath, err)
	}
	narrator.ID = narratorID // Ensure ID is set from filename

	return &narrator, nil
}

func (r *RedisStorage) ListNarrators(ctx context.Context) ([]string, error) {
	narratorsPath := filepath.Join(r.dataDir, "narrators")

	entries, err := os.ReadDir(narratorsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read narrators directory: %w", err)
	}

	var narratorIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			narratorID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			narratorIDs = append(narratorIDs, narratorID)
		}
	}

	return narratorIDs, nil
}
