package character

import (
	"encoding/json"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

func TestStats5e_ToAttributes(t *testing.T) {
	stats := Stats5e{
		Strength:     16,
		Dexterity:    14,
		Constitution: 15,
		Intelligence: 10,
		Wisdom:       12,
		Charisma:     8,
	}

	attrs := stats.ToAttributes()

	tests := []struct {
		key      string
		expected int
	}{
		{"strength", 16},
		{"dexterity", 14},
		{"constitution", 15},
		{"intelligence", 10},
		{"wisdom", 12},
		{"charisma", 8},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := attrs[tt.key]; got != tt.expected {
				t.Errorf("ToAttributes()[%q] = %d, want %d", tt.key, got, tt.expected)
			}
		})
	}
}

func TestPC_MarshalJSON(t *testing.T) {
	pc := &PC{
		ID:          "test_pc",
		Name:        "Test Character",
		Class:       "Rogue",
		Level:       3,
		Race:        "Elf",
		Pronouns:    "she/her",
		Description: "A test character",
		Background:  "Test background",
		Stats: Stats5e{
			Strength:     10,
			Dexterity:    18,
			Constitution: 12,
			Intelligence: 14,
			Wisdom:       13,
			Charisma:     16,
		},
		HP:    20,
		MaxHP: 20,
		AC:    15,
		CombatModifiers: map[string]int{
			"dexterity":   4,
			"proficiency": 2,
		},
		Attributes: map[string]int{
			"stealth":    7,
			"perception": 5,
		},
		Inventory: []string{"dagger", "thieves' tools"},
	}

	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result["id"] != "test_pc" {
		t.Errorf("Marshaled id = %v, want %q", result["id"], "test_pc")
	}
	if result["name"] != "Test Character" {
		t.Errorf("Marshaled name = %v, want %q", result["name"], "Test Character")
	}
	if result["class"] != "Rogue" {
		t.Errorf("Marshaled class = %v, want %q", result["class"], "Rogue")
	}
	if result["pronouns"] != "she/her" {
		t.Errorf("Marshaled pronouns = %v, want %q", result["pronouns"], "she/her")
	}

	if hp, ok := result["hp"].(float64); !ok || int(hp) != 20 {
		t.Errorf("Marshaled hp = %v, want %d", result["hp"], 20)
	}
	if ac, ok := result["ac"].(float64); !ok || int(ac) != 15 {
		t.Errorf("Marshaled ac = %v, want %d", result["ac"], 15)
	}

	stats, ok := result["stats"].(map[string]any)
	if !ok {
		t.Fatal("Marshaled stats missing or wrong type")
	}
	if strength, ok := stats["strength"].(float64); !ok || int(strength) != 10 {
		t.Errorf("Marshaled stats.strength = %v, want %d", stats["strength"], 10)
	}

	attrs, ok := result["attributes"].(map[string]any)
	if !ok {
		t.Fatal("Marshaled attributes missing or wrong type")
	}
	if _, exists := attrs["strength"]; exists {
		t.Error("Marshaled attributes should not include core stats like 'strength'")
	}
	if stealth, ok := attrs["stealth"].(float64); !ok || int(stealth) != 7 {
		t.Errorf("Marshaled attributes.stealth = %v, want %d", attrs["stealth"], 7)
	}

	inv, ok := result["inventory"].([]any)
	if !ok {
		t.Fatal("Marshaled inventory missing or wrong type")
	}
	if len(inv) != 2 {
		t.Errorf("Marshaled inventory has %d items, want 2", len(inv))
	}
}

func TestPC_MarshalJSON_NilPC(t *testing.T) {
	var pc *PC
	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("MarshalJSON() with nil PC error = %v", err)
	}
	if string(data) != "null" {
		t.Errorf("MarshalJSON() with nil PC = %q, want %q", string(data), "null")
	}
}

