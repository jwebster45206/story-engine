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
type Ruling struct {
	Allowed   bool        `json:"allowed"`
	Reasoning string      `json:"reasoning,omitempty"`
	Reaction  string      `json:"reaction,omitempty"`
	Scope     RulingScope `json:"scope,omitempty"`
	Subject   string      `json:"subject,omitempty"` // empty or "PC" means the player
	Object    string      `json:"object,omitempty"`
	ActionID  string      `json:"action_id,omitempty"` // id from the acting actor's menu; empty when no roll
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
		ActionID  *string     `json:"action_id"`
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
	if raw.ActionID != nil {
		r.ActionID = *raw.ActionID
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
	r.ActionID = strings.TrimSpace(r.ActionID)
	if r.Allowed {
		r.Reaction = ""
	}
	r.Scope = normalizeRulingScope(string(r.Scope))
}

// IsPCSubject reports whether the PC is acting (empty, "PC", or the PC's name).
func (r *Ruling) IsPCSubject(pcName string) bool {
	if r == nil {
		return true
	}
	s := strings.TrimSpace(r.Subject)
	if s == "" || strings.EqualFold(s, "PC") {
		return true
	}
	pcName = strings.TrimSpace(pcName)
	return pcName != "" && strings.EqualFold(s, pcName)
}

// ActorName is the display name of who is acting. Empty subject uses pcName, or "PC".
func (r *Ruling) ActorName(pcName string) string {
	if r.IsPCSubject(pcName) {
		if name := strings.TrimSpace(pcName); name != "" {
			return name
		}
		return "PC"
	}
	return strings.TrimSpace(r.Subject)
}

// TargetName is the display name of the action's object. Empty object is empty.
// "PC" maps to pcName, or "PC" if pcName is empty.
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
