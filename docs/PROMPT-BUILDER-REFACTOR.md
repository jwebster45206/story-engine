# Prompt Builder Refactor Plan

## Updates After Storage Refactor (October 2025)

**Key Changes:**
1. ✅ **Storage refactor completed** - Clean separation between storage layer (`internal/storage`) and domain models (`pkg/*`)
2. 🎯 **Phase 0 REVISED** - We will embed Narrator and PC in GameState, but load Scenario on each request
3. 🔄 **Partial optimization** - Narrator/PC loaded once at gamestate creation, scenario loaded per-request

**Impact on this plan:**
- Phase 0 is now part of the refactor (not deferred)
- Handlers will load narrator/PC ONCE at gamestate creation
- Chat handlers will load scenario from storage on each request (TODO: add caching layer)
- Prompt builder receives scenario as parameter (not from GameState)
- GameState methods keep scenario parameter: `GetStatePrompt(scenario)`, `GetContingencyPrompts(scenario)`, `LoadScene(scenario, sceneName)`

**Performance & Design Benefits:**
- **Before**: Every chat = 2-3 file reads (scenario + narrator + gamestate from Redis)
- **After**: Every chat = 1 file read (scenario) + 1 Redis read (gamestate with narrator/PC)
- Scenarios can be updated and running games see the changes
- Narrator voice and PC stats remain stable for running games
- TODO: Add scenario caching layer to reduce filesystem reads to zero

---

## Current State Analysis

### The Problem

`GameState.GetChatMessages()` is doing too much and violating separation of concerns:

```go
func (gs *GameState) GetChatMessages(requestMessage string, requestRole string, 
    s *scenario.Scenario, count int, storyEventPrompt string) ([]chat.ChatMessage, error)
```

**What it currently does:**
1. ✅ Extracts gamestate data (appropriate for GameState)
2. ❌ Loads narrator from filesystem (I/O operation)
3. ❌ Builds complex system prompts (formatting/presentation logic)
4. ❌ Assembles message arrays (LLM-specific concerns)
5. ❌ Handles history windowing (could be elsewhere)
6. ❌ Manages story events (unclear responsibility)

**Issues:**
- GameState has file I/O dependencies (`scenario.LoadNarrator`)
- Hard to test in isolation
- Violates Single Responsibility Principle
- TODO comment acknowledges this: `// TODO: This func needs to be refactored`

### Current Architecture

```
Handler (chat.go)
    ↓
GameState.GetChatMessages()  ← DOES EVERYTHING
    ↓
LLMService.Chat(messages)
```

## LLM Provider Differences

After analyzing the codebase, here's what differs by provider:

### Anthropic (Claude)
- **Special handling**: Requires system messages to be extracted and sent in a separate `system` field
- **Message transformation**: Uses `splitChatMessages()` to separate system vs conversation messages
- **Unique to Anthropic**: Cannot have system messages in the messages array

### Venice AI
- **Standard OpenAI format**: Accepts system messages in the messages array
- **No transformation needed**: Passes messages through directly

### Ollama
- **Standard OpenAI format**: Accepts system messages in the messages array
- **No transformation needed**: Passes messages through directly

### Conclusion
**Only Anthropic requires message transformation.** Venice and Ollama use standard OpenAI-compatible format.

## Architectural Approach: Dedicated Prompt Builder
**Philosophy**: Prompt building is complex enough to be its own concern.

**Location**: `pkg/prompts` - This is domain logic that operates on domain models (GameState, Scenario), not infrastructure, so it belongs in `pkg/` where it can be reused and doesn't depend on internal implementation details.

