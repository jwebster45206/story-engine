package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jwebster45206/story-engine/pkg/actor"
)

// PC operations (filesystem-backed, returns PCSpec only)

func (r *RedisStorage) GetPCSpec(ctx context.Context, pcID string) (*actor.PCSpec, error) {
	// Construct the full path internally
	path := filepath.Join(r.dataDir, "pcs", pcID+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PC file: %w", err)
	}

	var spec actor.PCSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PC spec: %w", err)
	}

	// Ensure ID is set from the parameter
	spec.ID = pcID

	return &spec, nil
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
