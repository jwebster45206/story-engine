package actor

import "github.com/jwebster45206/story-engine/pkg/conditionals"

// NPC represents a non-player character in the game.
// NPCs can be defined inline in a scenario or loaded from a standalone JSON
// template in data/npcs/. Set TemplateID to reference a standalone file; the
// template is loaded at game-state creation time and merged with any inline
// overrides (location, disposition changes, etc.).
type NPC struct {
	// Template reference — set this to load from data/npcs/{template_id}.json.
	// Leave empty for fully inline NPCs (original behavior, unchanged).
	TemplateID string `json:"template_id,omitempty"`

	Name        string `json:"name"`
	Type        string `json:"type"`                  // e.g. "villager", "guard", "merchant"
	Disposition string `json:"disposition"`           // e.g. "hostile", "neutral", "friendly"
	Description string `json:"description,omitempty"` // short description or backstory
	IsImportant bool   `json:"important,omitempty"`   // whether this NPC is important to the story
	Location    string `json:"location,omitempty"`    // where the NPC is currently located
	Following   string `json:"following,omitempty"`   // ID of actor being followed ("pc" or NPC ID); empty = not following
	Items       []string `json:"items,omitempty"`     // items the NPC has or can give

	// Actor properties — only populated for standalone NPCs loaded from templates.
	// These are optional even in standalone files; omit them for purely narrative NPCs.
	AC               int            `json:"ac,omitempty"`
	HP               int            `json:"hp,omitempty"`
	MaxHP            int            `json:"max_hp,omitempty"`
	Attributes       map[string]int `json:"attributes,omitempty"`        // e.g. {"strength": 14, "dexterity": 12}
	CombatMods       map[string]int `json:"combat_modifiers,omitempty"`  // e.g. {"sword": 3}
	DropItemsOnDefeat bool          `json:"drop_items_on_defeat,omitempty"`

	ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts,omitempty"` // NPC-specific prompts shown when at player location
}

// NewNPCFromTemplate creates an NPC by merging a template with scenario-level overrides.
// The template is the base loaded from data/npcs/ (required, provides defaults).
// The overrides come from the scenario's inline NPC definition and supply
// instance-specific values: location, disposition changes, item additions, etc.
// Non-zero / non-empty override fields take precedence over template values.
func NewNPCFromTemplate(template *NPC, overrides *NPC) *NPC {
	if template == nil || overrides == nil {
		return nil
	}

	// Start with a copy of the template
	n := *template

	// Always carry the template reference forward
	n.TemplateID = template.TemplateID

	// Scalar string overrides
	if overrides.Name != "" {
		n.Name = overrides.Name
	}
	if overrides.Type != "" {
		n.Type = overrides.Type
	}
	if overrides.Disposition != "" {
		n.Disposition = overrides.Disposition
	}
	if overrides.Description != "" {
		n.Description = overrides.Description
	}
	if overrides.Location != "" {
		n.Location = overrides.Location
	}
	if overrides.Following != "" {
		n.Following = overrides.Following
	}

	// Boolean overrides (only override when explicitly set to true)
	if overrides.IsImportant {
		n.IsImportant = true
	}
	if overrides.DropItemsOnDefeat {
		n.DropItemsOnDefeat = true
	}

	// Numeric actor property overrides
	if overrides.AC != 0 {
		n.AC = overrides.AC
	}
	if overrides.HP != 0 {
		n.HP = overrides.HP
	}
	if overrides.MaxHP != 0 {
		n.MaxHP = overrides.MaxHP
	}

	// Map overrides (merge on top of template)
	if len(overrides.Attributes) > 0 {
		if n.Attributes == nil {
			n.Attributes = make(map[string]int)
		}
		for k, v := range overrides.Attributes {
			n.Attributes[k] = v
		}
	}
	if len(overrides.CombatMods) > 0 {
		if n.CombatMods == nil {
			n.CombatMods = make(map[string]int)
		}
		for k, v := range overrides.CombatMods {
			n.CombatMods[k] = v
		}
	}

	// Items: overrides replace template items if provided
	if len(overrides.Items) > 0 {
		n.Items = overrides.Items
	}

	// ContingencyPrompts: overrides replace template prompts if provided
	if len(overrides.ContingencyPrompts) > 0 {
		n.ContingencyPrompts = overrides.ContingencyPrompts
	}

	// Ensure MaxHP is consistent: if MaxHP set but HP is zero, default HP to MaxHP
	if n.MaxHP > 0 && n.HP == 0 {
		n.HP = n.MaxHP
	}

	return &n
}

// TakeDamage reduces the NPC's HP by the specified amount (floor: 0).
// Only meaningful for standalone NPCs with actor properties (HP > 0).
func (n *NPC) TakeDamage(amount int) {
	if amount <= 0 {
		return
	}
	n.HP -= amount
	if n.HP < 0 {
		n.HP = 0
	}
}

// Heal increases the NPC's HP by the specified amount (ceiling: MaxHP).
// Only meaningful for standalone NPCs with actor properties (MaxHP > 0).
func (n *NPC) Heal(amount int) {
	if amount <= 0 {
		return
	}
	n.HP += amount
	if n.MaxHP > 0 && n.HP > n.MaxHP {
		n.HP = n.MaxHP
	}
}
