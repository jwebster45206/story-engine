package actor

// Monster represents a creature or enemy in the game world.
// Monsters are spawned from external JSON templates and managed by GameState.
type Monster struct {
	ID          string `json:"id"`
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

// NewMonster creates a new Monster instance from a template.
// The id parameter is the unique instance ID (e.g., "giant_rat_1").
// The base parameter is the template monster loaded from JSON.
// The location parameter is where the monster spawns.
func NewMonster(id string, base *Monster, location string) *Monster {
	if base == nil {
		return nil
	}

	// Shallow copy of the template
	m := *base
	m.ID = id
	m.Location = location

	// Initialize HP from MaxHP if not already set
	if m.MaxHP > 0 && m.HP == 0 {
		m.HP = m.MaxHP
	}

	// Ensure HP is non-negative
	if m.HP < 0 {
		m.HP = 0
	}

	// Ensure AC is non-negative
	if m.AC < 0 {
		m.AC = 0
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
