package character

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/d20/vocab"
)

// Stats is the serializable mechanical block shared by PC, NPC, and Monster.
// It projects to a d20.Actor at roll time.
type Stats struct {
	HP         int               `json:"hp,omitempty"`
	MaxHP      int               `json:"max_hp,omitempty"`
	AC         int               `json:"ac,omitempty"`
	Abilities  Abilities         `json:"abilities,omitzero"`   // the six scores
	Attributes map[string]int    `json:"attributes,omitempty"` // skill bonuses, other named numbers
	Modifiers  map[string]int    `json:"modifiers,omitempty"`  // roll bonuses
	Actions    map[string]Action `json:"actions,omitempty"`
}

// Abilities holds the six ability scores. Value, not pointer: both forms
// produce identical JSON, but a pointer would reintroduce the template-
// aliasing hazard Stage 1 removed, and would need nil guards at every read.
// omitzero is load-bearing — omitempty does nothing for structs.
type Abilities struct {
	Strength     int `json:"strength"`
	Dexterity    int `json:"dexterity"`
	Constitution int `json:"constitution"`
	Intelligence int `json:"intelligence"`
	Wisdom       int `json:"wisdom"`
	Charisma     int `json:"charisma"`
}

// merge overlays non-zero scores from over onto a.
func (a Abilities) merge(over Abilities) Abilities {
	if over.Strength != 0 {
		a.Strength = over.Strength
	}
	if over.Dexterity != 0 {
		a.Dexterity = over.Dexterity
	}
	if over.Constitution != 0 {
		a.Constitution = over.Constitution
	}
	if over.Intelligence != 0 {
		a.Intelligence = over.Intelligence
	}
	if over.Wisdom != 0 {
		a.Wisdom = over.Wisdom
	}
	if over.Charisma != 0 {
		a.Charisma = over.Charisma
	}
	return a
}

// ToAttributes converts the six scores to a map keyed by ability name.
// A zero Abilities returns nil. Scores left at 0 are included when any
// score is set; the struct cannot tell "unset" from "zero".
func (a Abilities) ToAttributes() map[string]int {
	if a == (Abilities{}) {
		return nil
	}
	return map[string]int{
		vocab.Strength:     a.Strength,
		vocab.Dexterity:    a.Dexterity,
		vocab.Constitution: a.Constitution,
		vocab.Intelligence: a.Intelligence,
		vocab.Wisdom:       a.Wisdom,
		vocab.Charisma:     a.Charisma,
	}
}

