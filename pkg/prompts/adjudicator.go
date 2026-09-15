package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// AdjudicatorHistoryLimit is the default chat-history window for the pre-chat
// rules pass. Smaller than the narrator window on purpose.
const AdjudicatorHistoryLimit = 2

const AdjudicatorHonorLine = "Honor this mechanical resolution. Narrate the outcome; do not re-adjudicate."

// AdjudicatorPrompt is the referee preamble. Mode-specific rules, WORLD STATE,
// and (in strict mode) examples are appended by BuildAdjudicatorMessages.
const AdjudicatorPrompt = `You are the mechanical referee for a roleplaying text adventure. You are not the narrator. You evaluate the player's input and apply game rules. You determine whether the action is allowed or not, and why. 

Reply in up to two sentences. 
Sentence 1: "Allowed." or "Not allowed." then reasoning.
Sentence 2: If not allowed only: What would happen in-world if the exact action were taken by the PC? This is a terse single-sentence for narrator hint. It is not narration.`

const adjudicatorRulesStrict = `1. Movement
- The player may only travel listed exits in current_location.
- Blocked exits are not usable.
- Invented destinations are not allowed.

2. Items
- Interact only with items in user_inventory or "Items here".
- Picking up items that are not listed as interactable is not allowed.

3. NPCs
- Only NPCs listed as "NPCs here" may be spoken to or acted on. NPCs elsewhere are out of reach this turn.

4. Global
- The player roleplays as the Player Character (PC) only; never as any NPC.
- Stay within the WORLD STATE sandbox: only listed locations, items, NPCs, and monsters exist. Invented creatures, places, items, or powers are not allowed.
- Only listed monsters may be engaged.
- Ordinary PC actions that stay in that sandbox are allowed.`

const adjudicatorRulesRelaxed = `1. Movement
- Known exits are the obvious paths; other directions may be allowed.
- Blocked exits are soft obstacles; a plausible attempt to pass them may be allowed.

2. Items
- Prefer listed items in user_inventory and "Items here".
- Improvised interactions with unlisted objects may be allowed.

3. NPCs
- NPCs listed as "NPCs here" are present this turn.
- Player-introduced people may appear and be acted on.
- Do not speak or act for the Player Character.

4. Global
- Honor people, creatures, places, and objects the player introduces.
- If the player attempts something outside the PC's defined abilities, the attempt is allowed; play it out rather than refuse.`

const adjudicatorExamplesStrict = `Examples:
Example:	"Allowed. The PC can move to the drawbridge."
Example:	"Not allowed. The PC cannot move to the banquet hall because it is blocked. The PC would be stopped by the guard. "

Example:	"Allowed. The PC attacks the giant rat."
Example:	"Not allowed. There is no giant rat to attack."
Example:	"Not allowed. The PC cannot dictate that the guard is defeated in the attack. The PC's attack occurs, but outcome is decided by the narrator."`

func adjudicatorRules(mode state.RulesMode) string {
	if mode == state.RulesRelaxed {
		return adjudicatorRulesRelaxed
	}
	return adjudicatorRulesStrict
}

func adjudicatorExamples(mode state.RulesMode) string {
	if mode == state.RulesRelaxed {
		return ""
	}
	return adjudicatorExamplesStrict
}

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

// BuildAdjudicatorMessages assembles the pre-chat referee call: mode-specific
// adjudicator rules, location-scoped WORLD STATE, strict-mode examples at the
// end, a short history window, and the current user line (no narrator <rules>
// block).
func BuildAdjudicatorMessages(gs *state.GameState, userMessage string, historyLimit int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}

	var sb strings.Builder
	sb.WriteString(AdjudicatorPrompt)
	sb.WriteString("\n\n")
	sb.WriteString(adjudicatorRules(gs.Rules))
	sb.WriteString("\n\n")
	sb.WriteString(ToPromptState(gs).ToSlimString())
	if examples := adjudicatorExamples(gs.Rules); examples != "" {
		sb.WriteString("\n")
		sb.WriteString(examples)
	}

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
