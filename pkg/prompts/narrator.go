package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const gameEndSystemPrompt = `This user's session has ended. Regardless of the user's input, the game will not continue. Respond in a way that will wrap up the game in a narrative manner. End with a fancy "*.*.*.*.*.*. THE END .*.*.*.*.*.*" line, followed by instructions to use Ctrl+N to start a new game or Ctrl+C to exit.`

const contentRatingG = `Write content suitable for young children. Avoid violence, romance and scary elements. Use simple language and positive messages. `
const contentRatingPG = `Write content suitable for children and families. Mild peril or tension is okay, but avoid strong language, explicit violence, or dark themes. `
const contentRatingPG13 = `Write content appropriate for teenagers. You may include mild swearing, romantic tension, action scenes, and complex emotional themes, but avoid explicit adult situations, graphic violence, or drug use. `
const contentRatingR = `Write with full freedom for adult audiences. All content should progress the story. `

// narratorRules are the highest-priority output constraints, injected into every user turn
// as a <rules> block appended after the player's message.
var narratorRules = []string{
	"Stay within the story world. Only NPCs, locations, items, and monsters defined in the WORLD STATE may appear — invent nothing.",
	"Do not act or speak for the Player Character. The player provides the PC's voice.",
	"Resolve exactly one action, exchange, or location reveal — then stop and let the player respond.",
}

func formatRulesBlock(rules []string) string {
	if len(rules) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<rules>")
	for _, r := range rules {
		sb.WriteString("\n- ")
		sb.WriteString(r)
	}
	sb.WriteString("\n</rules>")
	return sb.String()
}

// statePromptTemplate provides scenario + world-state context.
// The %s for world state is already wrapped in <world_state>...</world_state> tags by PromptState.ToString.
const statePromptTemplate = "The user is roleplaying this scenario: %s\n\nThe following describes the immediately surrounding world.\n\n%s\n"

const worldEventContinuationRule = `When the chat contains a world-event message describing something that just happened, do not re-narrate it — continue the story from after it.`

// systemPromptTemplate is the stitch point for the persistent narrator system prompt.
// %s slots in order: narrator name, narrator style, PC, locations, content rating.
const systemPromptTemplate = `You are %s, the omniscient narrator of a roleplaying text adventure. You describe the story to the user as it unfolds. You never discuss things outside of the game. Your perspective is third-person. You provide narration and NPC conversation.

### Writing rules for narrative output:
- Normal narration must never use colons. Colons are reserved only for dialogue lines.
- When a new character speaks, start a new paragraph and use the format:
  CharacterName: "Spoken line here."
- Always end your response on the world's side of the conversation. Close with the world, an NPC, or a situation in a state of waiting — not with the PC speaking, deciding, or acting. The player provides the PC's voice; you provide everything else.

### Narrator responses
- Do not break the fourth wall. Do not acknowledge that you are an AI or a computer program.
- Do not answer questions about the game mechanics or how to play.
- Move the story forward gradually, allowing the user to explore and discover things on their own.
%s
Your narrator style informs your voice, vocabulary, and output structure. It does not grant permission to ignore the game rules above.

### Player Character
%s

### Describing locations
%s

%s
`

// getPersistentNarratorPrompt constructs the persistent system prompt with narrator and PC prompts injected.
// mode selects the base ruleset (strict or relaxed). pc is optional - pass nil if no PC.
func getPersistentNarratorPrompt(narrator *scenario.Narrator, pc *character.PC, mode state.RulesMode, rating string) string {
	narratorPrompts := ""
	narratorName := "the narrator"
	if narrator != nil {
		narratorPrompts = narrator.GetPromptsAsString()
		narratorName = narrator.Name
	}
	pcPrompt := ""
	if pc != nil {
		pcPrompt = character.BuildPrompt(pc)
	}
	rs := getRuleSet(mode)
	return fmt.Sprintf(systemPromptTemplate,
		narratorName,
		narratorPrompts,
		pcPrompt,
		rs.Locations,
		formatContentRating(rating),
	)
}

func formatContentRating(rating string) string {
	line := "Content Rating: " + rating
	if ratingPrompt := contentRatingPrompt(rating); ratingPrompt != "" {
		line += " (" + ratingPrompt + ")"
	}
	return line
}

func contentRatingPrompt(rating string) string {
	switch rating {
	case scenario.RatingG:
		return contentRatingG
	case scenario.RatingPG:
		return contentRatingPG
	case scenario.RatingPG13:
		return contentRatingPG13
	case scenario.RatingR:
		return contentRatingR
	default:
		return contentRatingPG13
	}
}

func getStatePrompt(gs *state.GameState, s *scenario.Scenario) (chat.ChatMessage, error) {
	return getStatePromptWithFocus(gs, s, nil)
}

func getStatePromptWithFocus(gs *state.GameState, s *scenario.Scenario, ruling *chat.Ruling) (chat.ChatMessage, error) {
	if gs == nil {
		return chat.ChatMessage{}, fmt.Errorf("game state or scene is nil")
	}

	var scene *scenario.Scene
	if gs.SceneName != "" {
		sc, ok := s.Scenes[gs.SceneName]
		if !ok {
			return chat.ChatMessage{}, fmt.Errorf("scene %s not found in scenario %s", gs.SceneName, s.Name)
		}
		scene = &sc
	}

	story := s.Story
	if scene != nil && scene.Story != "" {
		story += "\n\n" + scene.Story
	}

	ps := toPromptStateAt(gs, narratorViewLocation(gs, ruling))
	if ruling != nil {
		ps.FocusedActors = ruling.Focus.Actors
	}
	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf(statePromptTemplate, story, ps.ToString(ruling)),
	}, nil
}

