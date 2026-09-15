package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/state"
)

// ruleSet is the mode-dependent prompt copy shared by narrator and referee.
// Empty string fields are omitted when a prompt is stitched.
type ruleSet struct {
	EnforceExits    bool
	Interpretation  string
	Locations       string
	GameMechanics   string
	Monsters        string
	WorldStateRules []string
	Movement        string
	Items           string
	NPCs            string
	Global          string
	Examples        string
}

func getRuleSet(mode state.RulesMode) ruleSet {
	if mode == state.RulesRelaxed {
		return relaxedRuleSet
	}
	return strictRuleSet
}

func (rs ruleSet) formatRefereeRules() string {
	var sb strings.Builder
	for _, sec := range []struct{ heading, body string }{
		{"Movement", rs.Movement},
		{"Items", rs.Items},
		{"NPCs", rs.NPCs},
		{"Global", rs.Global},
	} {
		body := strings.TrimSpace(sec.body)
		if body == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "%s\n%s", sec.heading, body)
	}
	return sb.String()
}