// Action is the authoring form of d20.Action: dice notation instead of d20.Dice.
type Action struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`    // caller-owned label: "attack", "heal"
	Attempt string `json:"attempt,omitempty"` // "1d20+4"; empty means automatic success
	Effect  string `json:"effect,omitempty"`  // "1d8+3"; empty means no HP change
	Charges *uint  `json:"charges,omitempty"`
}

func (a Action) clone() Action {
	if a.Charges == nil {
		return a
	}
	n := *a.Charges
	a.Charges = &n
	return a
}

func (a Action) toD20() (d20.Action, error) {
	var attempt, effect d20.Dice
	var err error
	if a.Attempt != "" {
		attempt, err = d20.DiceFromExpr(a.Attempt)
		if err != nil {
			return d20.Action{}, fmt.Errorf("attempt %q: %w", a.Attempt, err)
		}
	}
	if a.Effect != "" {
		effect, err = d20.DiceFromExpr(a.Effect)
		if err != nil {
			return d20.Action{}, fmt.Errorf("effect %q: %w", a.Effect, err)
		}
	}
	out := d20.Action{
		Name:    a.Name,
		Type:    a.Type,
		Attempt: attempt,
		Effect:  effect,
	}
	if a.Charges != nil {
		n := *a.Charges
		out.Charges = &n
	}
	return out, nil
}

// IsActor reports whether this block carries any mechanical stats.
// A narrative NPC with an empty block is not an actor.
func (s Stats) IsActor() bool {
	return s.HP > 0 || s.MaxHP > 0 || s.AC > 0 ||
		s.Abilities != (Abilities{}) ||
		len(s.Attributes) > 0 || len(s.Modifiers) > 0 || len(s.Actions) > 0
}

// Clone returns a deep copy of the maps and charge pointers.
func (s Stats) Clone() Stats {
	c := s
	c.Attributes = maps.Clone(s.Attributes)
	c.Modifiers = maps.Clone(s.Modifiers)
	if s.Actions != nil {
		c.Actions = make(map[string]Action, len(s.Actions))
		for id, action := range s.Actions {
			c.Actions[id] = action.clone()
		}
	}
	return c
}

// ApplyEffect applies n to HP. Positive n deals damage; negative n heals.
// HP is clamped to [0, MaxHP] when MaxHP is set, and to 0 otherwise.
// The returned hp is the value after clamping. defeated is HP <= 0.
func (s *Stats) ApplyEffect(n int) (hp int, defeated bool) {
	if s == nil {
		return 0, false
	}
	s.HP -= n
	if s.HP < 0 {
		s.HP = 0
	}
	if s.MaxHP > 0 && s.HP > s.MaxHP {
		s.HP = s.MaxHP
	}
	return s.HP, s.HP <= 0
}

// IsDefeated reports whether HP has reached 0.
func (s Stats) IsDefeated() bool {
	return s.HP <= 0
}

// ValidateActions checks that every non-empty attempt and effect is dice
// notation d20 can parse. Empty strings are allowed.
func (s Stats) ValidateActions() error {
	for id, action := range s.Actions {
		if action.Attempt != "" {
			if _, err := d20.DiceFromExpr(action.Attempt); err != nil {
				return fmt.Errorf("action %q: %w", id, err)
			}
		}
		if action.Effect != "" {
			if _, err := d20.DiceFromExpr(action.Effect); err != nil {
				return fmt.Errorf("action %q: %w", id, err)
			}
		}
	}
	return nil
}

// ToActor parses dice notation and normalizes, so authored content stays
// human-readable. d20.Actor.Attributes is filled from Abilities merged with
// Attributes. No d20 mechanic reads that field; rolls are driven by Modifiers.
// On a key collision, Attributes wins.
func (s Stats) ToActor(id, name string) (*d20.Actor, error) {
	actor := d20.NewActor(id)
	actor.Name = name
	actor.HP = s.HP
	actor.MaxHP = s.MaxHP
	actor.AC = s.AC

	attrs := s.Abilities.ToAttributes()
	if attrs == nil {
		attrs = map[string]int{}
	}
	for k, v := range s.Attributes {
		attrs[k] = v
	}
	actor.Attributes = attrs

	if s.Modifiers != nil {
		actor.Modifiers = maps.Clone(s.Modifiers)
	}

	for key, action := range s.Actions {
		converted, err := action.toD20()
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", key, err)
		}
		actor.Actions[key] = converted
	}

	if err := actor.Normalize(); err != nil {
		return nil, err
	}
	return actor, nil
}

// liftAbilityAttributes moves a pure ability-score map out of Attributes.
// Older NPC and monster files stored the six scores under "attributes".
// A map that contains any other key is left alone, and an Abilities value
// that is already set wins.
func (s *Stats) liftAbilityAttributes() {
	if s == nil || s.Abilities != (Abilities{}) || len(s.Attributes) == 0 {
		return
	}
	for k := range s.Attributes {
		switch k {
		case vocab.Strength, vocab.Dexterity, vocab.Constitution, vocab.Intelligence, vocab.Wisdom, vocab.Charisma:
		default:
			return
		}
	}
	for k, v := range s.Attributes {
		switch k {
		case vocab.Strength:
			s.Abilities.Strength = v
		case vocab.Dexterity:
			s.Abilities.Dexterity = v
		case vocab.Constitution:
			s.Abilities.Constitution = v
		case vocab.Intelligence:
			s.Abilities.Intelligence = v
		case vocab.Wisdom:
			s.Abilities.Wisdom = v
		case vocab.Charisma:
			s.Abilities.Charisma = v
		}
	}
	s.Attributes = nil
}

// unmarshalAliased rewrites legacy keys, then decodes into dest.
//
// Stats must not implement UnmarshalJSON. Anonymous embedding promotes that
// method onto PC, NPC, and Monster, and encoding/json would then call it
// with the whole object and leave every non-stats field empty.
func unmarshalAliased(data []byte, dest any) error {
	data, err := aliasLegacyKeys(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// aliasLegacyKeys rewrites top-level keys from the pre-Stats authoring
// format. "modifiers" and "abilities" win when both the old and new keys
// are present. The input is returned unchanged when neither legacy key is set.
func aliasLegacyKeys(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != '{' {
		return data, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	changed := false
	if _, ok := raw["modifiers"]; !ok {
		if legacy, ok := raw["combat_modifiers"]; ok {
			raw["modifiers"] = legacy
			delete(raw, "combat_modifiers")
			changed = true
		}
	}
	if _, ok := raw["abilities"]; !ok {
		if legacy, ok := raw["stats"]; ok {
			raw["abilities"] = legacy
			delete(raw, "stats")
			changed = true
		}
	}
	if !changed {
		return data, nil
	}
	return json.Marshal(raw)
}
