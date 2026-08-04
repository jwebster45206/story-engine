package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const GameEndSystemPrompt = `This user's session has ended. Regardless of the user's input, the game will not continue. Respond in a way that will wrap up the game in a narrative manner. End with a fancy "*.*.*.*.*.*. THE END .*.*.*.*.*.*" line, followed by instructions to use Ctrl+N to start a new game or Ctrl+C to exit.`

// ReducerPrompt provides instructions for translating narrative to game state delta
const ReducerPrompt = `You are a backend reducer. Read the latest narrative and current game state, then output ONLY a JSON object matching the provided schema. No prose.

OUTPUT SCHEMA (strict)
- user_location: string (always required)
- scene_change: object { to, reason } or null when no change
- item_events: array of { item, action, from?, to?, consumed?, evidence? } (always required, may be empty)
  • action ∈ {"acquire","give","drop","move","use"}
  • from/to.type ∈ {"player","npc","location"}; include name when type ≠ "player"
- npc_events: array of { npc_id, set_location } (always required, may be empty)
- set_vars: object (always required, may be empty)
- game_ended: boolean (always required) 

GENERAL RULES
- Do not invent scenes, locations, items, NPCs, or variables beyond those in the scenario.
- It is acceptable to output empty arrays or empty objects when nothing changes.
- Include all required fields every time.

LOCATION
- Always set user_location to the player’s current location.
- Movement only if destination is in current_location.exits, not blocked, and exactly one step.
- If no move, repeat the current location.

ITEMS
- Emit item_events only when possession changes or an item is used.
  • Observing/examining/mentioning/negotiating/failed attempts → no event.
  • acquire: item ends with player.
  • give: player → NPC.
  • drop: player → location.
  • move: explicit from→to between holders.
  • use: player uses an item they hold; set consumed=true only if narrative says so.
- Use canonical item IDs from the scenario/state.

NPC EVENTS
- Track NPC location changes when narrative explicitly indicates movement.
  • When an NPC follows the player to a new location
  • When an NPC is described as moving, leaving, or going somewhere
  • When an NPC is explicitly told to go somewhere and complies
- Format: {"npc_id": "gibbs", "set_location": "sleepy_mermaid"}
- Use canonical NPC IDs and location IDs from the scenario/state
- DO NOT track movements when:
  • NPCs are merely mentioned or thought about
  • Describing past events or speculation
  • NPC is described as being "somewhere" without active movement
- Examples:
  • "Gibbs follows you into the tavern." + user_location="sleepy_mermaid" → npc_events:[{npc_id:"gibbs", set_location:"sleepy_mermaid"}]
  • "You tell Calypso to meet you at the docks. She nods and heads out." + docks="port_royal_docks" → npc_events:[{npc_id:"calypso", set_location:"port_royal_docks"}]
  • "You think about Gibbs back at the ship." → npc_events:[] (no movement, just mention)

SCENES
- If a rule triggers a change in scene name, it is VERY IMPORTANT to include 'scene_change {to, reason}'.
- Otherwise set scene_change=null.

VARIABLES
- Use variables to reflect events and story state changes.
- Only update variables that already exist in the current game state.
- Set variables based on events in the player's most recent prompt and the narrator's response.
- The narrator's response may override the player's prompt.

GAME END
- true if narrative describes a definitive ending OR a rule ends the game this turn.
- false otherwise.

CONTINGENCY RULES
These scenario-provided rules can affect ANY field. Review all rules and apply all that are satisfied this turn. 
If a rule triggers a change in scene name, it is VERY IMPORTANT to include 'scene_change {to, reason}'.
Rules:
— %s

EXAMPLES
- "sees a sword" → item_events: []
- "picks up the sword from the table" →
  item_events:[{item:"Sword", action:"acquire", from:{type:"location", name:"Sword Chamber"}}]
- "gives bottle of rum to Calypso" →
  item_events:[{item:"Rum Bottle", action:"give", from:{type:"player"}, to:{type:"npc", name:"Calypso"}}]
- "uses bandage and it is consumed" →
  item_events:[{item:"Bandage", action:"use", consumed:true}]
- "repairs begin (rule:'Change scene to british_docks when repairs are started.')" →
  scene_change:{to:"british_docks", reason:"repairs were started"}
- "repairs are discussed (rule:'Change scene to british_docks when repairs are started.')" →
  scene_change:{} (no change, rule not triggered)
- "sees the sword in stone (rule:'Set the scene to sword_achieved when the sword is pulled from the stone.')" →
  scene_change:{} (no change, rule not triggered)
- "pulls the sword from the stone (rule:'Set the scene to sword_achieved when the sword is pulled from the stone.')" →
  scene_change:{to:"sword_achieved", reason:"player pulled sword from stone"},
  item_events:[{item:"Sword", action:"acquire", from:{type:"location", name:"sword room"}}]
`

