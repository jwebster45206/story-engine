package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const imagePromptSystemPrompt = `You are an expert at writing prompts for AI image generation tools (Stable Diffusion, Midjourney, DALL-E, etc.).
Based on the game context provided, write a single image generation prompt (120–180 words) that captures the most recent moment of the story.
You will be given a conversation history for broader context, but focus primarily on the last 2 exchanges (the final user message and narrator response).
Draw on ALL provided details to build a rich visual:
- Setting: location name, description, notable items or props present
- Player character: name, race, class, physical description
- NPCs present: name, type, disposition, physical description
- Action or mood derived from the latest exchange
- Lighting, atmosphere, and visual style that fit the tone of the scene
Output ONLY the image prompt. No commentary, no explanation, no quotation marks.`

// ImagePromptHistoryLimit is the maximum number of chat history messages included
// for context when building an image prompt. Matches the default used by Builder.
const ImagePromptHistoryLimit = 20

// BuildImagePromptMessages constructs the LLM message array used to generate an
// image prompt for the most recent narrative exchange in the game state.
//
// Returns an error if the game state has no chat history (no moment to illustrate).
func BuildImagePromptMessages(gs *state.GameState, s *scenario.Scenario) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("game state is required")
	}
	if s == nil {
		return nil, fmt.Errorf("scenario is required")
	}
	if len(gs.ChatHistory) == 0 {
		return nil, fmt.Errorf("no chat history available: there is no narrative moment to illustrate")
	}

	var sb strings.Builder

	// Current location
	sb.WriteString("SETTING: ")
	sb.WriteString(gs.Location)
	if loc, ok := gs.WorldLocations[gs.Location]; ok {
		if loc.Description != "" {
			sb.WriteString("\n")
			sb.WriteString(loc.Description)
		}
		if len(loc.Items) > 0 {
			sb.WriteString("\nNotable items/props: ")
			sb.WriteString(strings.Join(loc.Items, ", "))
		}
	}

	// Player character description
	if gs.PC != nil && gs.PC.Spec != nil {
		spec := gs.PC.Spec
		sb.WriteString("\n\nPLAYER CHARACTER: ")
		sb.WriteString(spec.Name)
		if spec.Race != "" {
			sb.WriteString(" | Race: ")
			sb.WriteString(spec.Race)
		}
		if spec.Class != "" {
			sb.WriteString(" | Class: ")
			sb.WriteString(spec.Class)
		}
		if spec.Pronouns != "" {
			sb.WriteString(" | Pronouns: ")
			sb.WriteString(spec.Pronouns)
		}
		if spec.Description != "" {
			sb.WriteString("\n")
			sb.WriteString(spec.Description)
		}
		if spec.Background != "" {
			sb.WriteString("\nBackground: ")
			sb.WriteString(spec.Background)
		}
	}

	// NPCs present at the current location (or important to the story)
	for _, npc := range gs.NPCs {
		if npc.Location != gs.Location && !npc.IsImportant {
			continue
		}
		sb.WriteString("\n\nNPC: ")
		sb.WriteString(npc.Name)
		if npc.Type != "" {
			sb.WriteString(" | Type: ")
			sb.WriteString(npc.Type)
		}
		if npc.Disposition != "" {
			sb.WriteString(" | Disposition: ")
			sb.WriteString(npc.Disposition)
		}
		if npc.Description != "" {
			sb.WriteString("\n")
			sb.WriteString(npc.Description)
		}
	}

	// Chat history (windowed)
	history := gs.ChatHistory
	if len(history) > ImagePromptHistoryLimit {
		history = history[len(history)-ImagePromptHistoryLimit:]
	}

	// Mark the boundary where the "focus" exchanges begin (last 2 messages).
	focusStart := len(history) - 2
	if focusStart < 0 {
		focusStart = 0
	}

	sb.WriteString("\n\nCONVERSATION HISTORY:")
	for i, msg := range history {
		if i == focusStart {
			sb.WriteString("\n\n-- FOCUS ON THE FOLLOWING EXCHANGES FOR THE IMAGE --")
		}
		switch msg.Role {
		case chat.ChatRoleUser:
			sb.WriteString("\nUser: ")
		case chat.ChatRoleAgent:
			sb.WriteString("\nNarrator: ")
		default:
			continue
		}
		sb.WriteString(msg.Content)
	}

	sb.WriteString("\n\nGenerate an image generation prompt for this moment.")

	return []chat.ChatMessage{
		{Role: chat.ChatRoleSystem, Content: imagePromptSystemPrompt},
		{Role: chat.ChatRoleUser, Content: sb.String()},
	}, nil
}
