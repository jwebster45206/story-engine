# Story Engine PC Character Design Plan (v1)

## Overview
We are introducing a **PC (Player Character)** system for Story Engine.  
PCs will be defined in standalone JSON files (like narrators), and later connected to combat and skill systems via the `d20` library.

The immediate goal is to support narrative usage, while laying groundwork for statistics and combat.

---

## Goals
- PCs are defined as **JSON documents** and can be loaded independently.
- PCs are based on a lightweight `PCSpec` struct for unmarshaling.
- A runtime `PC` struct embeds a `d20.Actor`
- Future versions can add combat and skill checks using `d20.Roll`.

---

## JSON Example (`pcs/fighter_01.json`)

```json
{
  "id": "fighter_01",
  "name": "Aric of Durnholde",
  "class": "Fighter",
  "level": 1,
  "race": "Human",
  "pronouns": "he/him",
  "description": "Disciplined and pragmatic, distrustful of magic. Seeks to protect the weak and earn a captain's commission.",
  "background": "Veteran soldier of the northern wars... [lengthy description]",
  "stats": {
    "strength": 17,
    "dexterity": 13,
    "constitution": 15,
    "intelligence": 10,
    "wisdom": 12,
    "charisma": 9
  },
  "hp": 12,
  "ac": 16,
  "combat_modifiers": {
    "strength": 3,
    "proficiency": 2
  },
  "attributes": {
    "perception": 1,
    "heroics": 1
  },
  "inventory": [
    "longsword",
    "shield",
    "chainmail",
    "rations (5 days)"
  ]
}
```

---

## Go Data Model

### Stats5e (Core D&D 5e Ability Scores)
```go
// Stats5e represents the six core D&D 5e ability scores
type Stats5e struct {
    Strength     int `json:"strength"`
    Dexterity    int `json:"dexterity"`
    Constitution int `json:"constitution"`
    Intelligence int `json:"intelligence"`
    Wisdom       int `json:"wisdom"`
    Charisma     int `json:"charisma"`
}

// ToAttributes converts Stats5e to a map for d20.Actor compatibility
func (s *Stats5e) ToAttributes() map[string]int {
    return map[string]int{
        "strength":     s.Strength,
        "dexterity":    s.Dexterity,
        "constitution": s.Constitution,
        "intelligence": s.Intelligence,
        "wisdom":       s.Wisdom,
        "charisma":     s.Charisma,
    }
}
```

### PCSpec (Serializable)
```go
type PCSpec struct {
    ID              string         `json:"id"`
    Name            string         `json:"name"`
    Class           string         `json:"class"`
    Level           int            `json:"level"`
    Race            string         `json:"race"`
    Pronouns        string         `json:"pronouns,omitempty"`
    Description     string         `json:"description,omitempty"`
    Background      string         `json:"background,omitempty"`
    Stats           Stats5e        `json:"stats"`
    HP              int            `json:"hp"`
    AC              int            `json:"ac"`
    CombatModifiers map[string]int `json:"combat_modifiers,omitempty"`
    Attributes      map[string]int `json:"attributes,omitempty"` // Skills, proficiencies, etc.
    Inventory       []string       `json:"inventory,omitempty"`
}
```

### PC (Runtime)
```go
type PC struct {
    Spec  *PCSpec
    Actor *d20.Actor // Built at runtime from PCSpec
}
```

---

## Loading Strategy

We will use the **builder pattern** (`LoadPC`) to keep `story-engine` decoupled from JSON implementation details.
The filename always overrides the ID in the JSON file.

```go
func LoadPC(path string) (*PC, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var spec PCSpec
    if err := json.Unmarshal(data, &spec); err != nil {
        return nil, err
    }
    
    // Filename overrides any ID in the JSON
    spec.ID = strings.TrimSuffix(filepath.Base(path), ".json")
    
    pc := &PC{
        Spec: &spec,
    }
    
    // Build d20.Actor from PCSpec
    // Start with core stats as attributes
    allAttrs := spec.Stats.ToAttributes()
    
    // Add additional attributes (skills, proficiencies, etc.)
    for k, v := range spec.Attributes {
        allAttrs[k] = v
    }
    
    // Build the actor
    actor, err := d20.NewActor(spec.Name, spec.HP, spec.AC).
        WithAttributes(allAttrs).
        WithCombatModifiers(spec.CombatModifiers).
        Build()
    if err != nil {
        return nil, fmt.Errorf("failed to build actor: %w", err)
    }
    
    pc.Actor = actor
    return pc, nil
}
```

---

## API Serialization

When returning PCs via API, we need to convert the runtime `PC` (with d20.Actor) back to JSON.
The Actor's current state should be reflected in the serialized PCSpec.