// GlobalContingencyRules contains the contingency rules that apply to all scenes.
// Contingency rules are non-user-facing rules that affect background updates of gamestate.
var GlobalContingencyRules []string = []string{
	"If the player suffers major physical harm, the game ends.",
	"If the player repeatedly tries to break character, the game ends.",
}

// The following are user-facing rules that affect storytelling responses.
// Content rating prompts
const ContentRatingG = `Write content suitable for young children. Avoid violence, romance and scary elements. Use simple language and positive messages. `
const ContentRatingPG = `Write content suitable for children and families. Mild peril or tension is okay, but avoid strong language, explicit violence, or dark themes. `
const ContentRatingPG13 = `Write content appropriate for teenagers. You may include mild swearing, romantic tension, action scenes, and complex emotional themes, but avoid explicit adult situations, graphic violence, or drug use. `
const ContentRatingR = `Write with full freedom for adult audiences. All content should progress the story. `

// NarratorRules are the highest-priority output constraints, injected into every user turn
// as a <rules> block appended after the player's message.
var NarratorRules = []string{
	"Stay within the story world. Only NPCs, locations, items, and monsters defined in the WORLD STATE may appear — invent nothing.",
	"Do not act or speak for the Player Character. The player provides the PC's voice.",
	"Resolve exactly one action, exchange, or location reveal — then stop and let the player respond.",
}

// FormatRulesBlock formats a slice of rule strings into a <rules> block
// suitable for appending to a user message.
func FormatRulesBlock(rules []string) string {
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

// StatePromptTemplate provides a rich context for the LLM to understand the scenario and current game state.
// The %s for world state is already wrapped in <world_state>...</world_state> tags by PromptState.ToString,
// so no additional delimiter is needed.
const StatePromptTemplate = "The user is roleplaying this scenario: %s\n\nThe following describes the immediately surrounding world.\n\n%s\n"

// BuildSystemPrompt constructs the system prompt with narrator and PC prompts injected.
// mode selects the base ruleset (strict or relaxed). pc is optional - pass nil if no PC.
func BuildSystemPrompt(narrator *scenario.Narrator, pc *actor.PC, mode state.RulesMode) string {
	narratorPrompts := ""
	narratorName := "the narrator"
	if narrator != nil {
		narratorPrompts = narrator.GetPromptsAsString()
		narratorName = narrator.Name
	}
	pcPrompt := ""
	if pc != nil {
		pcPrompt = actor.BuildPrompt(pc)
	}
	rs := GetRuleSet(mode)
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

// GetContentRatingPrompt returns the appropriate content rating prompt
func GetContentRatingPrompt(rating string) string {
	switch rating {
	case scenario.RatingG:
		return ContentRatingG
	case scenario.RatingPG:
		return ContentRatingPG
	case scenario.RatingPG13:
		return ContentRatingPG13
	case scenario.RatingR:
		return ContentRatingR
	default:
		return ContentRatingPG13 // Default to PG-13
	}
}

// GetStatePrompt provides gameplay and story instructions to the LLM.
// It also provides scenario context and current game state context.
func GetStatePrompt(gs *state.GameState, s *scenario.Scenario) (chat.ChatMessage, error) {
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
		Content: fmt.Sprintf(StatePromptTemplate, story, ToPromptState(gs).ToString()),
	}, nil
}
