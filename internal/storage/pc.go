package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jwebster45206/story-engine/pkg/character"
)

// PC operations (filesystem-backed)

func (r *RedisStorage) GetPC(ctx context.Context, pcID string) (*character.PC, error) {
	// Construct the full path internally
	path := filepath.Join(r.dataDir, "pcs", pcID+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PC file: %w", err)
	}

	var pc character.PC
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PC: %w", err)
	}

	// Ensure ID is set from the parameter
	pc.ID = pcID

	return &pc, nil
}

func (r *RedisStorage) ListPCs(ctx context.Context) ([]string, error) {
	pcsPath := filepath.Join(r.dataDir, "pcs")

	entries, err := os.ReadDir(pcsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read PCs directory: %w", err)
	}

	var pcIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			pcID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			pcIDs = append(pcIDs, pcID)
		}
	}

	return pcIDs, nil
}
