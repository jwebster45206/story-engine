package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
)

// PC operations (filesystem-backed, returns PCSpec only)

func (r *RedisStorage) GetPCSpec(ctx context.Context, path string) (*actor.PCSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PC file: %w", err)
	}

	var spec actor.PCSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PC spec: %w", err)
	}

	// Filename overrides any ID in the JSON
	spec.ID = strings.TrimSuffix(filepath.Base(path), ".json")

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
