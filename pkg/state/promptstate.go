package state

import "github.com/jwebster45206/roleplay-agent/pkg/scenario"

// PromptState is a simplified representation of the game state
// packaged for sharing with a language model.
// Assumes that the model is capable of understanding
// semantic json.
type PromptState struct {
	Location  string                  `json:"location"`
	NPCs      map[string]scenario.NPC `json:"npcs"`
	Flags     map[string]bool         `json:"flags"`
	Inventory []string                `json:"inventory"`
}

func ToPromptState(gs *GameState) *PromptState {
	return &PromptState{
		Location:  gs.Location,
		NPCs:      gs.NPCs,
		Flags:     gs.Flags,
		Inventory: gs.Inventory,
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
