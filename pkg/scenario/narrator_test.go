package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNarratorGetPromptsAsString(t *testing.T) {
	tests := []struct {
		name     string
		narrator Narrator
		expected string
	}{
		{
			name: "single prompt",
			narrator: Narrator{
				ID:      "test",
				Prompts: []string{"You are dramatic."},
			},
			expected: "- You are dramatic.\n",
		},
		{
			name: "multiple prompts",
			narrator: Narrator{
				ID: "test",
				Prompts: []string{
					"You are dramatic.",
					"You use vivid language.",
				},
			},
			expected: "- You are dramatic.\n- You use vivid language.\n",
		},
		{
			name: "empty prompts",
			narrator: Narrator{
				ID:      "test",
				Prompts: []string{},
			},
			expected: "",
		},
		{
			name: "nil prompts",
			narrator: Narrator{
				ID:      "test",
				Prompts: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.narrator.GetPromptsAsString()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLoadNarrator(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	narratorsDir := filepath.Join(dataDir, "narrators")
	if err := os.MkdirAll(narratorsDir, 0755); err != nil {
		t.Fatalf("failed to create temp narrators dir: %v", err)
	}

	// Create test narrator file
	testNarrator := Narrator{
		ID:          "test_narrator",
		Name:        "Test Narrator",
		Description: "A test narrator",
		Prompts:     []string{"Prompt 1", "Prompt 2"},
	}

	data, _ := json.MarshalIndent(testNarrator, "", "  ")
	narratorPath := filepath.Join(narratorsDir, "test_narrator.json")
	if err := os.WriteFile(narratorPath, data, 0644); err != nil {
		t.Fatalf("failed to write test narrator file: %v", err)
	}

	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Test loading existing narrator
	t.Run("load existing narrator", func(t *testing.T) {
		narrator, err := LoadNarrator("test_narrator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if narrator == nil {
			t.Fatal("expected narrator, got nil")
		}
		if narrator.ID != "test_narrator" {
			t.Errorf("expected ID 'test_narrator', got %q", narrator.ID)
		}
		if len(narrator.Prompts) != 2 {
			t.Errorf("expected 2 prompts, got %d", len(narrator.Prompts))
		}
	})

	// Test loading non-existent narrator
	t.Run("load non-existent narrator", func(t *testing.T) {
		_, err := LoadNarrator("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent narrator, got nil")
		}
		if !strings.Contains(err.Error(), "narrator not found") {
			t.Errorf("expected 'narrator not found' error, got: %v", err)
		}
	})

	// Test empty narrator ID
	t.Run("empty narrator ID", func(t *testing.T) {
		narrator, err := LoadNarrator("")
		if err != nil {
			t.Errorf("unexpected error for empty ID: %v", err)
		}
		if narrator != nil {
			t.Error("expected nil narrator for empty ID")
		}
	})

	// Test ID mismatch
	t.Run("ID mismatch", func(t *testing.T) {
		dataDir := filepath.Join(tempDir, "data")
		narratorsDir := filepath.Join(dataDir, "narrators")

		mismatchNarrator := Narrator{
			ID:      "wrong_id",
			Name:    "Mismatch",
			Prompts: []string{"Test"},
		}
		data, _ := json.MarshalIndent(mismatchNarrator, "", "  ")
		mismatchPath := filepath.Join(narratorsDir, "correct_filename.json")
		if err := os.WriteFile(mismatchPath, data, 0644); err != nil {
			t.Fatalf("failed to write mismatch test file: %v", err)
		}

		_, err := LoadNarrator("correct_filename")
		if err == nil {
			t.Error("expected error for ID mismatch, got nil")
		}
		if !strings.Contains(err.Error(), "mismatch") {
			t.Errorf("expected 'mismatch' error, got: %v", err)
		}
	})
}

func TestListNarrators(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	narratorsDir := filepath.Join(dataDir, "narrators")
	if err := os.MkdirAll(narratorsDir, 0755); err != nil {
		t.Fatalf("failed to create temp narrators dir: %v", err)
	}

	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Create test narrator files
	for _, id := range []string{"classic", "noir", "comedic"} {
		narrator := Narrator{ID: id, Name: id, Prompts: []string{"test"}}
		data, _ := json.MarshalIndent(narrator, "", "  ")
		path := filepath.Join(narratorsDir, id+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("failed to write test narrator: %v", err)
		}
	}

	// Test listing narrators
	t.Run("list narrators", func(t *testing.T) {
		narrators, err := ListNarrators()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(narrators) != 3 {
			t.Errorf("expected 3 narrators, got %d", len(narrators))
		}

		// Check that all expected IDs are present
		narratorMap := make(map[string]bool)
		for _, n := range narrators {
			narratorMap[n] = true
		}

		for _, expected := range []string{"classic", "noir", "comedic"} {
			if !narratorMap[expected] {
				t.Errorf("expected narrator %q not found in list", expected)
			}
		}
	})

	// Test non-existent directory
	t.Run("non-existent directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		if err := os.Chdir(emptyDir); err != nil {
			t.Fatalf("failed to change to empty dir: %v", err)
		}

		narrators, err := ListNarrators()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(narrators) != 0 {
			t.Errorf("expected 0 narrators, got %d", len(narrators))
		}
	})
}

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name             string
		narrator         *Narrator
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "with narrator prompts",
			narrator: &Narrator{
				ID:      "test",
				Prompts: []string{"You are dramatic.", "You use vivid language."},
			},
			shouldContain: []string{
				"- You are dramatic.",
				"- You use vivid language.",
				"omniscient narrator",
			},
		},
		{
			name:     "without narrator",
			narrator: nil,
			shouldContain: []string{
				"omniscient narrator",
				"### Narrator responses:",
			},
			shouldNotContain: []string{
				"- You are",
			},
		},
		{
			name: "empty narrator prompts",
			narrator: &Narrator{
				ID:      "empty",
				Prompts: []string{},
			},
			shouldContain: []string{
				"omniscient narrator",
			},
			shouldNotContain: []string{
				"- You are",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildSystemPrompt(tt.narrator)

			for _, phrase := range tt.shouldContain {
				if !strings.Contains(result, phrase) {
					t.Errorf("expected prompt to contain %q", phrase)
				}
			}

			for _, phrase := range tt.shouldNotContain {
				if strings.Contains(result, phrase) {
					t.Errorf("expected prompt to NOT contain %q", phrase)
				}
			}
		})
	}
}

func TestGetContentRatingPrompt(t *testing.T) {
	tests := []struct {
		rating        string
		shouldContain string
	}{
		{RatingG, "young children"},
		{RatingPG, "children and families"},
		{RatingPG13, "teenagers"},
		{RatingR, "adult audiences"},
		{"", "teenagers"},        // default
		{"UNKNOWN", "teenagers"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.rating, func(t *testing.T) {
			result := GetContentRatingPrompt(tt.rating)
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("expected rating prompt for %q to contain %q, got: %s", tt.rating, tt.shouldContain, result)
			}
		})
	}
}
