package prompts

import (
	"encoding/json"
	"fmt"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// ReducerPrompt is the session-stable reducer instruction body. BEFORE-state
// is attached separately so this block can be cached. Field shapes live in
// the provider schema (Anthropic tool / Venice json_schema).
const ReducerPrompt = `You are a backend reducer. Read the latest narrative and current game state, then output ONLY a JSON object matching the provided schema. No prose.

Do not invent scenes, locations, items, NPCs, or variables beyond those in the scenario. Empty arrays or objects are fine. Include every required field.

LOCATION
- Always set user_location to the player's current location.
- Move only via a listed, unblocked exit, exactly one step. Otherwise repeat the current location.

ITEMS
- Emit item_events only when possession changes or an item is used. Observing, examining, mentioning, negotiating, or a failed attempt is not an event.
- acquire: item ends with player. give: player → NPC. drop: player → location. move: explicit from→to. use: player uses a held item; consumed=true only if the narrative says so.
- Use canonical item IDs from the scenario/state.

NPC EVENTS
- Track location only when the narrative shows active movement (follows, leaves, or is sent and complies). Mentions, thoughts, and past or speculative location are not movement.
- Use canonical NPC IDs and location IDs.

SCENES
- scene_change {to, reason} only when a rule is satisfied this turn; otherwise null.

VARIABLES
- Only update variables that already exist. Ground updates in the narrator response.

GAME END
- true if the narrative is a definitive ending OR a rule ends the game this turn; otherwise false.

EXAMPLES
- "sees a sword" → item_events: []
- "picks up the sword from the table" → item_events:[{item:"Sword", action:"acquire", from:{type:"location", name:"Sword Chamber"}}]
- "gives bottle of rum to Calypso" → item_events:[{item:"Rum Bottle", action:"give", from:{type:"player"}, to:{type:"npc", name:"Calypso"}}]
- "uses bandage and it is consumed" → item_events:[{item:"Bandage", action:"use", consumed:true}]
- "repairs are discussed (rule:'Change scene to british_docks when repairs are started.')" → scene_change:null
- "repairs begin (same rule)" → scene_change:{to:"british_docks", reason:"repairs were started"}
`

// BuildReducerMessages assembles the background delta-extraction call: a
// persistent instruction block, a per-turn BEFORE-state block, the narrator
// reply, and a closing extract request. The player prompt is omitted so the
// reducer is grounded only in what the narrator confirms happened.
func BuildReducerMessages(gs *state.GameState, sc *scenario.Scenario, responseMessage string) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}
	if sc == nil {
		return nil, fmt.Errorf("scenario is required")
	}

	stateJSON, err := json.Marshal(ToBackgroundPromptState(gs))
	if err != nil {
		return nil, fmt.Errorf("marshal background prompt state: %w", err)
	}

	return []chat.ChatMessage{
		{
			Role:         chat.ChatRoleSystem,
			Content:      ReducerPrompt,
			IsPersistent: true,
		},
		{
			Role:    chat.ChatRoleSystem,
			Content: "BEFORE game state: " + string(stateJSON),
		},
		{
			Role:    chat.ChatRoleAgent,
			Content: responseMessage,
		},
		{
			Role:    chat.ChatRoleUser,
			Content: "Interpret the game state delta from assistant's last message.",
		},
	}, nil
}
