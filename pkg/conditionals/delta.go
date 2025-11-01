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

	NPCEvents []NPCEvent `json:"npc_events,omitempty"`

	// TODO: Maybe add LocationEvents structure to track stateful elements of locations:
	// such as exits being blocked/unblocked, conditions changing, etc.

	SetVars   map[string]string `json:"set_vars,omitempty"`
	GameEnded *bool             `json:"game_ended,omitempty"`
	Prompt    *string           `json:"prompt,omitempty"` // Narrative prompt to inject (use "STORY EVENT: " prefix for story events)
}

// NPCEvent represents a change to an NPC's state
type NPCEvent struct {
	NPCID        string  `json:"npc_id"`
	SetLocation  *string `json:"set_location,omitempty"`  // Set NPC to specific location
	SetFollowing *string `json:"set_following,omitempty"` // Set following target ("pc", npc_id, or "" to clear).
}
