package actor

import (
	"encoding/json"
	"testing"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

func TestPCSpec_MarshalJSON_WithContingencyPrompts(t *testing.T) {
	minTurns := 5
	pc := &PC{
		Spec: &PCSpec{
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
		},
	}

	// Create actor for marshaling
	attrs := pc.Spec.Stats.ToAttributes()
	pc.Actor, _ = d20.NewActor(pc.Spec.Name).
		WithHP(pc.Spec.MaxHP).
		WithAC(pc.Spec.AC).
		WithAttributes(attrs).
		Build()

	jsonData, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify contingency prompts are in the JSON
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	prompts, ok := result["contingency_prompts"]
	if !ok {
		t.Error("JSON should contain contingency_prompts field")
	}

	promptsArray, ok := prompts.([]interface{})
	if !ok || len(promptsArray) != 2 {
		t.Errorf("contingency_prompts should be an array of length 2, got %v", prompts)
	}
}

func TestPCSpec_UnmarshalJSON_WithContingencyPrompts(t *testing.T) {
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

	if len(pc.Spec.ContingencyPrompts) != 2 {
		t.Fatalf("Expected 2 contingency prompts, got %d", len(pc.Spec.ContingencyPrompts))
	}

	// Check first prompt
	if pc.Spec.ContingencyPrompts[0].Prompt != "Simple string prompt" {
		t.Errorf("First prompt = %q, want %q", pc.Spec.ContingencyPrompts[0].Prompt, "Simple string prompt")
	}
	if pc.Spec.ContingencyPrompts[0].When != nil {
		t.Error("First prompt should have nil When clause")
	}

	// Check second prompt
	if pc.Spec.ContingencyPrompts[1].Prompt != "Complex conditional prompt" {
		t.Errorf("Second prompt = %q, want %q", pc.Spec.ContingencyPrompts[1].Prompt, "Complex conditional prompt")
	}
	if pc.Spec.ContingencyPrompts[1].When == nil {
		t.Fatal("Second prompt should have non-nil When clause")
	}
	if pc.Spec.ContingencyPrompts[1].When.Vars["test_var"] != "test_value" {
		t.Error("Second prompt When clause should have test_var=test_value")
	}
}