```go
// pkg/prompts/builder.go (new package)
package prompts

import (
    "github.com/jwebster45206/story-engine/pkg/chat"
    "github.com/jwebster45206/story-engine/pkg/scenario"
    "github.com/jwebster45206/story-engine/pkg/state"
)

// Builder constructs chat messages for LLM interaction
type Builder struct {
    historyLimit int
}

func NewBuilder(historyLimit int) *Builder {
    return &Builder{historyLimit: historyLimit}
}

// BuildChatMessages constructs the full message array for LLM consumption
func (b *Builder) BuildChatMessages(
    gs *state.GameState,
    scenario *scenario.Scenario,
    narrator *scenario.Narrator,
    userMessage string,
    userRole string,
) ([]chat.ChatMessage, error) {
    
    messages := make([]chat.ChatMessage, 0)
    
    // 1. System prompt
    systemPrompt := b.buildSystemPrompt(gs, scenario, narrator)
    messages = append(messages, chat.ChatMessage{
        Role:    chat.ChatRoleSystem,
        Content: systemPrompt,
    })
    
    // 2. Chat history (windowed)
    history := b.windowHistory(gs.ChatHistory, b.historyLimit)
    messages = append(messages, history...)
    
    // 3. User message
    messages = append(messages, chat.ChatMessage{
        Role:    userRole,
        Content: userMessage,
    })
    
    // 4. Story events (if any)
    if storyEvents := gs.GetStoryEvents(); storyEvents != "" {
        messages = append(messages, chat.ChatMessage{
            Role:    chat.ChatRoleAgent,
            Content: storyEvents,
        })
    }
    
    // 5. Final reminders
    finalPrompt := b.buildFinalPrompt(gs, scenario)
    messages = append(messages, chat.ChatMessage{
        Role:    chat.ChatRoleSystem,
        Content: finalPrompt,
    })
    
    return messages, nil
}

// buildSystemPrompt constructs the main system prompt
func (b *Builder) buildSystemPrompt(
    gs *state.GameState,
    scenario *scenario.Scenario,
    narrator *scenario.Narrator,
) string {
    var sb strings.Builder
    
    // Narrator voice
    if narrator != nil {
        sb.WriteString(scenario.BuildSystemPrompt(narrator, gs.PC))
        sb.WriteString("\n\n")
    }
    
    // Content rating
    sb.WriteString("Content Rating: " + scenario.Rating)
    if ratingPrompt := scenario.GetContentRatingPrompt(scenario.Rating); ratingPrompt != "" {
        sb.WriteString(" (" + ratingPrompt + ")")
    }
    sb.WriteString("\n\n")
    
    // Game state context
    statePrompt, err := gs.GetStatePrompt(scenario)
    if err == nil {
        sb.WriteString(statePrompt.Content)
        sb.WriteString("\n\n")
    }
    
    // Contingency prompts
    if prompts := gs.GetContingencyPrompts(scenario); len(prompts) > 0 {
        sb.WriteString("Some important storytelling guidelines:\n\n")
        for i, prompt := range prompts {
            sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, prompt))
        }
    }
    
    return sb.String()
}

// buildFinalPrompt adds game-end or standard reminders
func (b *Builder) buildFinalPrompt(gs *state.GameState, scenario *scenario.Scenario) string {
    if gs.IsEnded {
        endPrompt := scenario.GameEndSystemPrompt
        if scenario.GameEndPrompt != "" {
            endPrompt += "\n\n" + scenario.GameEndPrompt
        }
        return endPrompt
    }
    return scenario.UserPostPrompt
}

// windowHistory returns the last N messages from history
func (b *Builder) windowHistory(history []chat.ChatMessage, limit int) []chat.ChatMessage {
    if len(history) <= limit {
        return history
    }
    return history[len(history)-limit:]
}
```

**Handler Usage:**
```go
// internal/handlers/chat.go
func (h *ChatHandler) handleChat(w http.ResponseWriter, r *http.Request) {
    // ... load gamestate, scenario, narrator from storage ...
    
    // Build messages using dedicated builder
    promptBuilder := prompts.NewBuilder(PromptHistoryLimit)
    messages, err := promptBuilder.BuildChatMessages(
        gs, 
        scenario, 
        narrator, 
        cmdResult.Message, 
        cmdResult.Role,
    )
    
    // LLM only receives messages - no domain knowledge needed
    response, err := h.llm.Chat(r.Context(), messages)
}
```

**LLM providers handle their own quirks:**
```go
// internal/services/anthropic.go - unchanged, still does its transformation
func (a *AnthropicService) Chat(ctx context.Context, messages []chat.ChatMessage) {
    // Anthropic-specific: extract system messages
    systemPrompt, conversationMessages := a.splitChatMessages(messages)
    
    // Call API with transformed format
    return a.chatCompletion(ctx, conversationMessages, systemPrompt)
}

// internal/services/venice.go - unchanged, passes through
func (v *VeniceService) Chat(ctx context.Context, messages []chat.ChatMessage) {
    // Venice: standard format, no transformation needed
    return v.chatCompletion(ctx, messages)
}
```

**Pros:**
- ✅ **Clear separation**: Prompt building is isolated and testable
- ✅ **Reusable**: Multiple handlers can use the same builder
- ✅ **Maintainable**: All prompt logic in one place
- ✅ **Flexible**: Easy to add new prompt features
- ✅ **Testable**: Can unit test prompt construction without I/O
- ✅ **LLM providers stay simple**: Only handle API-specific formatting

**Cons:**
- Adds a new package (minor)
- One more layer to understand (but clearer responsibilities)



## Package Location: `pkg/prompts` vs `internal/prompts`

### Decision: Use `pkg/prompts`

**Rationale:**
- **Domain logic**: Prompt building operates on domain models (GameState, Scenario, Narrator)
- **Reusable**: Could be used by CLI tools, tests, or other packages
- **No infrastructure dependencies**: Doesn't depend on HTTP, databases, or external services
- **Follows Go conventions**: `pkg/` for importable libraries, `internal/` for app-specific code

**Alternative (`internal/prompts`) would make sense if:**
- It had HTTP or database dependencies
- It was tightly coupled to handler implementation
- We never wanted it imported outside this project

But since it's pure transformation logic on domain models, `pkg/` is appropriate.

---

## Embedding Narrator and PC in GameState

### Current Problem

Every chat request requires loading:
```go
scenario := storage.GetScenario(gs.Scenario)  // Loads from disk every time
narrator := storage.GetNarrator(gs.NarratorID) // Loads from disk every time
```

The scenario loading is actually acceptable (and preferred) since:
- Scenarios can be updated and games should see those changes
- Scenarios contain story content that may evolve
- TODO: We can add a caching layer later to optimize this, using scenario_id+gamestate_uuid as a cache key that also allows updates to scenario

However, narrator and PC should be stable for a running game.

### Solution: Embed Narrator and PC in GameState

**Change GameState to store full objects for narrator and PC, but keep scenario as filename:**

