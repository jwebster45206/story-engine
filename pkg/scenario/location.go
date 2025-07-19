package scenario

// Location represents a place in the game world with exits and entry logic.
type Location struct {
	Name         string            `json:"name"`                    // Also the key in the map.
	Description  string            `json:"description,omitempty"`   // Scene description
	Exits        map[string]string `json:"exits,omitempty"`         // Direction → Location Key
	BlockedExits map[string]string `json:"blocked_exits,omitempty"` // Direction → Reason for blocking
	Items        []string          `json:"items,omitempty"`         // Items that can be found in this location
}
