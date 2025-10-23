package actor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jwebster45206/d20"
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

func TestLoadPC(t *testing.T) {
	// Create a temporary test PC file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_fighter.json")

	testPC := PCSpec{
		ID:          "should_be_overridden",
		Name:        "Test Fighter",
		Class:       "Fighter",
		Level:       1,
		Race:        "Human",
		Pronouns:    "they/them",
		Description: "A test character",
		Background:  "Test background",
		Stats: Stats5e{
			Strength:     16,
			Dexterity:    13,
			Constitution: 14,
			Intelligence: 10,
			Wisdom:       12,
			Charisma:     8,
		},
		HP:    12,
		MaxHP: 12,
		AC:    16,
		CombatModifiers: map[string]int{
			"strength":    3,
			"proficiency": 2,
		},
		Attributes: map[string]int{
			"athletics":  5,
			"perception": 3,
		},
		Inventory: []string{"longsword", "shield"},
	}

	data, err := json.Marshal(testPC)
	if err != nil {
		t.Fatalf("Failed to marshal test PC: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test loading
	pc, err := LoadPC(testFile)
	if err != nil {
		t.Fatalf("LoadPC() error = %v", err)
	}

	// Verify ID was overridden by filename
	if pc.Spec.ID != "test_fighter" {
		t.Errorf("PC.Spec.ID = %q, want %q", pc.Spec.ID, "test_fighter")
	}

	// Verify basic fields
	if pc.Spec.Name != "Test Fighter" {
		t.Errorf("PC.Spec.Name = %q, want %q", pc.Spec.Name, "Test Fighter")
	}

	if pc.Spec.Class != "Fighter" {
		t.Errorf("PC.Spec.Class = %q, want %q", pc.Spec.Class, "Fighter")
	}

	if pc.Spec.Level != 1 {
		t.Errorf("PC.Spec.Level = %d, want %d", pc.Spec.Level, 1)
	}

	if pc.Spec.Pronouns != "they/them" {
		t.Errorf("PC.Spec.Pronouns = %q, want %q", pc.Spec.Pronouns, "they/them")
	}

	// Verify Actor was built
	if pc.Actor == nil {
		t.Fatal("PC.Actor is nil, want non-nil")
	}

	// Verify Actor properties
	if pc.Actor.MaxHP() != 12 {
		t.Errorf("Actor.MaxHP() = %d, want %d", pc.Actor.MaxHP(), 12)
	}

	if pc.Actor.AC() != 16 {
		t.Errorf("Actor.AC() = %d, want %d", pc.Actor.AC(), 16)
	}

	// Verify core stats are in Actor attributes
	strength, ok := pc.Actor.Attribute("strength")
	if !ok {
		t.Error("Actor missing 'strength' attribute")
	}
	if strength != 16 {
		t.Errorf("Actor.Attribute('strength') = %d, want %d", strength, 16)
	}

	// Verify additional attributes are in Actor
	athletics, ok := pc.Actor.Attribute("athletics")
	if !ok {
		t.Error("Actor missing 'athletics' attribute")
	}
	if athletics != 5 {
		t.Errorf("Actor.Attribute('athletics') = %d, want %d", athletics, 5)
	}

	// Verify combat modifiers are in Actor
	mods := pc.Actor.GetCombatModifiers()
	if len(mods) != 2 {
		t.Errorf("Actor has %d combat modifiers, want 2", len(mods))
	}
}

func TestLoadPC_FileNotFound(t *testing.T) {
	_, err := LoadPC("/nonexistent/path/to/pc.json")
	if err == nil {
		t.Error("LoadPC() with nonexistent file should return error")
	}
}

func TestLoadPC_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "invalid.json")

	if err := os.WriteFile(testFile, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadPC(testFile)
	if err == nil {
		t.Error("LoadPC() with invalid JSON should return error")
	}
}

func TestLoadPC_InvalidActorData(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "bad_actor.json")

	// PC with invalid MaxHP (0 or negative should fail Actor creation)
	badPC := PCSpec{
		Name:  "Bad Fighter",
		Class: "Fighter",
		Level: 1,
		Race:  "Human",
		Stats: Stats5e{
			Strength: 10,
		},
		HP:    10,
		MaxHP: 0, // Invalid - must be > 0
		AC:    10,
	}

	data, err := json.Marshal(badPC)
	if err != nil {
		t.Fatalf("Failed to marshal test PC: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = LoadPC(testFile)
	if err == nil {
		t.Error("LoadPC() with invalid Actor data should return error")
	}
}

func TestPC_MarshalJSON(t *testing.T) {
	// Create a test PC
	spec := &PCSpec{
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

	// Build Actor
	allAttrs := spec.Stats.ToAttributes()
	for k, v := range spec.Attributes {
		allAttrs[k] = v
	}

	actor, err := d20.NewActor(spec.Name).
		WithHP(spec.HP).
		WithAC(spec.AC).
		WithAttributes(allAttrs).
		WithCombatModifiers(spec.CombatModifiers).
		Build()
	if err != nil {
		t.Fatalf("Failed to build actor: %v", err)
	}

	pc := &PC{
		Spec:  spec,
		Actor: actor,
	}

	// Marshal to JSON
	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Unmarshal to verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify key fields
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

	// Verify HP comes from Actor.MaxHP()
	if hp, ok := result["hp"].(float64); !ok || int(hp) != 20 {
		t.Errorf("Marshaled hp = %v, want %d", result["hp"], 20)
	}

	// Verify AC comes from Actor
	if ac, ok := result["ac"].(float64); !ok || int(ac) != 15 {
		t.Errorf("Marshaled ac = %v, want %d", result["ac"], 15)
	}

	// Verify stats are preserved
	stats, ok := result["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("Marshaled stats missing or wrong type")
	}
	if strength, ok := stats["strength"].(float64); !ok || int(strength) != 10 {
		t.Errorf("Marshaled stats.strength = %v, want %d", stats["strength"], 10)
	}

	// Verify attributes don't include core stats
	attrs, ok := result["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("Marshaled attributes missing or wrong type")
	}
	if _, exists := attrs["strength"]; exists {
		t.Error("Marshaled attributes should not include core stats like 'strength'")
	}
	if stealth, ok := attrs["stealth"].(float64); !ok || int(stealth) != 7 {
		t.Errorf("Marshaled attributes.stealth = %v, want %d", attrs["stealth"], 7)
	}

	// Verify inventory
	inv, ok := result["inventory"].([]interface{})
	if !ok {
		t.Fatal("Marshaled inventory missing or wrong type")
	}
	if len(inv) != 2 {
		t.Errorf("Marshaled inventory has %d items, want 2", len(inv))
	}
}

func TestPC_MarshalJSON_NilActor(t *testing.T) {
	// Test marshaling a PC with nil Actor (should not panic)
	spec := &PCSpec{
		ID:          "test_pc",
		Name:        "Test Character",
		Class:       "Fighter",
		Level:       1,
		Race:        "Human",
		Pronouns:    "he/him",
		Description: "A test character",
		HP:          10,
		MaxHP:       10,
		AC:          16,
	}

	pc := &PC{
		Spec:  spec,
		Actor: nil, // Explicitly nil
	}

	// This should not panic
	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("MarshalJSON() with nil Actor error = %v", err)
	}

	// Unmarshal to verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify basic fields are still present
	if result["id"] != "test_pc" {
		t.Errorf("Marshaled id = %v, want %q", result["id"], "test_pc")
	}

	if result["name"] != "Test Character" {
		t.Errorf("Marshaled name = %v, want %q", result["name"], "Test Character")
	}
}

func TestPC_MarshalJSON_NilPC(t *testing.T) {
	// Test marshaling a nil PC pointer (should not panic)
	var pc *PC = nil

	// This should not panic
	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("MarshalJSON() with nil PC error = %v", err)
	}

	// Should return "null"
	if string(data) != "null" {
		t.Errorf("MarshalJSON() with nil PC = %q, want %q", string(data), "null")
	}
}

func TestPC_MarshalUnmarshalRoundTrip(t *testing.T) {
	// Create a PC with full data
	spec := &PCSpec{
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

	// Build Actor
	allAttrs := spec.Stats.ToAttributes()
	for k, v := range spec.Attributes {
		allAttrs[k] = v
	}

	actor, err := d20.NewActor(spec.Name).
		WithHP(spec.MaxHP).
		WithAC(spec.AC).
		WithAttributes(allAttrs).
		WithCombatModifiers(spec.CombatModifiers).
		Build()
	if err != nil {
		t.Fatalf("Failed to build actor: %v", err)
	}

	// Set HP to different value
	if err := actor.SetHP(spec.HP); err != nil {
		t.Fatalf("Failed to set HP: %v", err)
	}

	originalPC := &PC{
		Spec:  spec,
		Actor: actor,
	}

	// Marshal to JSON
	data, err := json.Marshal(originalPC)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Unmarshal back
	var restoredPC PC
	if err := json.Unmarshal(data, &restoredPC); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Verify Spec fields
	if restoredPC.Spec.ID != spec.ID {
		t.Errorf("ID = %q, want %q", restoredPC.Spec.ID, spec.ID)
	}
	if restoredPC.Spec.Name != spec.Name {
		t.Errorf("Name = %q, want %q", restoredPC.Spec.Name, spec.Name)
	}
	if restoredPC.Spec.Class != spec.Class {
		t.Errorf("Class = %q, want %q", restoredPC.Spec.Class, spec.Class)
	}

	// Verify Actor was rebuilt
	if restoredPC.Actor == nil {
		t.Fatal("Actor is nil after unmarshal, should be rebuilt")
	}

	// Verify Actor properties
	if restoredPC.Actor.HP() != spec.HP {
		t.Errorf("Actor.HP() = %d, want %d", restoredPC.Actor.HP(), spec.HP)
	}
	if restoredPC.Actor.MaxHP() != spec.MaxHP {
		t.Errorf("Actor.MaxHP() = %d, want %d", restoredPC.Actor.MaxHP(), spec.MaxHP)
	}
	if restoredPC.Actor.AC() != spec.AC {
		t.Errorf("Actor.AC() = %d, want %d", restoredPC.Actor.AC(), spec.AC)
	}

	// Verify core stats
	if dex, ok := restoredPC.Actor.Attribute("dexterity"); !ok || dex != 18 {
		t.Errorf("Attribute('dexterity') = %d, %v, want 18, true", dex, ok)
	}

	// Verify additional attributes
	if survival, ok := restoredPC.Actor.Attribute("survival"); !ok || survival != 8 {
		t.Errorf("Attribute('survival') = %d, %v, want 8, true", survival, ok)
	}

	// Verify combat modifiers
	mods := restoredPC.Actor.GetCombatModifiers()
	if len(mods) != 2 {
		t.Errorf("CombatModifiers count = %d, want 2", len(mods))
	}
}

func TestLoadPC_RealFiles(t *testing.T) {
	// This test loads the actual PC files from data/pcs if they exist
	// Skip if files don't exist
	testFiles := []struct {
		filename string
		wantName string
		wantID   string
	}{
		{"../../data/pcs/classic.json", "Adventurer", "classic"},
		{"../../data/pcs/pirate_captain.json", "Captain Jack Sparrow", "pirate_captain"},
		{"../../data/pcs/alexandra_kane.json", "Alexandra Kane", "alexandra_kane"},
	}

	for _, tt := range testFiles {
		t.Run(tt.filename, func(t *testing.T) {
			if _, err := os.Stat(tt.filename); os.IsNotExist(err) {
				t.Skipf("Test file %s does not exist", tt.filename)
			}

			pc, err := LoadPC(tt.filename)
			if err != nil {
				t.Fatalf("LoadPC(%q) error = %v", tt.filename, err)
			}

			if pc.Spec.ID != tt.wantID {
				t.Errorf("PC.Spec.ID = %q, want %q", pc.Spec.ID, tt.wantID)
			}

			if pc.Spec.Name != tt.wantName {
				t.Errorf("PC.Spec.Name = %q, want %q", pc.Spec.Name, tt.wantName)
			}

			if pc.Actor == nil {
				t.Error("PC.Actor is nil, want non-nil")
			}

			// Verify all PCs have pronouns
			if pc.Spec.Pronouns == "" {
				t.Error("PC.Spec.Pronouns is empty, should be set")
			}
		})
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
				Spec: &PCSpec{
					Name:        "Sir Galahad",
					Pronouns:    "he/him",
					Level:       5,
					Class:       "Paladin",
					Description: "A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Sir Galahad (he/him), Level 5 Paladin. A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.",
		},
		{
			name: "PC without pronouns",
			pc: &PC{
				Spec: &PCSpec{
					Name:        "Aragorn",
					Level:       10,
					Class:       "Ranger",
					Description: "A skilled ranger and heir to the throne of Gondor.",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Aragorn, Level 10 Ranger. A skilled ranger and heir to the throne of Gondor.",
		},
		{
			name: "PC without level",
			pc: &PC{
				Spec: &PCSpec{
					Name:        "Gandalf",
					Pronouns:    "he/him",
					Class:       "Wizard",
					Description: "A wise wizard of great power.",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Gandalf (he/him), Wizard. A wise wizard of great power.",
		},
		{
			name: "PC without class",
			pc: &PC{
				Spec: &PCSpec{
					Name:        "Frodo",
					Pronouns:    "he/him",
					Level:       3,
					Description: "A brave hobbit carrying a heavy burden.",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Frodo (he/him), Level 3. A brave hobbit carrying a heavy burden.",
		},
		{
			name: "PC without level or class",
			pc: &PC{
				Spec: &PCSpec{
					Name:        "Samwise",
					Pronouns:    "he/him",
					Description: "A loyal friend and companion.",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Samwise (he/him). A loyal friend and companion.",
		},
		{
			name: "PC without description",
			pc: &PC{
				Spec: &PCSpec{
					Name:     "Gimli",
					Pronouns: "he/him",
					Level:    8,
					Class:    "Fighter",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Gimli (he/him), Level 8 Fighter",
		},
		{
			name: "PC with name only",
			pc: &PC{
				Spec: &PCSpec{
					Name: "Legolas",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Legolas",
		},
		{
			name: "PC with class but no level",
			pc: &PC{
				Spec: &PCSpec{
					Name:  "Boromir",
					Class: "Fighter",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Boromir, Fighter",
		},
		{
			name: "PC with level zero but has class",
			pc: &PC{
				Spec: &PCSpec{
					Name:  "Young Apprentice",
					Level: 0,
					Class: "Wizard",
				},
			},
			want: "REMEMBER: In this game, the user is controlling: Young Apprentice, Wizard",
		},
		{
			name: "PC with Race",
			pc: &PC{
				Spec: &PCSpec{
					Name:        "Fooman",
					Pronouns:    "hi/him",
					Level:       4,
					Race:        "Human",
					Class:       "Rogue",
					Description: "A strange dude with a mysterious past.",
				},
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

func TestBuildPrompt_WithActor(t *testing.T) {
	// Test that BuildPrompt works correctly even when Actor is built
	spec := &PCSpec{
		Name:        "Test Paladin",
		Pronouns:    "he/him",
		Level:       5,
		Class:       "Paladin",
		Description: "A holy warrior.",
		Stats: Stats5e{
			Strength:     16,
			Dexterity:    10,
			Constitution: 14,
			Intelligence: 8,
			Wisdom:       12,
			Charisma:     16,
		},
		HP:    40,
		MaxHP: 40,
		AC:    18,
	}

	actor, err := d20.NewActor(spec.Name).
		WithHP(spec.MaxHP).
		WithAC(spec.AC).
		WithAttributes(spec.Stats.ToAttributes()).
		Build()
	if err != nil {
		t.Fatalf("Failed to build actor: %v", err)
	}

	pc := &PC{
		Spec:  spec,
		Actor: actor,
	}

	got := BuildPrompt(pc)
	want := "REMEMBER: In this game, the user is controlling: Test Paladin (he/him), Level 5 Paladin. A holy warrior."

	if got != want {
		t.Errorf("BuildPrompt() = %q, want %q", got, want)
	}
}
