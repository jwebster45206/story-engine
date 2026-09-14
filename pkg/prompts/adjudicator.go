package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// AdjudicatorHistoryLimit is the default chat-history window for the pre-chat
// rules pass. Smaller than the narrator window on purpose.
const AdjudicatorHistoryLimit = 4

const AdjudicatorHonorLine = "Honor this mechanical resolution. Narrate the outcome; do not re-adjudicate."

// AdjudicatorPrompt is the referee preamble. RuleSet bodies and WORLD STATE
// are appended by BuildAdjudicatorMessages.
const AdjudicatorPrompt = `You are the rules adjudicator for a roleplaying text adventure. You do not narrate. Decide what mechanically applies to this player input given the selected ruleset and WORLD STATE.

Output short bullets only, no fiction:
- Attempted action
- Allowed or not, and why (per the ruleset)
- Which listed entities, exits, or items apply
- Constraints the narrator must honor

Do not write story prose.`

// AdjudicatorWindow returns the history window for the adjudicator: min of
// the narrator limit and AdjudicatorHistoryLimit.
func AdjudicatorWindow(narratorLimit int) int {
	if narratorLimit <= 0 || narratorLimit > AdjudicatorHistoryLimit {
		return AdjudicatorHistoryLimit
	}
	return narratorLimit
}

// FormatAdjudicationBlock wraps a ruling for the narrator user turn.
func FormatAdjudicationBlock(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return "<adjudication>\n" + text + "\n</adjudication>\n" + AdjudicatorHonorLine
}

// BuildAdjudicatorMessages assembles the pre-chat referee call: RuleSet from
// gs.Rules, location-scoped WORLD STATE, a short history window, and the
// current user line (no narrator <rules> block).
func BuildAdjudicatorMessages(gs *state.GameState, userMessage string, historyLimit int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}

	rs := GetRuleSet(gs.Rules)
	var sb strings.Builder
	sb.WriteString(AdjudicatorPrompt)
	sb.WriteString("\n\n### HOW YOU INTERPRET USER PROMPTS:\n")
	sb.WriteString(rs.Interpretation)
	sb.WriteString("\n\n### Describing locations\n")
	sb.WriteString(rs.Locations)
	sb.WriteString("\n\n### Game mechanics:\n")
	sb.WriteString(rs.GameMechanics)
	sb.WriteString("\n\n### Monsters\n")
	sb.WriteString(rs.Monsters)
	sb.WriteString("\n\n")
	sb.WriteString(ToPromptState(gs).ToString())

	msgs := []chat.ChatMessage{{
		Role:    chat.ChatRoleSystem,
		Content: sb.String(),
	}}

	if historyLimit <= 0 {
		historyLimit = AdjudicatorHistoryLimit
	}
	if n := len(gs.ChatHistory); n > 0 {
		start := 0
		if n > historyLimit {
			start = n - historyLimit
		}
		msgs = append(msgs, gs.ChatHistory[start:]...)
	}
	if userMessage != "" {
		msgs = append(msgs, chat.ChatMessage{
			Role:    chat.ChatRoleUser,
			Content: userMessage,
		})
	}
	return msgs, nil
}