const narratorHistoryDefault = 20

// BuildNarratorMessages assembles a slice of ChatMessages
// for the streaming narrator call.
func BuildNarratorMessages(gs *state.GameState, sc *scenario.Scenario, userMessage string, historyLimit int, ruling *chat.Ruling, refereeExtra ...string) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}
	if sc == nil {
		return nil, fmt.Errorf("scenario is required")
	}
	if historyLimit <= 0 {
		historyLimit = narratorHistoryDefault
	}

	// build truncated chat history; needed for dynamic system prompt
	history := gs.ChatHistory
	n := len(history)
	if n > historyLimit {
		history = history[n-historyLimit:]
	}

	dynamicNarratorPrompt, err := getDynamicNarratorPrompt(gs, sc, history, ruling)
	if err != nil {
		return nil, fmt.Errorf("error building system prompt: %w", err)
	}

	msgs := []chat.ChatMessage{
		// Persistent cacheable system prompt
		{
			Role:         chat.ChatRoleSystem,
			Content:      getPersistentNarratorPrompt(gs.Narrator, gs.PC, gs.Rules, sc.Rating),
			IsPersistent: true,
		},
		// Dynamic systemprompt built from current game state
		{
			Role:    chat.ChatRoleSystem,
			Content: dynamicNarratorPrompt,
		},
	}

	// add truncatedchat history
	msgs = append(msgs, history...)

	// add user prompt with referee rulings
	msgs = append(msgs, getUserPrompt(gs, userMessage, ruling, refereeExtra...))

	if gs.IsEnded {
		msgs = append(msgs, narratorGameEndMessage(sc))
	}
	return msgs, nil
}

func getDynamicNarratorPrompt(gs *state.GameState, sc *scenario.Scenario, history []chat.ChatMessage, ruling *chat.Ruling) (string, error) {
	statePrompt, err := getStatePromptWithFocus(gs, sc, ruling)
	if err != nil {
		return "", fmt.Errorf("error getting state prompt: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(statePrompt.Content)

	view := toPromptStateAt(gs, narratorViewLocation(gs, ruling))
	if len(view.Monsters) > 0 {
		rs := getRuleSet(gs.Rules)
		sb.WriteString("\n\n### Monsters\n")
		sb.WriteString(rs.Monsters)
	}

	if dests := movementDestinations(gs, ruling); len(dests) > 0 {
		name := dests[0].Loc.Name
		if name == "" {
			name = dests[0].Key
		}
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "The PC is arriving at %s. Briefly describe the new location.", name)
	}

	if historyHasStoryEvent(history) {
		sb.WriteString("\n\n")
		sb.WriteString(worldEventContinuationRule)
	}

	contingencyPrompts := gs.GetContingencyPrompts(sc)
	if len(contingencyPrompts) > 0 {
		sb.WriteString("\n\n<contingencies>\n")
		for i, prompt := range contingencyPrompts {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, prompt)
		}
		sb.WriteString("</contingencies>")
	}
	return sb.String(), nil
}

func movementDestinations(gs *state.GameState, ruling *chat.Ruling) []resolvedLocation {
	if !movementAllowed(ruling) {
		return nil
	}
	return resolveMovementDestinations(gs, ruling.Focus.Locations)
}

func movementAllowed(ruling *chat.Ruling) bool {
	return ruling != nil && ruling.Allowed && ruling.Scope == chat.RulingScopeMovement
}

func narratorViewLocation(gs *state.GameState, ruling *chat.Ruling) string {
	if dests := movementDestinations(gs, ruling); len(dests) > 0 {
		return dests[0].Key
	}
	return gs.Location
}

func historyHasStoryEvent(history []chat.ChatMessage) bool {
	for _, m := range history {
		if m.IsStoryEvent {
			return true
		}
	}
	return false
}

// getUserPrompt constructs the user prompt with referee rulings.
// extra rulings are appended to the narrator rules and the overall
// allow/disallow ruling.
func getUserPrompt(gs *state.GameState, userMessage string, ruling *chat.Ruling, extra ...string) chat.ChatMessage {
	if userMessage == "" {
		return chat.ChatMessage{}
	}

	allRules := make([]string, len(narratorRules))
	copy(allRules, narratorRules)
	if gs.Narrator != nil && len(gs.Narrator.Rules) > 0 {
		allRules = append(allRules, gs.Narrator.Rules...)
	}
	rulesBlock := "\n\n" + formatRulesBlock(allRules)

	refereeBlock := ""
	if ruling != nil {
		parts := append([]string{ruling.NarratorText()}, extra...)
		refereeBlock = "\n\n" + formatRefereeBlock(parts...)
	}

	return chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: userMessage + rulesBlock + refereeBlock,
	}
}

func narratorGameEndMessage(sc *scenario.Scenario) chat.ChatMessage {
	finalPrompt := gameEndSystemPrompt
	if sc.GameEndPrompt != "" {
		finalPrompt += "\n\n" + sc.GameEndPrompt
	}
	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: finalPrompt,
	}
}
