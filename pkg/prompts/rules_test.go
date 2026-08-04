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
}

func TestRuleSet_BaseSystemPromptHasThreeSlots(t *testing.T) {
	for _, rs := range []RuleSet{StrictRuleSet, RelaxedRuleSet} {
		count := strings.Count(rs.BaseSystemPrompt, "%s")
		if count != 3 {
			t.Errorf("%s BaseSystemPrompt has %d %%s slots, want 3", rs.Mode, count)
		}
	}
}

func TestRuleSet_SharedNarratorVoiceHeading(t *testing.T) {
	const heading = "### Writing rules for narrative output:"
	for _, rs := range []RuleSet{StrictRuleSet, RelaxedRuleSet} {
		if !strings.Contains(rs.BaseSystemPrompt, heading) {
			t.Errorf("%s BaseSystemPrompt missing shared heading %q", rs.Mode, heading)
		}
	}
}
