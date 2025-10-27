package conditionals

// GameStateDelta is a compact, structured representation of changes to game state.
// It is intentionally smaller and stricter than the full game state.
type GameStateDelta struct {
	UserLocation string `json:"user_location"`
	SceneChange  *struct {
		To     string `json:"to"`
		Reason string `json:"reason"`
	} `json:"scene_change,omitempty"`

	ItemEvents []struct {
		Item   string `json:"item"`
		Action string `json:"action"` // enum "acquire" | "give" | "drop" | "move" | "use"
		From   *struct {
			Type string `json:"type"` // enum "player" | "npc" | "location"
			Name string `json:"name,omitempty"`
		} `json:"from,omitempty"`
		To *struct {
			Type string `json:"type"` // enum "player" | "npc" | "location"
			Name string `json:"name,omitempty"`
		} `json:"to,omitempty"`
		Consumed *bool `json:"consumed,omitempty"`
	} `json:"item_events,omitempty"`

	NPCEvents      []NPCEvent      `json:"npc_events,omitempty"`
	LocationEvents []LocationEvent `json:"location_events,omitempty"`

	SetVars   map[string]string `json:"set_vars,omitempty"`
	GameEnded *bool             `json:"game_ended,omitempty"`
	Prompt    *string           `json:"prompt,omitempty"` // Narrative prompt to inject (optional "STORY EVENT: " prefix for clarity)
}

// NPCEvent represents changes to an NPC's state
type NPCEvent struct {
	NPCID          string `json:"npc_id"`
	LocationChange *struct {
		To     string `json:"to"`     // Location ID to move to
		Reason string `json:"reason"` // Reason for the move
	} `json:"location_change,omitempty"`
}

// LocationEvent represents changes to a location's state
type LocationEvent struct {
	LocationID  string       `json:"location_id"`
	ExitChanges []ExitChange `json:"exit_changes,omitempty"`
}

// ExitChange represents a change to an exit's availability
type ExitChange struct {
	ExitID string `json:"exit_id"`          // The exit identifier (e.g., "north", "secret door")
	Status string `json:"status"`           // "blocked" or "unblocked"
	Reason string `json:"reason,omitempty"` // Optional reason for the change
}
