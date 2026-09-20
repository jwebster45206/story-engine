package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestGetRuleSet_DefaultsToStrict(t *testing.T) {
	for _, mode := range []state.RulesMode{"", state.RulesStrict, "unknown"} {
		rs := getRuleSet(mode)
		if !strings.Contains(rs.Locations, "Never mention exits that aren't in the WORLD STATE") {
			t.Errorf("getRuleSet(%q) should use strict locations copy", mode)
		}
	}
}

func TestGetRuleSet_Relaxed(t *testing.T) {
	rs := getRuleSet(state.RulesRelaxed)
	if rs.Examples != "" {
		t.Errorf("relaxed Examples = %q, want empty", rs.Examples)
	}
	if !strings.Contains(rs.Locations, "draw from the WORLD STATE as your starting point") {
		t.Error("expected relaxed locations copy")
	}
}

func TestGetRuleSet_StrictExamplesNonEmpty(t *testing.T) {
	rs := getRuleSet(state.RulesStrict)
	if strings.TrimSpace(rs.Examples) == "" {
		t.Error("strict Examples should be non-empty")
	}
}

func TestRuleSet_FormatRefereeRules_SkipsEmpty(t *testing.T) {
	rs := ruleSet{
		Movement: "- go north",
		Global:   "- stay in sandbox",
	}
	got := rs.formatRefereeRules()
	if !strings.Contains(got, "Movement\n- go north") {
		t.Errorf("missing Movement heading:\n%s", got)
	}
	if !strings.Contains(got, "Global\n- stay in sandbox") {
		t.Errorf("missing Global heading:\n%s", got)
	}
	if strings.Contains(got, "Items") || strings.Contains(got, "NPCs") {
		t.Errorf("empty sections should be omitted:\n%s", got)
	}
}

func TestSystemPromptTemplate_HasFiveSlots(t *testing.T) {
	count := strings.Count(systemPromptTemplate, "%s")
	if count != 5 {
		t.Errorf("systemPromptTemplate has %d %%s slots, want 5", count)
	}
}

func TestSystemPromptTemplate_SharedNarratorVoiceHeading(t *testing.T) {
	const heading = "### Writing rules for narrative output:"
	if !strings.Contains(systemPromptTemplate, heading) {
		t.Errorf("systemPromptTemplate missing shared heading %q", heading)
	}
	if strings.Contains(systemPromptTemplate, "HOW YOU INTERPRET USER PROMPTS") {
		t.Error("interpretation section should be removed from narrator template")
	}
	if strings.Contains(systemPromptTemplate, "### Game mechanics:") {
		t.Error("game mechanics section should be removed from narrator template")
	}
	if strings.Contains(systemPromptTemplate, "### Monsters") {
		t.Error("monsters copy should not be in the persistent template")
	}
}
