package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestGetRuleSet_DefaultsToStrict(t *testing.T) {
	for _, mode := range []state.RulesMode{"", state.RulesStrict, "unknown"} {
		rs := GetRuleSet(mode)
		if rs.Mode != state.RulesStrict {
			t.Errorf("GetRuleSet(%q).Mode = %q, want strict", mode, rs.Mode)
		}
		if !rs.EnforceExits {
			t.Errorf("GetRuleSet(%q).EnforceExits = false, want true", mode)
		}
	}
}

func TestGetRuleSet_Relaxed(t *testing.T) {
	rs := GetRuleSet(state.RulesRelaxed)
	if rs.Mode != state.RulesRelaxed {
		t.Errorf("Mode = %q, want relaxed", rs.Mode)
	}
	if rs.EnforceExits {
		t.Error("EnforceExits = true, want false for relaxed")
	}
	if rs.Examples != "" {
		t.Errorf("relaxed Examples = %q, want empty", rs.Examples)
	}
}

func TestGetRuleSet_StrictExamplesNonEmpty(t *testing.T) {
	rs := GetRuleSet(state.RulesStrict)
	if strings.TrimSpace(rs.Examples) == "" {
		t.Error("strict Examples should be non-empty")
	}
}

func TestRuleSet_FormatRefereeRules_SkipsEmpty(t *testing.T) {
	rs := RuleSet{
		Movement: "- go north",
		Global:   "- stay in sandbox",
	}
	got := rs.formatRefereeRules()
	if !strings.Contains(got, "1. Movement") {
		t.Errorf("missing Movement heading:\n%s", got)
	}
	if !strings.Contains(got, "2. Global") {
		t.Errorf("Global should be numbered 2 when Items/NPCs are empty:\n%s", got)
	}
	if strings.Contains(got, "Items") || strings.Contains(got, "NPCs") {
		t.Errorf("empty sections should be omitted:\n%s", got)
	}
}

func TestSystemPromptTemplate_HasSevenSlots(t *testing.T) {
	count := strings.Count(systemPromptTemplate, "%s")
	if count != 7 {
		t.Errorf("systemPromptTemplate has %d %%s slots, want 7", count)
	}
}

func TestSystemPromptTemplate_SharedNarratorVoiceHeading(t *testing.T) {
	const heading = "### Writing rules for narrative output:"
	if !strings.Contains(systemPromptTemplate, heading) {
		t.Errorf("systemPromptTemplate missing shared heading %q", heading)
	}
}
