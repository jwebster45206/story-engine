package scenario

// NPC represents a non-player character in the game
type NPC struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`                  // e.g. "villager", "guard", "merchant"
	Disposition string   `json:"disposition"`           // e.g. "hostile", "neutral", "friendly"
	Description string   `json:"description,omitempty"` // short description or backstory
	IsImportant bool     `json:"important,omitempty"`   // whether this NPC is important to the story
	Location    string   `json:"location,omitempty"`    // where the NPC is currently located
	Items       []string `json:"items,omitempty"`       // items the NPC has or can give
}

// Scenario is the template for a roleplay game session.
type Scenario struct {
	Name             string              `json:"name"`                        // Name of the scenario
	FileName         string              `json:"file_name,omitempty"`         // Name of the file containing the scenario
	Story            string              `json:"story,omitempty"`             // Brief description of the scenario
	Rating           string              `json:"rating,omitempty"`            // Content rating of the scenario
	Locations        map[string]Location `json:"locations,omitempty"`         // Map of location names to Location objects
	Inventory        []string            `json:"inventory,omitempty"`         // Potential inventory items throughout the scenario
	NPCs             map[string]NPC      `json:"npcs,omitempty"`              // Map of NPC names to their data
	Scenes           map[string]Scene    `json:"scenes"`                      // Map of scene names to Scene objectsOpeningPrompt    string              `json:"opening_prompt,omitempty"`    // Initial prompt to start the scenario
	OpeningPrompt    string              `json:"opening_prompt,omitempty"`    // Initial prompt to start the scenario
	OpeningLocation  string              `json:"opening_location,omitempty"`  // Initial location for the user
	OpeningInventory []string            `json:"opening_inventory,omitempty"` // Initial inventory items for the user
	OpeningScene     string              `json:"opening_scene"`               // Which scene to start with

	Vars               map[string]string   `json:"vars,omitempty"`                // Custom variables for the scenario
	ContingencyPrompts []ContingencyPrompt `json:"contingency_prompts,omitempty"` // Conditional prompts for LLM
	ContingencyRules   []string            `json:"contingency_rules,omitempty"`   // Backend rules for LLM to follow
	GameEndPrompt      string              `json:"game_end_prompt,omitempty"`     // Optional instructions for writing a game ending
}

const (
	RatingG    = "G"     // Suitable for all ages
	RatingPG   = "PG"    // Parental guidance suggested
	RatingPG13 = "PG-13" // Parents strongly cautioned
	RatingR    = "R"     // Restricted to adults
)

// HasScene checks if a scene with the given name exists in the scenario
func (s *Scenario) HasScene(sceneName string) bool {
	if s.Scenes == nil {
		return false
	}
	_, exists := s.Scenes[sceneName]
	return exists
}
