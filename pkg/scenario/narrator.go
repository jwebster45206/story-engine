package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Narrator defines the voice and style of the game narrator
type Narrator struct {
	ID          string   `json:"id"`                    // Unique identifier (e.g., "vincent_price", "classic", "comedic")
	Name        string   `json:"name"`                  // Display name
	Description string   `json:"description,omitempty"` // What this narrator style is like (not used in prompts)
	Prompts     []string `json:"prompts"`               // The actual narrator instructions
}

// GetPromptsAsString returns all narrator prompts joined with newlines and bullet points
func (n *Narrator) GetPromptsAsString() string {
	if len(n.Prompts) == 0 {
		return ""
	}

	result := ""
	for _, prompt := range n.Prompts {
		result += "- " + prompt + "\n"
	}
	return result
}

// LoadNarrator loads a narrator from a JSON file by ID from the data/narrators directory
func LoadNarrator(narratorID string) (*Narrator, error) {
	if narratorID == "" {
		return nil, nil // No narrator specified, return nil (not an error)
	}

	narratorPath := filepath.Join("./data/narrators", narratorID+".json")

	// Get absolute path for better error messages
	absPath, _ := filepath.Abs(narratorPath)

	data, err := os.ReadFile(narratorPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Get current working directory for debugging
			cwd, _ := os.Getwd()
			return nil, fmt.Errorf("narrator not found: %s (tried: %s, cwd: %s)", narratorID, absPath, cwd)
		}
		return nil, fmt.Errorf("failed to read narrator file %s: %w", narratorPath, err)
	}

	var narrator Narrator
	if err := json.Unmarshal(data, &narrator); err != nil {
		return nil, fmt.Errorf("failed to parse narrator JSON from %s: %w", narratorPath, err)
	}
	narrator.ID = narratorID // Ensure ID is set from filename

	return &narrator, nil
}

// ListNarrators returns all available narrator IDs in the data/narrators directory
func ListNarrators() ([]string, error) {
	narratorsPath := "./data/narrators"

	entries, err := os.ReadDir(narratorsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No narrators directory is fine
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
