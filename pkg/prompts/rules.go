package prompts

import "github.com/jwebster45206/story-engine/pkg/state"

// RuleSet is the mode-dependent portion of the prompt: the base system
// prompt template and the per-turn world-state rules.
type RuleSet struct {
	Mode             state.RulesMode
	BaseSystemPrompt string   // three %s slots: narrator name, narrator style, PC section
	WorldStateRules  []string // static lines for the <world_state_rules> block
	EnforceExits     bool     // strict enumerates allowed destinations + redirect line
}

// GetRuleSet returns the RuleSet for the given mode. Unknown or empty modes
// resolve to strict (the engine default).
func GetRuleSet(mode state.RulesMode) RuleSet {
	if mode == state.RulesRelaxed {
		return RelaxedRuleSet
	}
	return StrictRuleSet
}
