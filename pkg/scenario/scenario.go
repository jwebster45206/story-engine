package scenario

// NPC represents a non-player character in the game
type NPC struct {
	Name        string `json:"name"`
	Type        string `json:"type"`                  // e.g. "villager", "guard", "merchant"
	Disposition string `json:"disposition"`           // e.g. "hostile", "neutral", "friendly"
	Description string `json:"description,omitempty"` // short description or backstory
	IsImportant bool   `json:"important,omitempty"`   // whether this NPC is important to the story
	Location    string `json:"location,omitempty"`    // where the NPC is currently located
}

// Scenario is the template for a roleplay game session.
type Scenario struct {
	Name          string            `json:"name"`           // Name of the scenario
	Story         string            `json:"story"`          // Brief description of the scenario
	Locations     map[string]string `json:"locations"`      // Map of location names to descriptions
	Inventory     []string          `json:"inventory"`      // Initial inventory items available to the user
	NPCs          map[string]NPC    `json:"npcs"`           // Map of NPC names to their data
	Flags         map[string]string `json:"flags"`          // Map of flags and their values
	Triggers      []string          `json:"triggers"`       // List of triggers for the scenario
	OpeningPrompt string            `json:"opening_prompt"` // Initial prompt to start the scenario
}

// PirateScenarioPrompt is a scenario prompt for testing the roleplay agent.
const PirateScenarioPrompt = `The user is the pirate captain in the Caribbean during the Golden Age of Piracy. The user's ship, The Black Pearl, has just docked at Tortuga, a notorious pirate haven. The crew is eager for adventure and treasure.`

const MermaidLagoonPrompt = `The user is in a mermaid lagoon, a magical place filled with mermaids. The lagoon is surrounded by lush greenery and the sound of water splashing fills the air.`
