package actor

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// Stats5e represents the six core D&D 5e ability scores
type Stats5e struct {
	Strength     int `json:"strength"`
	Dexterity    int `json:"dexterity"`
	Constitution int `json:"constitution"`
	Intelligence int `json:"intelligence"`
	Wisdom       int `json:"wisdom"`
	Charisma     int `json:"charisma"`
}

// ToAttributes converts Stats5e to a map for d20.Actor compatibility
func (s *Stats5e) ToAttributes() map[string]int {
	return map[string]int{
		"strength":     s.Strength,
		"dexterity":    s.Dexterity,
		"constitution": s.Constitution,
		"intelligence": s.Intelligence,
		"wisdom":       s.Wisdom,
		"charisma":     s.Charisma,
	}
}

// PCSpec is the serializable specification for a Player Character
type PCSpec struct {
	ID                 string                           `json:"id"`
	Name               string                           `json:"name,omitempty"`
	Class              string                           `json:"class,omitempty"`
	Level              int                              `json:"level,omitempty"`
	Race               string                           `json:"race,omitempty"`
	Pronouns           string                           `json:"pronouns,omitempty"`
	Description        string                           `json:"description,omitempty"`
	Background         string                           `json:"background,omitempty"`
	OpeningPrompt      string                           `json:"opening_prompt,omitempty"`      // PC-specific opening text
	ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts,omitempty"` // Conditional prompts for this PC
	Stats              Stats5e                          `json:"stats,omitzero"`
	HP                 int                              `json:"hp,omitempty"`     // Current HP (for serialization)
	MaxHP              int                              `json:"max_hp,omitempty"` // Maximum HP
	AC                 int                              `json:"ac,omitempty"`
	CombatModifiers    map[string]int                   `json:"combat_modifiers,omitempty"`
	Attributes         map[string]int                   `json:"attributes,omitempty"` // Skills, proficiencies, etc.
	Inventory          []string                         `json:"inventory,omitempty"`
}

// PC is the runtime representation of a Player Character
type PC struct {
	Spec  *PCSpec
	Actor *d20.Actor // Built at runtime from PCSpec
}

// NewPCFromSpec creates a PC from a PCSpec
// This is the preferred way to construct PCs after loading from storage
func NewPCFromSpec(spec *PCSpec) (*PC, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec cannot be nil")
	}

	pc := &PC{
		Spec: spec,
	}

	// Build d20.Actor from PCSpec
	// Start with core stats as attributes
	allAttrs := spec.Stats.ToAttributes()

	// Add additional attributes (skills, proficiencies, etc.)
	maps.Copy(allAttrs, spec.Attributes)

	actor := d20.NewActor(spec.ID)
	actor.MaxHP = spec.MaxHP
	actor.HP = spec.MaxHP
	actor.AC = spec.AC
	actor.Attributes = allAttrs
	if spec.CombatModifiers != nil {
		actor.Modifiers = maps.Clone(spec.CombatModifiers)
	}
	if spec.HP != spec.MaxHP && spec.HP > 0 {
		actor.HP = spec.HP
	}

	pc.Actor = actor
	return pc, nil
}

