package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const refereeHistoryLimit = 2

const refereeHonorLine = "Honor this mechanical resolution. Narrate the outcome; do not re-decide."

const refereePrompt = `You are the mechanical referee for a roleplaying text adventure. You are not the narrator. You evaluate the player's input and apply game rules. You determine whether the action is allowed or not, and why.

Respond with JSON:
- allowed: whether the action is permitted
- reasoning: brief explanation, or null
- reaction: only when the action is not allowed. A terse in-world hint of what would happen if the PC took the exact action. It is not narration. Null when allowed is true.
- scope: the primary intent of this turn. One of: dialogue, movement, combat, examine, ambient, other.
- focus: NPCs and locations the PC is engaging this turn. Use WORLD STATE names from "NPCs here" and from exits / adjacent_previews. Empty arrays if none.`

func refereeWindow(narratorLimit int) int {
	if narratorLimit <= 0 || narratorLimit > refereeHistoryLimit {
		return refereeHistoryLimit
	}
	return narratorLimit
}

func formatRefereeBlock(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return "<referee>\n" + text + "\n</referee>\n" + refereeHonorLine
}

// BuildRefereeMessages assembles the pre-chat referee call: RuleSet referee
// sections, location-scoped WORLD STATE, examples at the end when present, a
// short history window, and the current user line (no narrator <rules> block).
func BuildRefereeMessages(gs *state.GameState, userMessage string, historyLimit int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}

	rs := getRuleSet(gs.Rules)
	var sb strings.Builder
	sb.WriteString(refereePrompt)
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

	historyLimit = refereeWindow(historyLimit)
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
