package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// RefereeHistoryLimit is the default chat-history window for the pre-chat
// rules pass. Smaller than the narrator window on purpose.
const RefereeHistoryLimit = 2

const RefereeHonorLine = "Honor this mechanical resolution. Narrate the outcome; do not re-decide."

// RefereePrompt is the referee preamble. Mode-specific RuleSet sections, WORLD
// STATE, and (when present) examples are appended by BuildRefereeMessages.
const RefereePrompt = `You are the mechanical referee for a roleplaying text adventure. You are not the narrator. You evaluate the player's input and apply game rules. You determine whether the action is allowed or not, and why. 

Reply in up to two sentences. 
Sentence 1: "Allowed." or "Not allowed." then reasoning.
Sentence 2: If not allowed only: What would happen in-world if the exact action were taken by the PC? This is a terse single-sentence for narrator hint. It is not narration.`

// RefereeWindow returns the history window for the referee: min of the
// narrator limit and RefereeHistoryLimit.
func RefereeWindow(narratorLimit int) int {
	if narratorLimit <= 0 || narratorLimit > RefereeHistoryLimit {
		return RefereeHistoryLimit
	}
	return narratorLimit
}

// FormatRefereeBlock wraps a ruling for the narrator user turn.
func FormatRefereeBlock(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return "<referee>\n" + text + "\n</referee>\n" + RefereeHonorLine
}

// BuildRefereeMessages assembles the pre-chat referee call: RuleSet referee
// sections, location-scoped WORLD STATE, examples at the end when present, a
// short history window, and the current user line (no narrator <rules> block).
func BuildRefereeMessages(gs *state.GameState, userMessage string, historyLimit int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}

	rs := GetRuleSet(gs.Rules)
	var sb strings.Builder
	sb.WriteString(RefereePrompt)
	if rules := rs.formatRefereeRules(); rules != "" {
		sb.WriteString("\n\n")
		sb.WriteString(rules)
	}
	sb.WriteString("\n\n")
	sb.WriteString(ToPromptState(gs).ToSlimString())
	if examples := strings.TrimSpace(rs.Examples); examples != "" {
		sb.WriteString("\nExamples:\n")
		sb.WriteString(examples)
	}

	msgs := []chat.ChatMessage{{
		Role:    chat.ChatRoleSystem,
		Content: sb.String(),
	}}

	if historyLimit <= 0 {
		historyLimit = RefereeHistoryLimit
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