// MarshalJSON converts PC back to PCSpec format for API responses
// Reads current runtime state from the Actor
func (pc *PC) MarshalJSON() ([]byte, error) {
	// Handle nil PC or nil Actor gracefully
	if pc == nil {
		return []byte("null"), nil
	}
	if pc.Actor == nil {
		// If Actor is nil, just serialize the Spec directly
		return json.Marshal(pc.Spec)
	}

	getAttr := func(key string) int {
		return pc.Actor.Attributes[key]
	}

	// Create a response struct for serialization
	type PCResponse struct {
		ID                 string                           `json:"id"`
		Name               string                           `json:"name"`
		Class              string                           `json:"class"`
		Level              int                              `json:"level"`
		Race               string                           `json:"race"`
		Pronouns           string                           `json:"pronouns,omitempty"`
		Description        string                           `json:"description,omitempty"`
		Background         string                           `json:"background,omitempty"`
		OpeningPrompt      string                           `json:"opening_prompt,omitempty"`
		ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts,omitempty"`
		Stats              Stats5e                          `json:"stats"`
		HP                 int                              `json:"hp"`
		MaxHP              int                              `json:"max_hp"`
		AC                 int                              `json:"ac"`
		CombatModifiers    map[string]int                   `json:"combat_modifiers,omitempty"`
		Attributes         map[string]int                   `json:"attributes,omitempty"`
		Inventory          []string                         `json:"inventory,omitempty"`
	}

	// Start with static fields from spec
	resp := PCResponse{
		ID:                 pc.Spec.ID,
		Name:               pc.Spec.Name,
		Class:              pc.Spec.Class,
		Level:              pc.Spec.Level,
		Race:               pc.Spec.Race,
		Pronouns:           pc.Spec.Pronouns,
		Description:        pc.Spec.Description,
		Background:         pc.Spec.Background,
		OpeningPrompt:      pc.Spec.OpeningPrompt,
		ContingencyPrompts: pc.Spec.ContingencyPrompts,
		Inventory:          pc.Spec.Inventory,
	}

	resp.HP = pc.Actor.HP
	resp.MaxHP = pc.Actor.MaxHP
	resp.AC = pc.Actor.AC

	// Rebuild Stats5e from Actor's current attributes
	resp.Stats = Stats5e{
		Strength:     getAttr("strength"),
		Dexterity:    getAttr("dexterity"),
		Constitution: getAttr("constitution"),
		Intelligence: getAttr("intelligence"),
		Wisdom:       getAttr("wisdom"),
		Charisma:     getAttr("charisma"),
	}

	// Get combat modifiers from Actor
	resp.CombatModifiers = maps.Clone(pc.Actor.Modifiers)

	// Get non-core attributes from Actor
	resp.Attributes = make(map[string]int)
	coreStats := map[string]bool{
		"strength": true, "dexterity": true, "constitution": true,
		"intelligence": true, "wisdom": true, "charisma": true,
	}
	for key := range pc.Spec.Attributes {
		if !coreStats[key] {
			if val, ok := pc.Actor.Attributes[key]; ok {
				resp.Attributes[key] = val
			}
		}
	}

	return json.Marshal(resp)
}

// UnmarshalJSON reconstructs a PC from JSON. The Actor is left nil; callers
// that need a live d20.Actor should use NewPCFromSpec after loading a full
// spec. This allows id-only references (e.g. {"id":"pirate_captain"}) to
// decode without requiring HP/AC.
func (pc *PC) UnmarshalJSON(data []byte) error {
	var spec PCSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to unmarshal PC spec: %w", err)
	}
	pc.Spec = &spec
	return nil
}

// BuildPrompt constructs the player character section for the system prompt
// Returns an empty string if pc is nil
//
// Example output:
// The user is controlling: Sir Galahad (he/him), Level 5 Paladin.A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.
func BuildPrompt(pc *PC) string {
	if pc == nil {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("REMEMBER: In this game, the user is controlling: ")
	sb.WriteString(pc.Spec.Name)
	if pc.Spec.Pronouns != "" {
		fmt.Fprintf(&sb, " (%s)", pc.Spec.Pronouns)
	}
	if pc.Spec.Level > 0 || pc.Spec.Class != "" || pc.Spec.Race != "" {
		summaryParts := []string{}
		if pc.Spec.Level > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("Level %d", pc.Spec.Level))
		}
		if pc.Spec.Race != "" {
			summaryParts = append(summaryParts, pc.Spec.Race)
		}
		if pc.Spec.Class != "" {
			summaryParts = append(summaryParts, pc.Spec.Class)
		}
		sb.WriteString(", " + strings.Join(summaryParts, " "))
	}
	if pc.Spec.Description != "" {
		sb.WriteString(". " + pc.Spec.Description)
	}
	return sb.String()
}