```go
// MarshalJSON converts PC back to PCSpec format for API responses
func (pc *PC) MarshalJSON() ([]byte, error) {
    // Create a response struct for serialization
    type PCResponse struct {
        ID              string         `json:"id"`
        Name            string         `json:"name"`
        Class           string         `json:"class"`
        Level           int            `json:"level"`
        Race            string         `json:"race"`
        Pronouns        string         `json:"pronouns,omitempty"`
        Description     string         `json:"description,omitempty"`
        Background      string         `json:"background,omitempty"`
        Stats           Stats5e        `json:"stats"`
        HP              int            `json:"hp"`
        AC              int            `json:"ac"`
        CombatModifiers map[string]int `json:"combat_modifiers,omitempty"`
        Attributes      map[string]int `json:"attributes,omitempty"`
        Inventory       []string       `json:"inventory,omitempty"`
    }
    
    // Start with the spec
    resp := PCResponse{
        ID:              pc.Spec.ID,
        Name:            pc.Spec.Name,
        Class:           pc.Spec.Class,
        Level:           pc.Spec.Level,
        Race:            pc.Spec.Race,
        Pronouns:        pc.Spec.Pronouns,
        Description:     pc.Spec.Description,
        Background:      pc.Spec.Background,
        Stats:           pc.Spec.Stats,
        CombatModifiers: pc.Spec.CombatModifiers,
        Inventory:       pc.Spec.Inventory,
    }
    
    resp.HP = pc.Actor.MaxHP()
    resp.AC = pc.Actor.AC()
    
    // Extract non-stat attributes back from Actor
    // (Exclude the 6 core stats since they're already in Stats field)
    resp.Attributes = make(map[string]int)
    coreStats := map[string]bool{
        "strength": true, "dexterity": true, "constitution": true,
        "intelligence": true, "wisdom": true, "charisma": true,
    }
    for k, v := range pc.Spec.Attributes {
        if !coreStats[k] {
            resp.Attributes[k] = v
        }
    }
    return json.Marshal(resp)
}
```

---

## API Endpoints

### GET `/v1/pcs`
List all available PCs.

**Response:**
```json
{
  "pcs": [
    {
      "id": "fighter_01",
      "name": "Aric of Durnholde",
      "class": "Fighter",
      "level": 1,
      "race": "Human"
    }
  ]
}
```

**TODO:** Implement pagination for large PC collections.

### GET `/v1/pcs/{id}`
Get a single PC by ID.

**Response:**
```json
{
  "id": "fighter_01",
  "name": "Aric of Durnholde",
  "class": "Fighter",
  "level": 1,
  "race": "Human",
  "pronouns": "he/him",
  "description": "Disciplined and pragmatic...",
  "background": "Veteran soldier of the northern wars...",
  "stats": {
    "strength": 17,
    "dexterity": 13,
    "constitution": 15,
    "intelligence": 10,
    "wisdom": 12,
    "charisma": 9
  },
  "hp": 12,
  "ac": 16,
  "combat_modifiers": {
    "strength": 3,
    "proficiency": 2
  },
  "attributes": {
    "perception": 1,
    "heroics": 1
  },
  "inventory": [
    "longsword",
    "shield",
    "chainmail",
    "rations (5 days)"
  ]
}
```

---

## Key Design Decisions

### Stats vs Attributes
- **Stats5e**: Six core D&D ability scores (STR, DEX, CON, INT, WIS, CHA) with strong typing
- **Attributes**: Flexible key-value map for skills, proficiencies, and custom properties
- Stats are converted to attributes when building the d20.Actor for unified runtime handling

### Combat Integration
- **CombatModifiers**: Map of modifier name → value (e.g., "strength": 3, "proficiency": 2)
- Matches d20.Modifier structure for seamless AttackRoll integration
- Initiative is rolled at combat start using dexterity modifier, not stored in PCSpec

### Inventory
- **Simple string array** (matches existing story-engine convention)
- Items are plain strings, not counts (e.g., `["longsword", "shield"]` not `{"longsword": 1}`)
- Consistent with current world/NPC/location item storage

### MaxHP Handling
- Omitted from PCSpec to avoid duplication
- d20.Actor tracks both currentHP and maxHP at runtime
- PCSpec.HP represents maxHP for serialization

## Next Steps

### Part 1 

- [ ] Create `data/pcs/` directory with example characters (pirate captain, default/classic/generic, alexandra kane)
- [ ] Create `pkg/pc/` package with `Stats5e`, `PCSpec`, and `PC` types
- [ ] Implement `LoadPC()` builder function with filename-based ID extraction
- [ ] Implement `MarshalJSON()` for PC to handle Actor → PCSpec conversion
- [ ] Unit test PCSpec.

### Part 1.5

- [ ] Add API endpoints: `GET /v1/pcs` and `GET /v1/pcs/{id}`
- [ ] Unit test endpoints.
- [ ] Update README in main directory.
- [ ] Write README in data/pcs describing how to create 5e PCs, and generic PCs. 

### Part 2

- [ ] Wire PC selection into gamestate creation. Fall back to default/classic/generic if all else fails. 
- [ ] Add support for default PC to scenario spec.
- [ ] Inject character into narrative prompt. Not reducer. (TODO: reduce most important elements to a compact prompt)
- [ ] Add default PC to pirate scenario, and update prompts so that character is separated from story.

### Part 3

- [ ] Integ test to ensure the correct PC is being played, using pirate scenario and pirate captain. Must use a detail that's only available in the character file.