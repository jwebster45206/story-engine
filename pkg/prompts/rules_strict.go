package prompts

// strictRuleSet is the default ruleset: player movement and invention are
// constrained to the WORLD STATE.
var strictRuleSet = ruleSet{
	Locations:       strictLocations,
	Monsters:        strictMonsters,
	WorldStateRules: strictWorldStateRules,
	Movement:        strictRefereeMovement,
	Items:           strictRefereeItems,
	NPCs:            strictRefereeNPCs,
	Global:          strictRefereeGlobal,
	Examples:        strictRefereeExamples,
}

var strictWorldStateRules = []string{
	"Narrate ONLY current_location. Do not narrate inside adjacent locations.",
	"Use the description verbatim or paraphrased. You may add ambient sensory detail (smell, temperature, distant sound). Do NOT introduce doors, alcoves, statues, furniture, mechanisms, NPCs, items, or monsters not listed above.",
	"If just_entered is true, give a brief opening description; otherwise do not re-describe the room - continue the action.",
}

const strictLocations = `When narrating what the player sees, draw from the WORLD STATE — it contains the current location's description, exits, items, and NPCs. Follow these priorities:
1. **Physical space first.** Use the location's description as your primary source. You may add ambient sensory detail only (smell, temperature, distant sound). Do not add architecture, props, or named entities not in the WORLD STATE.
2. **Exits.** Weave real exits into the prose naturally. Do not list them mechanically, but let the player sense available paths ("A corridor stretches north; to the east, a heavy door stands ajar."). Never mention exits that aren't in the WORLD STATE.
3. **NPCs.** If characters are present at the location, include them in the scene — what they're doing, how they react. Don't ignore them.
4. **Items.** Mention visible items when it feels natural, but you may also let the player discover them through exploration. Not every item needs to be announced on arrival.
5. **Source priority.** The scenario description is your primary authority. You may supplement with general knowledge for atmospheric detail (what a jungle smells like, how torchlight behaves), but never use training data to invent facts — new objects, passages, characters, or history — that the scenario doesn't define.`

const strictMonsters = `Monsters are listed in the WORLD STATE only when present at the player's location. If combat occurs, resolve it dramatically based on the listed AC/HP; defeated monsters (HP 0) are removed by the engine.`

const strictRefereeMovement = `- The player may only travel listed exits in current_location.
- Blocked exits are not usable.
- Invented destinations are not allowed.`

const strictRefereeItems = `- Interact only with items in user_inventory or "Items here".
- Picking up items that are not listed as interactable is not allowed.`

const strictRefereeNPCs = `- Only NPCs listed as "NPCs here" may be spoken to or acted on. NPCs elsewhere are out of reach this turn.`

const strictRefereeGlobal = `- The player roleplays as the Player Character (PC) only; never as any NPC.
- Stay within the WORLD STATE sandbox: only listed locations, items, NPCs, and monsters exist. Invented creatures, places, items, or powers are not allowed.
- Only listed monsters may be engaged.
- Ordinary PC actions that stay in that sandbox are allowed.`

const strictRefereeExamples = `Example:	"Allowed. The PC can move to the drawbridge."
Example:	"Not allowed. The PC cannot move to the banquet hall because it is blocked. The PC would be stopped by the guard. "

Example:	"Allowed. The PC attacks the giant rat."
Example:	"Not allowed. There is no giant rat to attack."
Example:	"Not allowed. The PC cannot dictate that the guard is defeated in the attack. The PC's attack occurs, but outcome is decided by the narrator."`
