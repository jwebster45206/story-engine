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
	Name             string            `json:"name"`              // Name of the scenario
	FileName         string            `json:"file_name"`         // Name of the file containing the scenario
	Story            string            `json:"story"`             // Brief description of the scenario
	Locations        map[string]string `json:"locations"`         // Map of location names to descriptions
	Inventory        []string          `json:"inventory"`         // Potential inventory items throughout the scenario
	NPCs             map[string]NPC    `json:"npcs"`              // Map of NPC names to their data
	Flags            map[string]string `json:"flags"`             // Map of flags and their values
	Triggers         []string          `json:"triggers"`          // List of triggers for the scenario
	OpeningPrompt    string            `json:"opening_prompt"`    // Initial prompt to start the scenario
	OpeningLocation  string            `json:"opening_location"`  // Initial location for the user
	OpeningInventory []string          `json:"opening_inventory"` // Initial inventory items for the user
}
