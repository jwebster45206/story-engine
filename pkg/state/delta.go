package state

// GameStateDelta is a compact representation of the changes made to the game state
// after processing a chat message. A GameStateDelta is much faster
// for the LLM to generate than a full game state.
type GameStateDelta struct {
	UserLocation        string            `json:"user_location,omitempty"`
	SceneName           string            `json:"scene_name,omitempty"`
	AddToInventory      []string          `json:"add_to_inventory,omitempty"`
	RemoveFromInventory []string          `json:"remove_from_inventory,omitempty"`
	SetVars             map[string]string `json:"set_vars,omitempty"`
	GameEnded           *bool             `json:"game_ended,omitempty"`

	MovedItems []struct {
		Item string `json:"item"`
		From string `json:"from"`
		To   string `json:"to,omitempty"`
	} `json:"moved_items,omitempty"`

	UpdatedNPCs []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Location    string `json:"location"`
	} `json:"updated_npcs,omitempty"`
}

// IsEmpty checks if the GameStateDelta is empty
func (gsd *GameStateDelta) IsEmpty() bool {
	return gsd == nil || (gsd.UserLocation == "" &&
		len(gsd.AddToInventory) == 0 &&
		len(gsd.RemoveFromInventory) == 0 &&
		len(gsd.MovedItems) == 0 &&
		len(gsd.UpdatedNPCs) == 0)
}
