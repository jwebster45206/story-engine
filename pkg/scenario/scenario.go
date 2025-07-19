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
	Name             string              `json:"name"`                        // Name of the scenario
	FileName         string              `json:"file_name,omitempty"`         // Name of the file containing the scenario
	Story            string              `json:"story,omitempty"`             // Brief description of the scenario
	Rating           ContentRating       `json:"rating,omitempty"`            // Content rating of the scenario
	Locations        map[string]Location `json:"locations,omitempty"`         // Map of location names to Location objects
	Inventory        []string            `json:"inventory,omitempty"`         // Potential inventory items throughout the scenario
	NPCs             map[string]NPC      `json:"npcs,omitempty"`              // Map of NPC names to their data
	Flags            map[string]string   `json:"flags,omitempty"`             // Map of flags and their values
	OpeningPrompt    string              `json:"opening_prompt,omitempty"`    // Initial prompt to start the scenario
	OpeningLocation  string              `json:"opening_location,omitempty"`  // Initial location for the user
	OpeningInventory []string            `json:"opening_inventory,omitempty"` // Initial inventory items for the user
	// TODO: Triggers
}

type ContentRating string

const (
	RatingG    ContentRating = "G"     // Suitable for all ages
	RatingPG   ContentRating = "PG"    // Parental guidance suggested
	RatingPG13 ContentRating = "PG-13" // Parents strongly cautioned
	RatingR    ContentRating = "R"     // Restricted to adults
)
