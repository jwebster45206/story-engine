package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestGetRuleSet_DefaultsToStrict(t *testing.T) {
	for _, mode := range []state.RulesMode{"", state.RulesStrict, "unknown"} {
		rs := getRuleSet(mode)
		if !rs.EnforceExits {
			t.Errorf("getRuleSet(%q).EnforceExits = false, want true", mode)
		}
		if !strings.Contains(rs.Interpretation, "Do not allow the user to control NPCs") {
			t.Errorf("getRuleSet(%q) should use strict interpretation", mode)
		}
	}
}

func TestGetRuleSet_Relaxed(t *testing.T) {
	rs := getRuleSet(state.RulesRelaxed)
	if rs.EnforceExits {
		t.Error("EnforceExits = true, want false for relaxed")
	}
	if rs.Examples != "" {
		t.Errorf("relaxed Examples = %q, want empty", rs.Examples)
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
