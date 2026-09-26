package character

import (
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

// ToActor creates a transient mechanical representation of the character.
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
	maps.Copy(attrs, s.Attributes)
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