```go
// pkg/state/gamestate.go
type GameState struct {
    ID               uuid.UUID                    `json:"id"`
    ModelName        string                       `json:"model_name,omitempty"`
    
    // Scenario: Keep as filename - load on each request (allows updates)
    Scenario         string                       `json:"scenario,omitempty"`
    
    // Narrator and PC: Embed full objects - load once at creation
    Narrator         *scenario.Narrator           `json:"narrator,omitempty"`
    PC               *actor.PC                    `json:"pc,omitempty"`
    
    SceneName        string                       `json:"scene_name,omitempty"`
    NPCs             map[string]actor.NPC         `json:"npcs,omitempty"`
    // ... rest of fields ...
}
```

**Benefits:**
1. ✅ **Narrator loaded once at creation** - voice/style is stable for running games
2. ✅ **PC loaded once at creation** - character stats don't change mid-game
3. ✅ **Scenario loaded per-request** - games can see content updates
4. ✅ **Balanced approach** - optimize what should be stable, flexibility where needed
5. ✅ **TODO: Add caching** - Can cache scenarios to reduce I/O to near-zero

**Tradeoffs:**
- ⚠️ One filesystem read per chat (scenario) - acceptable, and can be cached later
- ✅ Scenario updates propagate to running games (this is actually desirable for story content)
- ✅ Narrator/PC remain stable (correct behavior - don't change mid-game)

## Why This Approach is Best

1. **Separation of Concerns**
   - GameState: Manages game state (data)
   - Prompt Builder: Formats state for LLM consumption (presentation)
   - LLM Service: Handles API communication (infrastructure)
   - Handlers: Orchestrate the flow (control)

2. **Testability**
   - Test prompt building without LLM APIs
   - Test prompt building without file I/O
   - Test different scenarios easily

3. **LLM Provider Independence**
   - Prompt builder creates standard message format
   - LLM providers transform as needed (Anthropic: splitChatMessages)
   - No duplication of prompt logic per provider

4. **GameState Can Focus on State**
   - Remove I/O operations (LoadNarrator)
   - Remove formatting logic
   - Keep helper methods for data extraction:
     - `GetStatePrompt()` - serialize state to prompt
     - `GetContingencyPrompts()` - filter conditional prompts
     - `GetStoryEvents()` - return queued events

5. **Clear Dependencies**
   ```
   Handler
     ↓ uses
   Prompt Builder (depends on GameState, Scenario, Narrator)
     ↓ produces
   []chat.ChatMessage
     ↓ consumed by
   LLM Service (no domain knowledge)
   ```

## Migration Checklist

### Phase 0: Embed Narrator and PC in GameState
**Status**: We will implement this phase to reduce I/O and keep narrator/PC stable.

**Changes to GameState:**
- [ ] Update `GameState` struct to store `*scenario.Narrator` and keep `PC` as embedded (instead of separate load)
- [ ] Keep `Scenario` as string (filename) - load on each request
- [ ] Update `NewGameState()` constructor signature to accept `*scenario.Narrator` (PC already embedded)
- [ ] Keep scenario parameter in: `GetStatePrompt(scenario)`, `GetContingencyPrompts(scenario)`, `LoadScene(scenario, sceneName)`
- [ ] Ensure PC is fully embedded in GameState (may already be done)

**Changes to Handlers:**
- [ ] Update gamestate creation handler to load and embed narrator at creation time
- [ ] Update gamestate creation to ensure PC is fully embedded
- [ ] Chat handlers load scenario on each request (add TODO comment for caching)
- [ ] Remove narrator storage lookups from chat handlers
- [ ] Keep scenario loading in chat handlers

**Testing:**
- [ ] Test gamestate serialization/deserialization with embedded narrator
- [ ] Verify Redis storage size is acceptable
- [ ] Test that narrator remains stable for running games
- [ ] Test that scenario updates propagate to running games (desired behavior)
- [ ] Verify backward compatibility or migration path for existing gamestates

### Phase 1: Create Prompt Builder
**Note**: With Phase 0 completed, the builder receives scenario as parameter (loaded by handler), gets narrator from embedded GameState.

- [ ] Create `pkg/prompts` package
- [ ] Implement `Builder` with `BuildChatMessages()` - receives scenario as parameter, gets narrator from gamestate
- [ ] Implement `buildSystemPrompt()` helper
- [ ] Implement `buildFinalPrompt()` helper
- [ ] Implement `windowHistory()` helper
- [ ] Add comprehensive unit tests

**Builder Signature:**
```go
// WithGameState sets the gamestate (contains embedded narrator and PC)
func (b *Builder) WithGameState(gs *state.GameState) *Builder

// WithScenario sets the scenario (loaded by handler on each request)
func (b *Builder) WithScenario(scenario *scenario.Scenario) *Builder

// WithUserMessage sets the user's message and role
func (b *Builder) WithUserMessage(message, role string) *Builder

// WithHistoryLimit sets the chat history window size
func (b *Builder) WithHistoryLimit(limit int) *Builder
```

### Phase 2: Update Handlers
- [ ] Update chat handler to use `prompts.Builder`
- [ ] Remove `GetChatMessages()` from GameState
- [ ] Keep scenario parameter in GameState methods
- [ ] Chat handlers load scenario on each request (TODO comment for caching)
- [ ] Update handler tests

### Phase 3: Testing & Validation
- [ ] Run all unit tests
- [ ] Run integration tests
- [ ] Test with Anthropic (verify splitChatMessages still works)
- [ ] Test with Venice
- [ ] Test with Ollama
- [ ] Verify scenario updates propagate to running games
- [ ] Verify narrator/PC remain stable in running games
- [ ] Verify gamestate save/load works correctly
- [ ] Add TODO comments for scenario caching optimization

---

## Migration Plan

### Phase 0: Embed Narrator and PC in GameState

**Step 0.1: Update GameState structure**

```go
// pkg/state/gamestate.go
type GameState struct {
    ID               uuid.UUID                    `json:"id"`
    ModelName        string                       `json:"model_name,omitempty"`
    
    // Scenario: Keep as filename - load per request
    Scenario         string                       `json:"scenario,omitempty"`
    
    // Narrator: Embed full object - load once at creation
    Narrator         *scenario.Narrator           `json:"narrator,omitempty"`
    
    SceneName        string                       `json:"scene_name,omitempty"`
    PC               *actor.PC                    `json:"pc,omitempty"`
    NPCs             map[string]actor.NPC         `json:"npcs,omitempty"`
    WorldLocations   map[string]scenario.Location `json:"locations,omitempty"`
    Location         string                       `json:"user_location,omitempty"`
    Inventory        []string                     `json:"user_inventory,omitempty"`
    ChatHistory      []chat.ChatMessage           `json:"chat_history,omitempty"`
    TurnCounter      int                          `json:"turn_counter"`
    SceneTurnCounter int                          `json:"scene_turn_counter"`
    Vars             map[string]string            `json:"vars,omitempty"`
    IsEnded          bool                         `json:"is_ended"`
    ContingencyPrompts []string                   `json:"contingency_prompts,omitempty"`
    StoryEventQueue  []string                     `json:"story_event_queue,omitempty"`
    CreatedAt        time.Time                    `json:"created_at"`
    UpdatedAt        time.Time                    `json:"updated_at"`
}
```

**Step 0.2: Update gamestate creation in handler**

```go
// internal/handlers/gamestate.go

func (h *GameStateHandler) CreateGameState(w http.ResponseWriter, r *http.Request) {
    // ... parse request ...
    
    // Load scenario from storage (will be loaded on each chat request too)
    scenario, err := h.storage.GetScenario(r.Context(), req.Scenario)
    if err != nil {
        // handle error
    }
    
    // Load narrator from storage (ONCE at creation - then embedded)
    var narrator *scenario.Narrator
    narratorID := req.NarratorID
    if narratorID == "" {
        narratorID = scenario.NarratorID
    }
    if narratorID != "" {
        narrator, err = h.storage.GetNarrator(r.Context(), narratorID)
        if err != nil {
            h.logger.Warn("Failed to load narrator", "id", narratorID, "error", err)
            // Continue without narrator
        }
    }
    
    // Load PC spec from storage (ONCE at creation)
    pcID := req.PCID
    if pcID == "" {
        pcID = scenario.DefaultPC
    }
    if pcID == "" {
        pcID = "classic"
    }
    pcPath := filepath.Join("data/pcs", pcID+".json")
    pcSpec, err := h.storage.GetPCSpec(r.Context(), pcPath)
    if err != nil {
        // handle error with fallback
    }
    
    // Build PC from spec
    pc, err := actor.NewPCFromSpec(pcSpec)
    if err != nil {
        // handle error
    }
    
    // Create gamestate with embedded narrator (scenario as filename)
    gs := state.NewGameState(req.Scenario, narrator, h.modelName)
    gs.PC = pc
    gs.NPCs = scenario.NPCs
    gs.Location = scenario.OpeningLocation
    gs.WorldLocations = scenario.Locations
    gs.Vars = scenario.Vars
    
    // ... continue with initialization ...
}
```

**Step 0.3: Update NewGameState constructor**

```go
// pkg/state/gamestate.go

func NewGameState(scenarioFilename string, narrator *scenario.Narrator, modelName string) *GameState {
    return &GameState{
        ID:                 uuid.New(),
        ModelName:          modelName,
        Scenario:           scenarioFilename,  // Store filename, not object
        Narrator:           narrator,          // Store full object
        ChatHistory:        make([]chat.ChatMessage, 0),
        TurnCounter:        0,
        SceneTurnCounter:   0,
        Vars:               make(map[string]string),
        ContingencyPrompts: make([]string, 0),
        StoryEventQueue:    make([]string, 0),
        NPCs:               make(map[string]actor.NPC),
        WorldLocations:     make(map[string]scenario.Location),
        CreatedAt:          time.Now(),
        UpdatedAt:          time.Now(),
    }
}
```

**Step 0.4: Keep helper methods with scenario parameter**

```go
// pkg/state/gamestate.go

// GetStatePrompt still needs scenario parameter (loaded by handler)
func (gs *GameState) GetStatePrompt(s *scenario.Scenario) (chat.ChatMessage, error) {
    if gs == nil || s == nil {
        return chat.ChatMessage{}, fmt.Errorf("game state or scenario is nil")
    }

    var scene *scenario.Scene
    if gs.SceneName != "" {
        sc, ok := s.Scenes[gs.SceneName]
        if !ok {
            return chat.ChatMessage{}, fmt.Errorf("scene %s not found", gs.SceneName)
        }
        scene = &sc
    }

    ps := ToPromptState(gs)
    jsonScene, err := json.Marshal(ps)
    if err != nil {
        return chat.ChatMessage{}, err
    }

    story := s.Story
    if scene != nil && scene.Story != "" {
        story += "\n\n" + scene.Story
    }
    
    return chat.ChatMessage{
        Role:    chat.ChatRoleSystem,
        Content: fmt.Sprintf(scenario.StatePromptTemplate, story, jsonScene),
    }, nil
}

// GetContingencyPrompts still needs scenario parameter
func (gs *GameState) GetContingencyPrompts(s *scenario.Scenario) []string {
    if gs == nil || s == nil {
        return nil
    }

    var prompts []string

    // Filter scenario-level contingency prompts
    scenarioPrompts := scenario.FilterContingencyPrompts(s.ContingencyPrompts, gs)
    prompts = append(prompts, scenarioPrompts...)

    // Filter PC-level contingency prompts
    if gs.PC != nil && gs.PC.Spec != nil {
        pcPrompts := scenario.FilterContingencyPrompts(gs.PC.Spec.ContingencyPrompts, gs)
        prompts = append(prompts, pcPrompts...)
    }

    // Add custom gamestate-level prompts
    prompts = append(prompts, gs.ContingencyPrompts...)

    // Filter scene-level contingency prompts
    if gs.SceneName != "" {
        if scene, ok := s.Scenes[gs.SceneName]; ok {
            scenePrompts := scenario.FilterContingencyPrompts(scene.ContingencyPrompts, gs)
            prompts = append(prompts, scenePrompts...)
        }
    }

    // NPC-level contingency prompts
    for _, npc := range gs.NPCs {
        if npc.Location != gs.Location {
            continue
        }
        prompts = append(prompts, scenario.FilterContingencyPrompts(npc.ContingencyPrompts, gs)...)
    }

    return prompts
}

// LoadScene still needs scenario parameter
func (gs *GameState) LoadScene(s *scenario.Scenario, sceneName string) error {
    if s == nil {
        return fmt.Errorf("scenario is nil")
    }
    
    scene, ok := s.Scenes[sceneName]
    if !ok {
        return fmt.Errorf("scene %s not found", sceneName)
    }
    
    gs.SceneName = sceneName
    gs.SceneTurnCounter = 0

    // ... rest of scene loading logic ...
}
```

**Step 0.5: Update chat handlers to load scenario**

```go
// internal/handlers/chat.go

func (h *ChatHandler) handleChat(w http.ResponseWriter, r *http.Request) {
    // Load gamestate from Redis (contains embedded narrator and PC)
    gs, err := h.storage.LoadGameState(r.Context(), sessionID)
    
    // Load scenario from filesystem on each request
    // TODO: Add caching layer to reduce filesystem I/O
    scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)
    
    // Narrator comes from embedded gamestate (no lookup needed!)
    
    // Build messages (Phase 1 will introduce prompt builder)
    messages, err := gs.GetChatMessages(scenario, ...)
}
```

### Phase 1: Create Prompt Builder

**Step 1.1: Create package structure**
```
pkg/
  └── prompts/
      ├── builder.go       # Main prompt builder
      └── builder_test.go  # Unit tests
```

**Step 1.2: Implement Fluent Builder**
- Use fluent/builder pattern similar to `d20.Actor`
- Extract logic from `GameState.GetChatMessages()`
- **Receive scenario as parameter, get narrator from embedded GameState**
- Add comprehensive tests

```go
// pkg/prompts/builder.go
package prompts

// Fluent builder pattern for constructing chat messages
type Builder struct {
    gs           *state.GameState
    scenario     *scenario.Scenario
    userMessage  string
    userRole     string
    historyLimit int
    messages     []chat.ChatMessage
}

// New creates a new prompt builder
func New() *Builder {
    return &Builder{
        historyLimit: 20, // default
        messages:     make([]chat.ChatMessage, 0),
    }
}

// WithGameState sets the gamestate (contains embedded narrator and PC)
func (b *Builder) WithGameState(gs *state.GameState) *Builder {
    b.gs = gs
    return b
}

// WithScenario sets the scenario (loaded by handler on each request)
func (b *Builder) WithScenario(scenario *scenario.Scenario) *Builder {
    b.scenario = scenario
    return b
}

// WithUserMessage sets the user's message and role
func (b *Builder) WithUserMessage(message string, role string) *Builder {
    b.userMessage = message
    b.userRole = role
    return b
}

// WithHistoryLimit sets the chat history window size
func (b *Builder) WithHistoryLimit(limit int) *Builder {
    b.historyLimit = limit
    return b
}

// Build constructs and returns the final message array
func (b *Builder) Build() ([]chat.ChatMessage, error) {
    if b.gs == nil {
        return nil, fmt.Errorf("gamestate is required")
    }
    if b.scenario == nil {
        return nil, fmt.Errorf("scenario is required")
    }
    
    // 1. System prompt
    b.addSystemPrompt()
    
    // 2. Windowed chat history
    b.addHistory()
    
    // 3. User message
    b.addUserMessage()
    
    // 4. Story events (if any)
    b.addStoryEvents()
    
    // 5. Final reminders
    b.addFinalPrompt()
    
    return b.messages, nil
}

// Private helpers for building each section
// - addSystemPrompt(): builds from gs.Narrator (embedded), scenario (parameter), rating, state, contingency prompts
// - addHistory(): windows chat history to limit
// - addUserMessage(): adds the current user message
// - addStoryEvents(): adds queued story events if present
// - addFinalPrompt(): adds game-end or standard reminders from scenario
```

**Handler Usage:**
```go
// internal/handlers/chat.go

// Load gamestate (contains embedded narrator and PC)
gs, err := h.storage.LoadGameState(r.Context(), sessionID)

// Load scenario from filesystem
// TODO: Add caching layer to optimize this
scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)

// Build messages - narrator from gamestate, scenario from parameter
messages, err := prompts.New().
    WithGameState(gs).              // Narrator and PC embedded!
    WithScenario(scenario).         // Loaded on each request
    WithUserMessage(msg, role).
    WithHistoryLimit(PromptHistoryLimit).
    Build()
```

**Alternative: Keep Simple Constructor for Common Case:**
```go
// pkg/prompts/builder.go

// BuildMessages is a convenience function for the common case
func BuildMessages(
    gs *state.GameState,
    scenario *scenario.Scenario,
    message string, 
    role string, 
    historyLimit int,
) ([]chat.ChatMessage, error) {
    return New().
        WithGameState(gs).
        WithScenario(scenario).
        WithUserMessage(message, role).
        WithHistoryLimit(historyLimit).
        Build()
}

// Handler can use either style:
messages, err := prompts.BuildMessages(gs, scenario, msg, role, PromptHistoryLimit)
// OR
messages, err := prompts.New().WithGameState(gs).WithScenario(scenario).WithUserMessage(msg, role).Build()
```


**Step 1.3: GameState cleanup**
- Keep these helper methods in GameState (with scenario parameter):
  - `GetStatePrompt(scenario)` - returns state as prompt
  - `GetContingencyPrompts(scenario)` - returns filtered prompts
  - `GetStoryEvents()` - returns queued events string
  - `LoadScene(scenario, sceneName)` - loads scene from scenario
- Remove:
  - `GetChatMessages()` - deprecated/removed (replaced by prompts.Builder)
  - Any direct I/O operations

### Phase 2: Update Handlers

**Step 2.1: Update chat handler**
```go
// internal/handlers/chat.go

func (h *ChatHandler) handleChat(w http.ResponseWriter, r *http.Request) {
    // Load gamestate from storage (contains embedded narrator and PC)
    gs, err := h.storage.LoadGameState(r.Context(), sessionID)
    
    // Load scenario from filesystem on each request
    // TODO: Add caching layer to reduce filesystem I/O
    scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)
    
    // Build messages with prompt builder
    messages, err := prompts.New().
        WithGameState(gs).                      // Narrator and PC embedded
        WithScenario(scenario).                 // Loaded on each request
        WithUserMessage(cmdResult.Message, cmdResult.Role).
        WithHistoryLimit(PromptHistoryLimit).
        Build()
    
    // Call LLM
    response, err := h.llm.Chat(r.Context(), messages)
}
```

**Step 2.2: Update gamestate creation handler**
```go
// internal/handlers/gamestate.go

func (h *GameStateHandler) CreateGameState(w http.ResponseWriter, r *http.Request) {
    // Load scenario from storage (used for initialization, not embedded)
    scenario, err := h.storage.GetScenario(r.Context(), req.Scenario)
    
    // Load narrator from storage (ONCE at creation - then embedded)
    var narrator *scenario.Narrator
    narratorID := req.NarratorID
    if narratorID == "" {
        narratorID = scenario.NarratorID
    }
    if narratorID != "" {
        narrator, err = h.storage.GetNarrator(r.Context(), narratorID)
    }
    
    // Load PC spec and build PC (ONCE at creation - then embedded)
    pcSpec, err := h.storage.GetPCSpec(r.Context(), pcID)
    pc, err := actor.NewPCFromSpec(pcSpec)
    
    // Create gamestate with scenario filename and embedded narrator
    gs := state.NewGameState(req.Scenario, narrator, h.modelName)
    gs.PC = pc
    gs.NPCs = scenario.NPCs
    gs.Location = scenario.OpeningLocation
    gs.WorldLocations = scenario.Locations
    gs.Vars = scenario.Vars
    
    // Save to storage - narrator and PC are embedded, scenario is filename
    h.storage.SaveGameState(r.Context(), gs.ID, gs)
}
```

**Step 2.3: Verify storage access patterns**
- ✅ Chat handlers: 1 Redis read (gamestate) + 1 filesystem read (scenario)
- ✅ No narrator lookups during chat (embedded in gamestate)
- ✅ PC is embedded in gamestate (no separate load)
- 🎯 TODO: Add caching layer for scenario to reduce to 1 Redis read only

### Phase 3: Testing & Validation

**Step 3.1: Unit tests for prompt builder**
- Test system prompt construction with narrator from gamestate, scenario from parameter
- Test history windowing
- Test story event injection
- Test game-end prompts
- Test with/without narrator
- Test that builder correctly uses gs.Narrator (embedded) and scenario (parameter)

**Step 3.2: Integration tests**
- Verify handlers load gamestate + scenario (narrator embedded, no separate load)
- Verify Anthropic still works (splitChatMessages)
- Verify Venice still works
- Verify Ollama still works
- Test that narrator remains stable for running games
- Test that scenario updates propagate to running games
- Test gamestate serialization with embedded narrator

**Step 3.3: Verify no regressions**
- Run full test suite
- Manual testing of chat flows
- Verify Redis storage size acceptable
- Add TODO comments for scenario caching optimization

## Key Architectural Improvements

### 1. Clean Storage Layer (✅ COMPLETED)
- ✅ **Unified storage interface**: All storage operations through `internal/storage.Storage`
- ✅ **Resource-specific files**: scenario.go, narrator.go, pc.go, gamestate.go
- ✅ **No I/O in domain packages**: `pkg/*` packages are pure domain logic
- ✅ **Simplified PC construction**: Storage returns `PCSpec`, domain layer builds `PC` with `NewPCFromSpec`

### 2. Partially Self-Contained GameState (🎯 TO BE COMPLETED - Phase 0)
- 🎯 **Embed narrator and PC**: `Narrator *scenario.Narrator`, `PC *actor.PC` (PC may already be embedded)
- 🎯 **Keep scenario as filename**: `Scenario string` - load on each request for flexibility
- 🎯 **Load narrator once at creation**: Storage I/O only when creating gamestate
- 🎯 **Load scenario per request**: Chat requests load scenario from filesystem (TODO: add caching)
- 🎯 **Simplified methods**: Keep scenario parameter in `GetStatePrompt(scenario)`, `GetContingencyPrompts(scenario)`, `LoadScene(scenario, sceneName)`
- 🎯 **Stable narrator/PC**: Running games don't see narrator changes (correct behavior)
- 🎯 **Flexible scenarios**: Running games see scenario updates (allows story content evolution)

### 3. Prompt Builder Benefits (🎯 TO BE COMPLETED - Phase 1)
- ✅ **Clean separation**: Prompt building logic separated from GameState
- ✅ **Testability**: Easy to test prompt construction in isolation
- ✅ **Reusability**: Multiple handlers can use same builder
- ✅ **Maintainability**: All prompt logic in one place (`pkg/prompts`)
- ✅ **Reduced I/O**: Narrator embedded in gamestate, only scenario loaded per request
- 🎯 **Future optimization**: Easy to add scenario caching layer

### 4. Clean Package Structure (After refactor)
- `pkg/state`: Pure state management, embedded narrator and PC
- `pkg/prompts`: Pure transformation logic (gamestate + scenario → messages)
- `internal/storage`: I/O operations only (loads resources from filesystem/Redis)
- `internal/handlers`: Orchestration (load gamestate + scenario → build prompts → call LLM)

### 5. Performance Benefits (🎯 TO BE ACHIEVED)
- **Before**: Every chat = 2 file reads (scenario + narrator) + 1 Redis read (gamestate)
- **After Phase 0**: Every chat = 1 file read (scenario) + 1 Redis read (gamestate with narrator)
- **After caching (future)**: Every chat = 1 Redis read (gamestate) + cached scenario lookup
- **Reduced latency** by eliminating narrator filesystem I/O from hot path
- **TODO**: Add scenario caching layer for further optimization

---

## Benefits After Refactor

### GameState (pkg/state/)
```go
// Clean, focused interface with embedded narrator and PC
type GameState struct {
    Scenario  string                // Filename - loaded per request
    Narrator  *scenario.Narrator    // Embedded - loaded once at creation
    PC        *actor.PC             // Embedded - loaded once at creation
    // ... other fields ...
}

// Data extraction helpers (scenario passed as parameter)
func (gs *GameState) GetStatePrompt(scenario *scenario.Scenario) (chat.ChatMessage, error)
func (gs *GameState) GetContingencyPrompts(scenario *scenario.Scenario) []string
func (gs *GameState) GetStoryEvents() string
func (gs *GameState) LoadScene(scenario *scenario.Scenario, sceneName string) error
```

### Prompt Builder (pkg/prompts/)
```go
// Fluent builder pattern for LLM messages
type Builder struct {
    gs           *state.GameState
    scenario     *scenario.Scenario
    userMessage  string
    userRole     string
    historyLimit int
}

func New() *Builder
func (b *Builder) WithGameState(gs *state.GameState) *Builder
func (b *Builder) WithScenario(scenario *scenario.Scenario) *Builder
func (b *Builder) WithUserMessage(message, role string) *Builder
func (b *Builder) WithHistoryLimit(limit int) *Builder
func (b *Builder) Build() ([]chat.ChatMessage, error)

// Convenience function for simple cases
func BuildMessages(gs *state.GameState, scenario *scenario.Scenario, message, role string, limit int) ([]chat.ChatMessage, error)
```

### Storage Layer (internal/storage/) - ✅ COMPLETED
```go
// Unified interface for all storage operations
type Storage interface {
    // Health and lifecycle
    Ping(ctx context.Context) error
    Close() error

    // GameState operations (Redis)
    SaveGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error
    LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error)
    DeleteGameState(ctx context.Context, id uuid.UUID) error

    // Resource operations (Filesystem)
    ListScenarios(ctx context.Context) (map[string]string, error)
    GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error)  // Loaded per request
    GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error)  // Loaded at creation only
    ListNarrators(ctx context.Context) ([]string, error)
    GetPCSpec(ctx context.Context, pcID string) (*actor.PCSpec, error)  // Loaded at creation only
    ListPCs(ctx context.Context) ([]string, error)
}
```

### LLM Services (internal/services/) - Unchanged
```go
// Provider-agnostic interface (unchanged)
type LLMService interface {
    Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
}

// Providers handle their own transformations
// Anthropic: splitChatMessages for system prompt extraction
// Venice: pass-through
// Ollama: pass-through
```

### Handlers (internal/handlers/)
```go
// Chat handler: Minimal storage lookups
func (h *ChatHandler) handleChat(w, r) {
    // Load gamestate from Redis (contains embedded narrator and PC)
    gs := h.storage.LoadGameState(ctx, sessionID)
    
    // Load scenario from filesystem
    // TODO: Add caching layer to optimize this
    scenario := h.storage.GetScenario(ctx, gs.Scenario)
    
    // Fluent builder - simple and clean
    messages, err := prompts.New().
        WithGameState(gs).          // Narrator and PC embedded
        WithScenario(scenario).     // Loaded per request
        WithUserMessage(msg, role).
        WithHistoryLimit(PromptHistoryLimit).
        Build()
    
    response := h.llm.Chat(ctx, messages)
}

// Create gamestate: Load narrator and PC once, keep scenario as filename
func (h *GameStateHandler) createGameState(w, r) {
    scenario := h.storage.GetScenario(ctx, scenarioFile)    // For initialization
    narrator := h.storage.GetNarrator(ctx, narratorID)      // Load once - embed
    pcSpec := h.storage.GetPCSpec(ctx, pcID)                // Load once - build and embed
    pc := actor.NewPCFromSpec(pcSpec)
    
    // Create gamestate with filename and embedded narrator
    gs := state.NewGameState(scenarioFile, narrator, modelName)
    gs.PC = pc
    gs.NPCs = scenario.NPCs
    gs.Location = scenario.OpeningLocation
    gs.WorldLocations = scenario.Locations
    gs.Vars = scenario.Vars
    
    // Save to storage - narrator and PC embedded, scenario as filename
    h.storage.SaveGameState(ctx, gs.ID, gs)
}
```

## Success Criteria

### ✅ Completed (from storage refactor)
- ✅ Clean storage layer in `internal/storage` package
- ✅ Domain packages (`pkg/*`) have no file I/O
- ✅ Storage returns `PCSpec`, domain builds `PC` with constructor
- ✅ Unified `Storage` interface for all operations

### 🎯 To be completed (this refactor)

**Phase 0 - GameState Embedding:**
- [ ] GameState embeds `*scenario.Narrator` (full object)
- [ ] GameState keeps `Scenario` as string (filename)
- [ ] GameState embeds `*actor.PC` (may already be done)
- [ ] `NewGameState()` accepts scenario filename and narrator object
- [ ] `GetStatePrompt(scenario)` keeps scenario parameter
- [ ] `GetContingencyPrompts(scenario)` keeps scenario parameter
- [ ] `LoadScene(scenario, sceneName)` keeps scenario parameter
- [ ] Gamestate creation handler loads and embeds narrator
- [ ] Chat handlers load scenario on each request (add TODO for caching)
- [ ] Chat handlers have no narrator lookups (embedded in gamestate)

**Phase 1 - Prompt Builder:**
- [ ] `GameState.GetChatMessages()` removed (moved to prompt builder)
- [ ] `GameState` has no formatting logic
- [ ] Prompt building is in `pkg/prompts` package
- [ ] Prompt building is tested in isolation
- [ ] Builder receives scenario as parameter, gets narrator from embedded GameState

**Phase 2 - Handler Updates:**
- [ ] Chat handlers load gamestate + scenario (no narrator lookup)
- [ ] Gamestate creation loads narrator once and embeds it
- [ ] Gamestate creation keeps scenario as filename reference
- [ ] All handler tests updated
- [ ] TODO comments added for scenario caching optimization

**Phase 3 - Validation:**
- [ ] LLM services remain provider-agnostic
- [ ] All existing tests pass
- [ ] Anthropic system message handling still works
- [ ] Narrator remains stable for running games (doesn't change mid-game)
- [ ] Scenarios can be updated and games see changes (desired behavior)
- [ ] Redis storage size acceptable
- [ ] Code is easier to understand and modify

## Future Enhancements

Once the fluent prompt builder is in place, we can easily add:

1. **Additional Builder Methods**:
   ```go
   .WithSystemPromptOverride(prompt string)  // Custom system prompt
   .WithoutNarrator()                         // Skip narrator prompts
   .WithExtraContext(context string)          // Add custom context
   .WithTokenLimit(limit int)                 // Estimate and trim to fit
   ```

2. **Prompt Templates**: Externalize prompt text to config files
3. **Prompt Versioning**: A/B test different prompt strategies
4. **Token Counting**: Estimate token usage before calling LLM
5. **Prompt Caching**: Cache expensive prompt construction
6. **Debug Mode**: 
   ```go
   .WithDebugLogging(logger)  // Log full prompts for debugging
   ```
7. **Provider-Specific Optimization**: Different builders for different providers (if needed)

## Conclusion

**Recommendation: Option 3 - Dedicated Prompt Builder**

This gives us:
- Clear separation between state management and prompt formatting
- Easy testing without mocks or I/O
- LLM providers stay simple and provider-agnostic
- GameState becomes pure state management
- Handlers orchestrate without complex logic
- Provider-specific quirks (Anthropic) handled in the right layer

The key insight is that **prompt building is presentation logic**, not state logic or LLM logic. It deserves its own layer.