func TestPC_MarshalUnmarshalRoundTrip(t *testing.T) {
	original := &PC{
		ID:          "test_pc",
		Name:        "Test Ranger",
		Class:       "Ranger",
		Level:       5,
		Race:        "Wood Elf",
		Pronouns:    "they/them",
		Description: "A skilled tracker",
		Background:  "Outlander",
		Stats: Stats5e{
			Strength:     14,
			Dexterity:    18,
			Constitution: 13,
			Intelligence: 10,
			Wisdom:       16,
			Charisma:     12,
		},
		HP:    35,
		MaxHP: 40,
		AC:    16,
		CombatModifiers: map[string]int{
			"dexterity":   4,
			"proficiency": 3,
		},
		Attributes: map[string]int{
			"survival":   8,
			"perception": 7,
			"stealth":    7,
		},
		Inventory: []string{"longbow", "arrows", "rope"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored PC
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID = %q, want %q", restored.ID, original.ID)
	}
	if restored.Name != original.Name {
		t.Errorf("Name = %q, want %q", restored.Name, original.Name)
	}
	if restored.Class != original.Class {
		t.Errorf("Class = %q, want %q", restored.Class, original.Class)
	}
	if restored.HP != original.HP {
		t.Errorf("HP = %d, want %d", restored.HP, original.HP)
	}
	if restored.MaxHP != original.MaxHP {
		t.Errorf("MaxHP = %d, want %d", restored.MaxHP, original.MaxHP)
	}
	if restored.AC != original.AC {
		t.Errorf("AC = %d, want %d", restored.AC, original.AC)
	}
	if restored.Stats.Dexterity != 18 {
		t.Errorf("Stats.Dexterity = %d, want 18", restored.Stats.Dexterity)
	}
	if restored.Attributes["survival"] != 8 {
		t.Errorf("Attributes[survival] = %d, want 8", restored.Attributes["survival"])
	}
	if len(restored.CombatModifiers) != 2 {
		t.Errorf("CombatModifiers count = %d, want 2", len(restored.CombatModifiers))
	}
}

func TestPC_UnmarshalJSON_IDOnly(t *testing.T) {
	var pc PC
	if err := json.Unmarshal([]byte(`{"id":"pirate_captain"}`), &pc); err != nil {
		t.Fatalf("Unmarshal id-only PC: %v", err)
	}
	if pc.ID != "pirate_captain" {
		t.Errorf("ID = %q, want %q", pc.ID, "pirate_captain")
	}
}

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name string
		pc   *PC
		want string
	}{
		{
			name: "nil PC returns empty string",
			pc:   nil,
			want: "",
		},
		{
			name: "PC with all fields",
			pc: &PC{
				Name:        "Sir Galahad",
				Pronouns:    "he/him",
				Level:       5,
				Class:       "Paladin",
				Description: "A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.",
			},
			want: "REMEMBER: In this game, the user is controlling: Sir Galahad (he/him), Level 5 Paladin. A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.",
		},
		{
			name: "PC without pronouns",
			pc: &PC{
				Name:        "Aragorn",
				Level:       10,
				Class:       "Ranger",
				Description: "A skilled ranger and heir to the throne of Gondor.",
			},
			want: "REMEMBER: In this game, the user is controlling: Aragorn, Level 10 Ranger. A skilled ranger and heir to the throne of Gondor.",
		},
		{
			name: "PC without level",
			pc: &PC{
				Name:        "Gandalf",
				Pronouns:    "he/him",
				Class:       "Wizard",
				Description: "A wise wizard of great power.",
			},
			want: "REMEMBER: In this game, the user is controlling: Gandalf (he/him), Wizard. A wise wizard of great power.",
		},
		{
			name: "PC without class",
			pc: &PC{
				Name:        "Frodo",
				Pronouns:    "he/him",
				Level:       3,
				Description: "A brave hobbit carrying a heavy burden.",
			},
			want: "REMEMBER: In this game, the user is controlling: Frodo (he/him), Level 3. A brave hobbit carrying a heavy burden.",
		},
		{
			name: "PC without level or class",
			pc: &PC{
				Name:        "Samwise",
				Pronouns:    "he/him",
				Description: "A loyal friend and companion.",
			},
			want: "REMEMBER: In this game, the user is controlling: Samwise (he/him). A loyal friend and companion.",
		},
		{
			name: "PC without description",
			pc: &PC{
				Name:     "Gimli",
				Pronouns: "he/him",
				Level:    8,
				Class:    "Fighter",
			},
			want: "REMEMBER: In this game, the user is controlling: Gimli (he/him), Level 8 Fighter",
		},
		{
			name: "PC with name only",
			pc: &PC{
				Name: "Legolas",
			},
			want: "REMEMBER: In this game, the user is controlling: Legolas",
		},
		{
			name: "PC with class but no level",
			pc: &PC{
				Name:  "Boromir",
				Class: "Fighter",
			},
			want: "REMEMBER: In this game, the user is controlling: Boromir, Fighter",
		},
		{
			name: "PC with level zero but has class",
			pc: &PC{
				Name:  "Young Apprentice",
				Level: 0,
				Class: "Wizard",
			},
			want: "REMEMBER: In this game, the user is controlling: Young Apprentice, Wizard",
		},
		{
			name: "PC with Race",
			pc: &PC{
				Name:        "Fooman",
				Pronouns:    "hi/him",
				Level:       4,
				Race:        "Human",
				Class:       "Rogue",
				Description: "A strange dude with a mysterious past.",
			},
			want: "REMEMBER: In this game, the user is controlling: Fooman (hi/him), Level 4 Human Rogue. A strange dude with a mysterious past.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPrompt(tt.pc)
			if got != tt.want {
				t.Errorf("BuildPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPC_MarshalJSON_WithContingencyPrompts(t *testing.T) {
	minTurns := 5
	pc := &PC{
		ID:       "test_pc",
		Name:     "Test",
		Pronouns: "they/them",
		Stats: Stats5e{
			Strength:     10,
			Dexterity:    10,
			Constitution: 10,
			Intelligence: 10,
			Wisdom:       10,
			Charisma:     10,
		},
		HP:    10,
		MaxHP: 10,
		AC:    10,
		ContingencyPrompts: []conditionals.ContingencyPrompt{
			{Prompt: "Always active"},
			{
				Prompt: "Conditional prompt",
				When:   &conditionals.ConditionalWhen{MinTurns: &minTurns},
			},
		},
	}

	jsonData, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	prompts, ok := result["contingency_prompts"]
	if !ok {
		t.Error("JSON should contain contingency_prompts field")
	}

	promptsArray, ok := prompts.([]any)
	if !ok || len(promptsArray) != 2 {
		t.Errorf("contingency_prompts should be an array of length 2, got %v", prompts)
	}
}

func TestPC_UnmarshalJSON_WithContingencyPrompts(t *testing.T) {
	jsonData := []byte(`{
		"id": "test",
		"name": "Test Character",
		"pronouns": "they/them",
		"stats": {"strength": 10, "dexterity": 10, "constitution": 10, "intelligence": 10, "wisdom": 10, "charisma": 10},
		"hp": 10,
		"max_hp": 10,
		"ac": 10,
		"contingency_prompts": [
			"Simple string prompt",
			{
				"prompt": "Complex conditional prompt",
				"when": {"vars": {"test_var": "test_value"}}
			}
		]
	}`)

	var pc PC
	if err := json.Unmarshal(jsonData, &pc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(pc.ContingencyPrompts) != 2 {
		t.Fatalf("Expected 2 contingency prompts, got %d", len(pc.ContingencyPrompts))
	}

	if pc.ContingencyPrompts[0].Prompt != "Simple string prompt" {
		t.Errorf("First prompt = %q, want %q", pc.ContingencyPrompts[0].Prompt, "Simple string prompt")
	}
	if pc.ContingencyPrompts[0].When != nil {
		t.Error("First prompt should have nil When clause")
	}

	if pc.ContingencyPrompts[1].Prompt != "Complex conditional prompt" {
		t.Errorf("Second prompt = %q, want %q", pc.ContingencyPrompts[1].Prompt, "Complex conditional prompt")
	}
	if pc.ContingencyPrompts[1].When == nil {
		t.Fatal("Second prompt should have non-nil When clause")
	}
	if pc.ContingencyPrompts[1].When.Vars["test_var"] != "test_value" {
		t.Error("Second prompt When clause should have test_var=test_value")
	}
}
