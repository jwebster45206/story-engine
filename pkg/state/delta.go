package state

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

	NPCMovements []NPCMovement `json:"npc_movements,omitempty"`

	SetVars   map[string]string `json:"set_vars,omitempty"`
	GameEnded *bool             `json:"game_ended,omitempty"`
}

// NPCMovement represents an NPC changing location
type NPCMovement struct {
	NPCID      string `json:"npc_id"`
	ToLocation string `json:"to_location"`
}
