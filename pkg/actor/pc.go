package actor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jwebster45206/d20"
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
	ID              string         `json:"id"`
	Name            string         `json:"name,omitempty"`
	Class           string         `json:"class,omitempty"`
	Level           int            `json:"level,omitempty"`
	Race            string         `json:"race,omitempty"`
	Pronouns        string         `json:"pronouns,omitempty"`
	Description     string         `json:"description,omitempty"`
	Background      string         `json:"background,omitempty"`
	Stats           Stats5e        `json:"stats,omitempty"`
	HP              int            `json:"hp,omitempty"`     // Current HP (for serialization)
	MaxHP           int            `json:"max_hp,omitempty"` // Maximum HP
	AC              int            `json:"ac,omitempty"`
	CombatModifiers map[string]int `json:"combat_modifiers,omitempty"`
	Attributes      map[string]int `json:"attributes,omitempty"` // Skills, proficiencies, etc.
	Inventory       []string       `json:"inventory,omitempty"`
}

// PC is the runtime representation of a Player Character
type PC struct {
	Spec  *PCSpec
	Actor *d20.Actor // Built at runtime from PCSpec
}

// LoadPC loads a PC from a JSON file and builds its d20.Actor
// The filename (without .json extension) overrides any ID in the JSON
func LoadPC(path string) (*PC, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PC file: %w", err)
	}

	var spec PCSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PC spec: %w", err)
	}

	// Filename overrides any ID in the JSON
	spec.ID = strings.TrimSuffix(filepath.Base(path), ".json")

	pc := &PC{
		Spec: &spec,
	}

	// Build d20.Actor from PCSpec
	// Start with core stats as attributes
	allAttrs := spec.Stats.ToAttributes()

	// Add additional attributes (skills, proficiencies, etc.)
	for k, v := range spec.Attributes {
		allAttrs[k] = v
	}

	// Build the actor
	actor, err := d20.NewActor(spec.Name, spec.MaxHP, spec.AC).
		WithAttributes(allAttrs).
		WithCombatModifiers(spec.CombatModifiers).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build actor: %w", err)
	}

	// Set current HP if different from max
	if spec.HP != spec.MaxHP && spec.HP > 0 {
		if err := actor.SetHP(spec.HP); err != nil {
			return nil, fmt.Errorf("failed to set HP: %w", err)
		}
	}

	pc.Actor = actor
	return pc, nil
}

// MarshalJSON converts PC back to PCSpec format for API responses
// Reads current runtime state from the Actor
func (pc *PC) MarshalJSON() ([]byte, error) {
	// Helper to safely get attribute from Actor
	getAttr := func(key string) int {
		if val, ok := pc.Actor.Attribute(key); ok {
			return val
		}
		return 0
	}

	// Create a response struct for serialization
	type PCResponse struct {
		ID              string         `json:"id"`
		Name            string         `json:"name"`
		Class           string         `json:"class"`
		Level           int            `json:"level"`
		Race            string         `json:"race"`
		Pronouns        string         `json:"pronouns,omitempty"`
		Description     string         `json:"description,omitempty"`
		Background      string         `json:"background,omitempty"`
		Stats           Stats5e        `json:"stats"`
		HP              int            `json:"hp"`
		MaxHP           int            `json:"max_hp"`
		AC              int            `json:"ac"`
		CombatModifiers map[string]int `json:"combat_modifiers,omitempty"`
		Attributes      map[string]int `json:"attributes,omitempty"`
		Inventory       []string       `json:"inventory,omitempty"`
	}

	// Start with static fields from spec
	resp := PCResponse{
		ID:          pc.Spec.ID,
		Name:        pc.Spec.Name,
		Class:       pc.Spec.Class,
		Level:       pc.Spec.Level,
		Race:        pc.Spec.Race,
		Pronouns:    pc.Spec.Pronouns,
		Description: pc.Spec.Description,
		Background:  pc.Spec.Background,
		Inventory:   pc.Spec.Inventory,
	}

	// Get current HP state from Actor
	resp.HP = pc.Actor.HP()
	resp.MaxHP = pc.Actor.MaxHP()
	resp.AC = pc.Actor.AC()

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
	resp.CombatModifiers = make(map[string]int)
	for _, mod := range pc.Actor.GetCombatModifiers() {
		resp.CombatModifiers[mod.Reason] = mod.Value
	}

	// Get non-core attributes from Actor
	resp.Attributes = make(map[string]int)
	coreStats := map[string]bool{
		"strength": true, "dexterity": true, "constitution": true,
		"intelligence": true, "wisdom": true, "charisma": true,
	}
	for key := range pc.Spec.Attributes {
		if !coreStats[key] {
			if val, ok := pc.Actor.Attribute(key); ok {
				resp.Attributes[key] = val
			}
		}
	}

	return json.Marshal(resp)
}
