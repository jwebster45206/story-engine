package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/state"
)

// RuleSet is the mode-dependent prompt copy shared by narrator and referee.
// Empty string fields are omitted when a prompt is stitched.
type RuleSet struct {
	Mode            state.RulesMode
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

// GetRuleSet returns the RuleSet for the given mode. Unknown or empty modes
// resolve to strict (the engine default).
func GetRuleSet(mode state.RulesMode) RuleSet {
	if mode == state.RulesRelaxed {
		return RelaxedRuleSet
	}
	return StrictRuleSet
}

// formatRefereeRules numbers Movement, Items, NPCs, and Global, skipping
// empty bodies so headings are never emitted without content.
func (rs RuleSet) formatRefereeRules() string {
	var sb strings.Builder
	n := 1
	n = appendNumberedSection(&sb, n, "Movement", rs.Movement)
	n = appendNumberedSection(&sb, n, "Items", rs.Items)
	n = appendNumberedSection(&sb, n, "NPCs", rs.NPCs)
	appendNumberedSection(&sb, n, "Global", rs.Global)
	return sb.String()
}

func appendNumberedSection(sb *strings.Builder, n int, heading, body string) int {
	body = strings.TrimSpace(body)
	if body == "" {
		return n
	}
	if sb.Len() > 0 {
		sb.WriteString("\n\n")
	}
	fmt.Fprintf(sb, "%d. %s\n%s", n, heading, body)
	return n + 1
}
