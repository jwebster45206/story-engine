package chat

import "strings"

// Ruling is the referee's structured allow/deny decision for a player action.
type Ruling struct {
	Allowed   bool   `json:"allowed"`
	Reasoning string `json:"reasoning,omitempty"`
	Reaction  string `json:"reaction,omitempty"`
}

// Normalize trims optional strings and drops reaction when the action is allowed.
func (r *Ruling) Normalize() {
	if r == nil {
		return
	}
	r.Reasoning = strings.TrimSpace(r.Reasoning)
	r.Reaction = strings.TrimSpace(r.Reaction)
	if r.Allowed {
		r.Reaction = ""
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
