package chat

import (
	"encoding/json"
	"strings"
)

// RulingScope is the primary intent of the player's turn, inferred by the referee.
type RulingScope string

const (
	RulingScopeDialogue RulingScope = "dialogue"
	RulingScopeMovement RulingScope = "movement"
	RulingScopeCombat   RulingScope = "combat"
	RulingScopeExamine  RulingScope = "examine"
	RulingScopeAmbient  RulingScope = "ambient"
	RulingScopeOther    RulingScope = "other"
)

// RulingFocus names WORLD STATE NPCs or monsters ("actors") and locations
// the PC is engaging this turn.
type RulingFocus struct {
	Actors    []string `json:"actors,omitempty"`
	Locations []string `json:"locations,omitempty"`
}

// Ruling is the referee's structured allow/deny decision for a player action.
type Ruling struct {
	Allowed   bool        `json:"allowed"`
	Reasoning string      `json:"reasoning,omitempty"`
	Reaction  string      `json:"reaction,omitempty"`
	Scope     RulingScope `json:"scope,omitempty"`
	Focus     RulingFocus `json:"focus"`
}

// UnmarshalJSON accepts JSON null for optional strings and focus.
func (r *Ruling) UnmarshalJSON(data []byte) error {
	var raw struct {
		Allowed   bool         `json:"allowed"`
		Reasoning *string      `json:"reasoning"`
		Reaction  *string      `json:"reaction"`
		Scope     RulingScope  `json:"scope"`
		Focus     *RulingFocus `json:"focus"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Allowed = raw.Allowed
	r.Scope = raw.Scope
	if raw.Reasoning != nil {
		r.Reasoning = *raw.Reasoning
	}
	if raw.Reaction != nil {
		r.Reaction = *raw.Reaction
	}
	if raw.Focus != nil {
		r.Focus = *raw.Focus
	}
	return nil
}

// Normalize trims optional strings, drops reaction when the action is allowed,
// canonicalizes scope, and cleans focus lists.
func (r *Ruling) Normalize() {
	if r == nil {
		return
	}
	r.Reasoning = strings.TrimSpace(r.Reasoning)
	r.Reaction = strings.TrimSpace(r.Reaction)
	if r.Allowed {
		r.Reaction = ""
	}
	r.Scope = normalizeRulingScope(string(r.Scope))
	r.Focus.Actors = normalizeStringList(r.Focus.Actors)
	r.Focus.Locations = normalizeStringList(r.Focus.Locations)
}

func normalizeRulingScope(s string) RulingScope {
	switch RulingScope(strings.ToLower(strings.TrimSpace(s))) {
	case RulingScopeDialogue:
		return RulingScopeDialogue
	case RulingScopeMovement:
		return RulingScopeMovement
	case RulingScopeCombat:
		return RulingScopeCombat
	case RulingScopeExamine:
		return RulingScopeExamine
	case RulingScopeAmbient:
		return RulingScopeAmbient
	default:
		return RulingScopeOther
	}
}

func normalizeStringList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NarratorText is the prose block injected into the narrator prompt.
func (r *Ruling) NarratorText() string {
	if r == nil {
		return ""
	}
	if r.Allowed {
		if r.Reasoning == "" {
			return "Allowed."
		}
		return "Allowed. " + r.Reasoning
	}
	line := "Not allowed."
	if r.Reasoning != "" {
		line += " " + r.Reasoning
	}
	if r.Reaction != "" {
		return line + "\n" + r.Reaction
	}
	return line
}
