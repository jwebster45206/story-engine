package prompts

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
