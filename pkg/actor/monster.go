package actor

import "maps"

// Monster represents a creature or enemy in the game world.
// Monsters are spawned from external JSON templates and managed by GameState.
type Monster struct {
	ID          string `json:"id"`
	TemplateID  string `json:"template_id,omitempty"` // Reference to template in data/monsters/ (used in scenarios)
	Name        string `json:"name"`
	Description string `json:"description"`
	Location    string `json:"location"`

	AC    int `json:"ac"`
	HP    int `json:"hp"`
	MaxHP int `json:"max_hp"`

	Attributes map[string]int `json:"attributes,omitempty"`       // Flexible key-value attributes (e.g., "strength": 16)
	CombatMods map[string]int `json:"combat_modifiers,omitempty"` // Combat modifiers (e.g., "bite": 5)
	Items      []string       `json:"items,omitempty"`            // Items dropped on defeat

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

	// Set the instance ID and location from overrides (required fields)
	m := *template
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
	if len(overrides.Attributes) > 0 {
		if m.Attributes == nil {
			m.Attributes = make(map[string]int)
		}
		maps.Copy(m.Attributes, overrides.Attributes)
	}
	if len(overrides.CombatMods) > 0 {
		if m.CombatMods == nil {
			m.CombatMods = make(map[string]int)
		}
		maps.Copy(m.CombatMods, overrides.CombatMods)
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

// TakeDamage reduces the monster's HP by the specified amount.
// HP cannot go below 0.
func (m *Monster) TakeDamage(n int) {
	if n <= 0 {
		return
	}
	m.HP -= n
	if m.HP < 0 {
		m.HP = 0
	}
}

// Heal increases the monster's HP by the specified amount.
// HP cannot exceed MaxHP.
func (m *Monster) Heal(n int) {
	if n <= 0 {
		return
	}
	m.HP += n
	if m.HP > m.MaxHP {
		m.HP = m.MaxHP
	}
}

// IsDefeated returns true if the monster's HP is 0 or less.
func (m *Monster) IsDefeated() bool {
	return m.HP <= 0
}

// MoveTo updates the monster's location.
func (m *Monster) MoveTo(loc string) {
	m.Location = loc
}
