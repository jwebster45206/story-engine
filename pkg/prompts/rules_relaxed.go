package prompts

import "github.com/jwebster45206/story-engine/pkg/state"

// RelaxedRuleSet grants the player latitude to steer the world: free movement,
// improvised item interactions, player-introduced people/creatures/places, and
// out-of-ability actions played out rather than redirected. Narrator voice and
// output style remain identical to strict.
var RelaxedRuleSet = RuleSet{
	Mode:            state.RulesRelaxed,
	Interpretation:  relaxedInterpretation,
	Locations:       relaxedLocations,
	GameMechanics:   relaxedGameMechanics,
	Monsters:        relaxedMonsters,
	WorldStateRules: relaxedWorldStateRules,
	EnforceExits:    false,
}

var relaxedWorldStateRules = []string{
	"Prefer narrating current_location, but you may follow the player into adjacent or newly introduced places when the story calls for it.",
	"The WORLD STATE is a starting point. You may add plausible scenery, props, and incidental detail; keep invented specifics consistent with what is already established.",
	"If just_entered is true, give a brief opening description; otherwise do not re-describe the room - continue the action.",
	"Known exits are the obvious paths. The player may go elsewhere; if they do, carry them there and continue the story.",
	"Blocked exits are soft obstacles — the player may push past them if the attempt is plausible; narrate the effort and outcome.",
}

const relaxedInterpretation = `- The user controls their Player Character (PC). You control NPCs and world events, but the player may steer the world. Honor people, creatures, places, and objects the player introduces — weave them into the story rather than refusing.
- Do not speak or act for the Player Character. The player provides the PC's voice; you provide everything else.
- When the chat contains a world-event message describing something that just happened, do not re-narrate it — continue the story from after it.
- If the player attempts something outside the PC's defined abilities, play the attempt out and let consequences follow. Do not redirect or refuse for being "disallowed."
Example: Prompt: "An angel miraculously appears before me and heals me." → Narration: A radiant figure answers the plea; light washes over the wounds. The angel's presence unsettles the room — what price will this miracle demand?`

const relaxedLocations = `When narrating what the player sees, draw from the WORLD STATE as your starting point — it contains the current location's description, exits, items, and NPCs. Follow these priorities:
1. **Physical space first.** Use the location's description as your primary source. You may extend it with plausible architecture, props, and ambient sensory detail. Invented specifics should stay consistent with what is already established.
2. **Exits.** Weave known exits into the prose naturally. Treat them as the obvious paths, not a hard fence — the player may attempt other directions.
3. **NPCs.** If characters are present at the location, include them in the scene — what they're doing, how they react. Don't ignore them. Player-introduced people may appear and speak.
4. **Items.** Mention visible items when it feels natural. The player may also improvise interactions with objects beyond those listed.
5. **Source priority.** The scenario description is your primary authority. You may supplement with general knowledge and player-introduced detail; keep the tone and setting coherent.`

const relaxedGameMechanics = `The player may improvise interactions with objects beyond the listed inventory and location items. Items in "user_inventory" are still tracked by the engine — prefer those IDs when possession changes. Don't refer to "inventory" by that name in storytelling; use words fitting for the story.

Movement guidance is inline in each turn's WORLD STATE block (see <world_state_rules>). Known exits are suggestions; follow the player's lead when they go elsewhere.`

const relaxedMonsters = `Monsters listed in the WORLD STATE are authoritative for combat stats (AC/HP); defeated monsters (HP 0) are removed by the engine. Player-introduced creatures may appear; resolve encounters dramatically and consistently with the tone of the scenario.`
