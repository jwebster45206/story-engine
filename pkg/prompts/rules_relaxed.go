package prompts

// relaxedRuleSet grants the player latitude to steer the world: free movement,
// improvised item interactions, player-introduced people/creatures/places, and
// out-of-ability actions played out rather than redirected. Narrator voice and
// output style remain identical to strict.
var relaxedRuleSet = ruleSet{
	Locations:       relaxedLocations,
	Monsters:        relaxedMonsters,
	WorldStateRules: relaxedWorldStateRules,
	Movement:        relaxedRefereeMovement,
	Items:           relaxedRefereeItems,
	NPCs:            relaxedRefereeNPCs,
	Global:          relaxedRefereeGlobal,
}

var relaxedWorldStateRules = []string{
	"The WORLD STATE is a starting point. You may add plausible scenery, props, and incidental detail; keep invented specifics consistent with what is already established.",
	"If just_entered is true, give a brief opening description; otherwise do not re-describe the room - continue the action.",
}

const relaxedLocations = `When narrating what the player sees, draw from the WORLD STATE as your starting point — it contains the current location's description, exits, items, and NPCs. Follow these priorities:
1. **Physical space first.** Use the location's description as your primary source. You may extend it with plausible architecture, props, and ambient sensory detail. Invented specifics should stay consistent with what is already established.
2. **Exits.** Weave known exits into the prose naturally. Do not list them mechanically, but let the player sense available paths.
3. **NPCs.** If characters are present at the location, include them in the scene — what they're doing, how they react. Don't ignore them.
4. **Items.** Mention visible items when it feels natural. Not every item needs to be announced on arrival.
5. **Source priority.** The scenario description is your primary authority. You may supplement with general knowledge; keep the tone and setting coherent.`

const relaxedMonsters = `Monsters listed in the WORLD STATE are authoritative for combat stats (AC/HP); defeated monsters (HP 0) are removed by the engine. Resolve encounters dramatically and consistently with the tone of the scenario.`

const relaxedRefereeMovement = `- Known exits are the obvious paths; other directions may be allowed.
- Blocked exits are soft obstacles; a plausible attempt to pass them may be allowed.`

const relaxedRefereeItems = `- Prefer listed items in user_inventory and "Items here".
- Improvised interactions with unlisted objects may be allowed.`

const relaxedRefereeNPCs = `- NPCs listed as "NPCs here" are present this turn.
- Player-introduced people may appear and be acted on.
- Do not speak or act for the Player Character.`

const relaxedRefereeGlobal = `- Honor people, creatures, places, and objects the player introduces.
- If the player attempts something outside the PC's defined abilities, the attempt is allowed; play it out rather than refuse.`
