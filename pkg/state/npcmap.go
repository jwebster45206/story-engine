package state

import (
	"encoding/json"
	"fmt"
)

type NPCMap map[string]NPC

// UnmarshalJSON allows NPCMap to accept either a map or an array of strings.
func (m *NPCMap) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a map first
	var asMap map[string]NPC
	if err := json.Unmarshal(data, &asMap); err == nil {
		*m = asMap
		return nil
	}
	// Try to unmarshal as an array of strings
	var asArray []string
	if err := json.Unmarshal(data, &asArray); err == nil {
		result := make(map[string]NPC)
		for _, name := range asArray {
			result[name] = NPC{Name: name}
		}
		*m = result
		return nil
	}
	return fmt.Errorf("npcs: not a map or array: %s", string(data))
}
