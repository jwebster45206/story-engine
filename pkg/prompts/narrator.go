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

// systemPromptTemplate is the stitch point for the narrator system prompt.
// %s slots in order: narrator name, interpretation, narrator style, PC,
// locations, game mechanics, monsters.
const systemPromptTemplate = `You are %s, the omniscient narrator of a roleplaying text adventure. You describe the story to the user as it unfolds. You never discuss things outside of the game. Your perspective is third-person. You provide narration and NPC conversation, but you don't speak for the user.

### HOW YOU INTERPRET USER PROMPTS:
%s

### Writing rules for narrative output:
- By default, respond in 1 to 3 short paragraphs of 1 to 3 sentences each. The narrator style section below may override this default with its own length and structure guidelines.
- Normal narration must never use colons. Colons are reserved only for dialogue lines.  
- When a new character speaks, start a new paragraph and use the format:
  CharacterName: "Spoken line here."
- Always end your response on the world's side of the conversation. Close with the world, an NPC, or a situation in a state of waiting — not with the PC speaking, deciding, or acting. The player provides the PC's voice; you provide everything else.
  Example (wrong): Madam Eva: "What do you seek?" The PC steps forward and answers that they seek the cure.
  Example (right): Madam Eva: "What do you seek?" Her eyes hold yours across the fire, patient as stone.

### Narrator responses 
- Do not break the fourth wall. Do not acknowledge that you are an AI or a computer program. 
- Do not answer questions about the game mechanics or how to play. 
- If the user breaks character, gently remind them to stay in character. 
- Move the story forward gradually, allowing the user to explore and discover things on their own. 
%s
Your narrator style informs your voice, vocabulary, and output structure. It does not grant permission to ignore the game rules above.

### Player Character
%s

### Describing locations
%s

### Game mechanics:
%s

### Monsters
%s
`

// buildNarratorPrompt constructs the system prompt with narrator and PC prompts injected.
// mode selects the base ruleset (strict or relaxed). pc is optional - pass nil if no PC.
func buildNarratorPrompt(narrator *scenario.Narrator, pc *character.PC, mode state.RulesMode) string {
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
		rs.Interpretation,
		narratorPrompts,
		pcPrompt,
		rs.Locations,
		rs.GameMechanics,
		rs.Monsters,
	)
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

	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf(statePromptTemplate, story, ToPromptState(gs).ToString()),
	}, nil
}

const narratorHistoryDefault = 20

// BuildNarratorMessages assembles the streaming narrator call: system prompt
// (ruleset, rating, story, world state, contingencies), windowed history, the
// current user line with <rules> and optional <referee>, and a game-end
// message when the session has ended.
func BuildNarratorMessages(gs *state.GameState, sc *scenario.Scenario, userMessage string, historyLimit int, referee string) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}
	if sc == nil {
		return nil, fmt.Errorf("scenario is required")
	}
	if historyLimit <= 0 {
		historyLimit = narratorHistoryDefault
	}

	system, err := narratorSystemPrompt(gs, sc)
	if err != nil {
		return nil, fmt.Errorf("error building system prompt: %w", err)
	}

	msgs := []chat.ChatMessage{{
		Role:    chat.ChatRoleSystem,
		Content: system,
	}}
	msgs = append(msgs, windowHistory(gs.ChatHistory, historyLimit)...)
	if user := narratorUserTurn(gs, userMessage, referee); user.Content != "" {
		msgs = append(msgs, user)
	}
	if gs.IsEnded {
		msgs = append(msgs, narratorGameEndMessage(sc))
	}
	return msgs, nil
}

func narratorSystemPrompt(gs *state.GameState, sc *scenario.Scenario) (string, error) {
	var sb strings.Builder
	sb.WriteString(buildNarratorPrompt(gs.Narrator, gs.PC, gs.Rules))

	sb.WriteString("\n\nContent Rating: " + sc.Rating)
	if ratingPrompt := contentRatingPrompt(sc.Rating); ratingPrompt != "" {
		sb.WriteString(" (" + ratingPrompt + ")")
	}

	statePrompt, err := getStatePrompt(gs, sc)
	if err != nil {
		return "", err
	}
	sb.WriteString("\n\n" + statePrompt.Content)

	contingencyPrompts := gs.GetContingencyPrompts(sc)
	if len(contingencyPrompts) > 0 {
		sb.WriteString("\n\nSome important storytelling guidelines:\n\n")
		for i, prompt := range contingencyPrompts {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, prompt)
		}
	}
	return sb.String(), nil
}

func windowHistory(history []chat.ChatMessage, limit int) []chat.ChatMessage {
	n := len(history)
	if n == 0 {
		return nil
	}
	if n > limit {
		return history[n-limit:]
	}
	return history
}

func narratorUserTurn(gs *state.GameState, userMessage, referee string) chat.ChatMessage {
	if userMessage == "" {
		return chat.ChatMessage{}
	}

	allRules := make([]string, len(narratorRules))
	copy(allRules, narratorRules)
	if gs.Narrator != nil && len(gs.Narrator.Rules) > 0 {
		allRules = append(allRules, gs.Narrator.Rules...)
	}

	content := userMessage
	if rulesBlock := formatRulesBlock(allRules); rulesBlock != "" {
		content += "\n\n" + rulesBlock
	}
	if refBlock := formatRefereeBlock(referee); refBlock != "" {
		content += "\n\n" + refBlock
	}
	return chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: content,
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
