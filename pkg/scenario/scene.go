package scenario

// Scene represents a single scene within a scenario with its own locations, NPCs, and rules
type Scene struct {
	Story              string              `json:"story"`               // Description of what happens in this scene
	Locations          map[string]Location `json:"locations"`           // Map of location names to Location objects for this scene
	NPCs               map[string]NPC      `json:"npcs"`                // Map of NPC names to their data for this scene
	Vars               map[string]string   `json:"vars"`                // Scene-specific variables
	ContingencyPrompts []string            `json:"contingency_prompts"` // Conditional prompts for LLM in this scene
	ContingencyRules   []string            `json:"contingency_rules"`   // Backend rules for LLM to follow in this scene
}
