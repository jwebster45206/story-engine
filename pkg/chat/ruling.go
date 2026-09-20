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

// Ruling is the referee's structured allow/deny decision for a player action.
// Empty Subject means the PC is acting. Object is the primary target (actor or location).
type Ruling struct {
	Allowed   bool        `json:"allowed"`
	Reasoning string      `json:"reasoning,omitempty"`
	Reaction  string      `json:"reaction,omitempty"`
	Scope     RulingScope `json:"scope,omitempty"`
	Subject   string      `json:"subject,omitempty"`
	Object    string      `json:"object,omitempty"`
}

// UnmarshalJSON accepts JSON null for optional strings.
func (r *Ruling) UnmarshalJSON(data []byte) error {
	var raw struct {
		Allowed   bool        `json:"allowed"`
		Reasoning *string     `json:"reasoning"`
		Reaction  *string     `json:"reaction"`
		Scope     RulingScope `json:"scope"`
		Subject   *string     `json:"subject"`
		Object    *string     `json:"object"`
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
	if raw.Subject != nil {
		r.Subject = *raw.Subject
	}
	if raw.Object != nil {
		r.Object = *raw.Object
	}
	return nil
}

// Normalize trims optional strings, drops reaction when the action is allowed,
// and canonicalizes scope.
func (r *Ruling) Normalize() {
	if r == nil {
		return
	}
	r.Reasoning = strings.TrimSpace(r.Reasoning)
	r.Reaction = strings.TrimSpace(r.Reaction)
	r.Subject = strings.TrimSpace(r.Subject)
	r.Object = strings.TrimSpace(r.Object)
	if r.Allowed {
		r.Reaction = ""
	}
	r.Scope = normalizeRulingScope(string(r.Scope))
}

// IsPCSubject reports whether the PC is the acting character (empty or "PC").
func (r *Ruling) IsPCSubject() bool {
	if r == nil {
		return true
	}
	s := strings.TrimSpace(r.Subject)
	return s == "" || strings.EqualFold(s, "PC")
}

// ActorName is the display name of who is acting. Empty subject uses pcName, or "PC".
func (r *Ruling) ActorName(pcName string) string {
	if r.IsPCSubject() {
		if name := strings.TrimSpace(pcName); name != "" {
			return name
		}
		return "PC"
	}
	return strings.TrimSpace(r.Subject)
}

// TargetName is the display name of the action's object. Empty object uses pcName when
// the subject is not the PC (a reaction targeting the player).
func (r *Ruling) TargetName(pcName string) string {
	if r == nil {
		return ""
	}
	if obj := strings.TrimSpace(r.Object); obj != "" {
		if strings.EqualFold(obj, "PC") {
			if name := strings.TrimSpace(pcName); name != "" {
				return name
			}
			return "PC"
		}
		return obj
	}
	return ""
}

// FocusedNames are non-PC subject and object names, for narrator highlighting.
func (r *Ruling) FocusedNames(pcName string) []string {
	if r == nil {
		return nil
	}
	pcName = strings.TrimSpace(pcName)
	skip := func(name string) bool {
		if name == "" || strings.EqualFold(name, "PC") {
			return true
		}
		return pcName != "" && strings.EqualFold(name, pcName)
	}
	var names []string
	if s := strings.TrimSpace(r.Subject); !skip(s) {
		names = append(names, s)
	}
	if obj := strings.TrimSpace(r.Object); !skip(obj) {
		names = append(names, obj)
	}
	return names
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
