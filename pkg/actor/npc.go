package actor

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
