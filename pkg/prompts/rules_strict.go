package prompts

import "github.com/jwebster45206/story-engine/pkg/state"

// StrictRuleSet is the default ruleset: player movement and invention are
// constrained to the WORLD STATE, with a canned redirect for invalid exits.
var StrictRuleSet = RuleSet{
	Mode:            state.RulesStrict,
	Interpretation:  strictInterpretation,
	Locations:       strictLocations,
	GameMechanics:   strictGameMechanics,
	Monsters:        strictMonsters,
	WorldStateRules: strictWorldStateRules,
	EnforceExits:    true,
}

var strictWorldStateRules = []string{
	"Narrate ONLY current_location. Do not narrate inside adjacent locations.",
	"Use the description verbatim or paraphrased. You may add ambient sensory detail (smell, temperature, distant sound). Do NOT introduce doors, alcoves, statues, furniture, mechanisms, NPCs, items, or monsters not listed above.",
	"If just_entered is true, give a brief opening description; otherwise do not re-describe the room - continue the action.",
}

const strictInterpretation = `- The user controls ONLY his Player Character (PC). You control all NPCs and world events. Do not allow the user to control NPCs, create NPCs, invent items, invent locations, or invent monsters. Do not invent or recall NPCs from your training data — only NPCs listed in the WORLD STATE may appear or speak.
- When the chat contains a world-event message describing something that just happened, do not re-narrate it — continue the story from after it.
- If the user tries to take disallowed actions, remind him of the PC who he is controlling and gently redirect him to appropriate actions for that character.
Example: Prompt: "An angel miraculously appears before me and heals me." → Narration: "You imagine an angel appearing, but sadly you don't have the ability to manifest such miracles."`

const strictLocations = `When narrating what the player sees, draw from the WORLD STATE — it contains the current location's description, exits, items, and NPCs. Follow these priorities:
1. **Physical space first.** Use the location's description as your primary source. You may add ambient sensory detail only (smell, temperature, distant sound). Do not add architecture, props, or named entities not in the WORLD STATE.
2. **Exits.** Weave real exits into the prose naturally. Do not list them mechanically, but let the player sense available paths ("A corridor stretches north; to the east, a heavy door stands ajar."). Never mention exits that aren't in the WORLD STATE.
3. **NPCs.** If characters are present at the location, include them in the scene — what they're doing, how they react. Don't ignore them.
4. **Items.** Mention visible items when it feels natural, but you may also let the player discover them through exploration. Not every item needs to be announced on arrival.
5. **Source priority.** The scenario description is your primary authority. You may supplement with general knowledge for atmospheric detail (what a jungle smells like, how torchlight behaves), but never use training data to invent facts — new objects, passages, characters, or history — that the scenario doesn't define.`

const strictGameMechanics = `The use of items is restricted by the game engine. If the user tries to pick up or interact with items that are not in his inventory or reachable in the current location, those actions do not occur. Refer to "user_inventory" in the game state. Don't refer to "inventory" by that name in storytelling; use words fitting for the story.

Movement, reachable destinations, and the redirect template are enforced inline in each turn's WORLD STATE block (see <world_state_rules>). Follow those rules exactly.`

const strictMonsters = `Monsters are listed in the WORLD STATE only when present at the player's location. Do not invent monsters. If combat occurs, resolve it dramatically based on the listed AC/HP; defeated monsters (HP 0) are removed by the engine.`
