package state

// PromptState is a simplified representation of the game state
// packaged for sharing with a language model.
// Use of PromptState assumes that the model is capable of understanding
// semantic json.
type PromptState struct {
	Location  string          `json:"location"`
	Flags     map[string]bool `json:"flags"`
	Inventory []string        `json:"inventory"`
	NPCs      map[string]NPC  `json:"npcs"`
}

func ToPromptState(gs *GameState) *PromptState {
	return &PromptState{
		Location:  gs.Location,
		Flags:     gs.Flags,
		Inventory: gs.Inventory,
		NPCs:      gs.NPCs,
	}
}

// ApplyPromptStateToGameState copies relevant fields from a PromptState to a GameState.
func ApplyPromptStateToGameState(ps *PromptState, gs *GameState) {
	if ps == nil || gs == nil {
		return
	}
	gs.Location = ps.Location
	gs.Flags = ps.Flags
	gs.Inventory = ps.Inventory
	gs.NPCs = ps.NPCs
}
