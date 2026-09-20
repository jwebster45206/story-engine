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
}

const strictLocations = `When narrating what the player sees, draw from the WORLD STATE — it contains the current location's description, exits, items, and NPCs. Follow these priorities:
1. **Exits.** Weave real exits into the prose naturally. Do not list them mechanically, but let the player sense available paths ("A corridor stretches north; to the east, a heavy door stands ajar."). Never mention exits that aren't in the WORLD STATE.
2. **NPCs.** If characters are present at the location, include them in the scene — what they're doing, how they react. Don't ignore them.
3. **Items.** Mention visible items when it feels natural, but you may also let the player discover them through exploration. Not every item needs to be announced on arrival.`

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

const strictRefereeExamples = `Example:	{"allowed":true,"reasoning":"You can move to the drawbridge.","reaction":null,"scope":"movement","focus":{"actors":[],"locations":["drawbridge"]}}
Example:	{"allowed":false,"reasoning":"You cannot move to the banquet hall because it is blocked.","reaction":"The PC would be stopped by the guard.","scope":"movement","focus":{"actors":[],"locations":["banquet hall"]}}

Example:	{"allowed":true,"reasoning":"You attack the giant rat.","reaction":null,"scope":"combat","focus":{"actors":["Giant Rat"],"locations":[]}}
Example:	{"allowed":false,"reasoning":"There is no giant rat to attack.","reaction":null,"scope":"combat","focus":{"actors":[],"locations":[]}}
Example:	{"allowed":false,"reasoning":"You cannot dictate that the guard is defeated in the attack. Your attack occurs, but the outcome is decided by the narrator.","reaction":null,"scope":"combat","focus":{"actors":["guard"],"locations":[]}}`
