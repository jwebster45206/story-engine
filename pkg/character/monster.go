package character

import (
	"maps"
	"slices"
)

// Monster represents a creature or enemy in the game world.
// Monsters are spawned from external JSON templates and managed by GameState.
type Monster struct {
	ID          string `json:"id,omitempty"`
	TemplateID  string `json:"template_id,omitempty"` // Reference to template in data/monsters/ (used in scenarios)
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`

	// Mechanical stats. Optional for a template that is only a name and description.
	Stats
	Items []string `json:"items,omitempty"` // Items dropped on defeat

	DropItemsOnDefeat bool `json:"drop_items_on_defeat,omitempty"`
}

// NewMonster creates a new Monster instance from a template with optional overrides.
// The template parameter is the base template monster loaded from JSON (required).
// The overrides parameter contains the monster definition from the scenario, which must include:
//   - ID: unique instance identifier
//   - Location: where the monster spawns
//
// And may include any other fields to override template values.
//
// The function builds the monster by:
// 1. Starting with the template as the base
// 2. Applying any non-zero/non-empty fields from overrides
// 3. Using ID and Location from overrides
func NewMonster(template *Monster, overrides *Monster) *Monster {
	if template == nil || overrides == nil {
		return nil
	}

	// Clone so map overlays do not write back into the template.
	m := *template.Clone()
	m.ID = overrides.ID
	m.Location = overrides.Location

	if overrides.Name != "" {
		m.Name = overrides.Name
	}
	if overrides.Description != "" {
		m.Description = overrides.Description
	}
	if overrides.AC != 0 {
		m.AC = overrides.AC
	}
	if overrides.HP != 0 {
		m.HP = overrides.HP
	}
	if overrides.MaxHP != 0 {
		m.MaxHP = overrides.MaxHP
	}
	m.Abilities = m.Abilities.merge(overrides.Abilities)
	if len(overrides.Attributes) > 0 {
		if m.Attributes == nil {
			m.Attributes = make(map[string]int)
		}
		maps.Copy(m.Attributes, overrides.Attributes)
	}
	if len(overrides.Modifiers) > 0 {
		if m.Modifiers == nil {
			m.Modifiers = make(map[string]int)
		}
		maps.Copy(m.Modifiers, overrides.Modifiers)
	}
	if len(overrides.Actions) > 0 {
		if m.Actions == nil {
			m.Actions = make(map[string]Action)
		}
		maps.Copy(m.Actions, overrides.Actions)
	}
	if len(overrides.Items) > 0 {
		m.Items = overrides.Items
	}
	if overrides.DropItemsOnDefeat {
		m.DropItemsOnDefeat = true
	}
	if m.MaxHP > 0 && m.HP == 0 {
		m.HP = m.MaxHP
	}
	return &m
}

func (m *Monster) Clone() *Monster {
	if m == nil {
		return nil
	}
	c := *m
	c.Stats = m.Stats.Clone()
	c.Items = slices.Clone(m.Items)
	return &c
}

// UnmarshalJSON accepts the deprecated "combat_modifiers" key.
// Embedding promotes Stats fields, so Stats.UnmarshalJSON does not run here.
func (m *Monster) UnmarshalJSON(data []byte) error {
	type plain Monster
	var decoded plain
	if err := unmarshalAliased(data, &decoded); err != nil {
		return err
	}
	decoded.liftAbilityAttributes()
	*m = Monster(decoded)
	return nil
}

// MoveTo updates the monster's location.
func (m *Monster) MoveTo(loc string) {
	m.Location = loc
}
