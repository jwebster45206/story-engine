package scenario

import (
	"maps"
	"slices"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// Location represents a place in the game world with exits and entry logic.
type Location struct {
	Name               string                           `json:"name"`                          // Also the key in the map.
	Description        string                           `json:"description,omitempty"`         // Scene description
	Preview            string                           `json:"preview,omitempty"`             // Short summary shown for adjacent locations (prevents description bleed)
	Exits              map[string]string                `json:"exits,omitempty"`               // Direction → Location Key
	BlockedExits       map[string]string                `json:"blocked_exits,omitempty"`       // Direction → Reason for blocking
	Items              []string                         `json:"items,omitempty"`               // Items that can be found in this location
	Monsters           map[string]*character.Monster    `json:"monsters,omitempty"`            // Active monster instances at this location (instance ID → Monster)
	IsImportant        bool                             `json:"important,omitempty"`           // whether this location is important to always show
	ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts,omitempty"` // Location-specific prompts shown when at player location
}

// Clone returns a deep copy. Monsters are held by pointer and mutated in play,
// so a location taken from a scenario template must be cloned before game
// state edits it, or those edits land in the template.
func (l Location) Clone() Location {
	c := l
	c.Exits = maps.Clone(l.Exits)
	c.BlockedExits = maps.Clone(l.BlockedExits)
	c.Items = slices.Clone(l.Items)
	c.ContingencyPrompts = slices.Clone(l.ContingencyPrompts)
	if l.Monsters != nil {
		c.Monsters = make(map[string]*character.Monster, len(l.Monsters))
		for id, m := range l.Monsters {
			c.Monsters[id] = m.Clone()
		}
	}
	return c
}
