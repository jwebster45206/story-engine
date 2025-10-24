# Prompt Builder Refactor Plan

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

## Architectural Options

### Option 1: Keep in Handler (Simple Cross-Platform)
**Philosophy**: If prompt building is simple and consistent, handlers orchestrate it.

```go
// internal/handlers/chat.go
func (h *ChatHandler) buildChatMessages(gs *GameState, scenario *Scenario, 
    narrator *Narrator, request string, role string) ([]chat.ChatMessage, error) {
    
    // Build system prompt
    systemPrompt := scenario.BuildSystemPrompt(narrator, gs.PC)
    systemPrompt += gs.GetStatePrompt(scenario)
    systemPrompt += gs.GetContingencyPrompts(scenario)
    
    // Assemble messages
    messages := []chat.ChatMessage{
        {Role: chat.ChatRoleSystem, Content: systemPrompt},
    }
    messages = append(messages, gs.GetRecentHistory(PromptHistoryLimit)...)
    messages = append(messages, chat.ChatMessage{Role: role, Content: request})
    
    return messages, nil
}
```

**Pros:**
- Simple and direct
- Handler already orchestrates other concerns
- Easy to understand data flow
- No new abstractions

**Cons:**
- Handler gets larger
- Duplicated if multiple handlers build prompts
- Harder to test prompt logic in isolation

---

### Option 2: Move to LLM Layer (Provider-Specific)
**Philosophy**: LLM providers know how to format their own prompts.

```go
// internal/services/llm.go
type LLMService interface {
    // New method: takes gamestate + scenario, builds messages internally
    ChatWithGameState(ctx context.Context, gs *state.GameState, 
        scenario *scenario.Scenario, request string) (*chat.ChatResponse, error)
    
    // Existing method still available for flexibility
    Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
}

// internal/services/anthropic.go
func (a *AnthropicService) ChatWithGameState(ctx, gs, scenario, request) {
    // Build messages
    systemPrompt := a.buildSystemPrompt(gs, scenario)
    messages := a.buildMessages(gs, request)
    
    // Anthropic-specific transformation
    systemPrompt, conversationMessages := a.splitChatMessages(messages)
    
    // Call API
    return a.chatCompletion(ctx, conversationMessages, systemPrompt)
}
```

**Pros:**
- Each provider can customize message building
- Encapsulates provider-specific quirks (Anthropic's system message handling)
- Clean separation: LLM layer owns prompt formatting

**Cons:**
- ❌ **Tight coupling**: LLM services now depend on domain models (GameState, Scenario)
- ❌ **Violates dependency inversion**: Services shouldn't know about domain
- ❌ **Wrong abstraction level**: LLMs should receive messages, not understand game concepts
- Too much responsibility in LLM layer

---

### Option 3: Dedicated Prompt Builder (Recommended)
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

---

### Option 4: Helper in GameState Package
**Philosophy**: Prompt building is stateless logic related to GameState.

```go
// pkg/state/prompts.go (new file in existing package)
package state

// BuildChatMessages is a helper that constructs LLM messages from gamestate
func BuildChatMessages(
    gs *GameState,
    scenario *scenario.Scenario,
    narrator *scenario.Narrator,
    userMessage string,
    userRole string,
    historyLimit int,
) ([]chat.ChatMessage, error) {
    // ... implementation similar to Option 3 ...
}
```

**Pros:**
- No new package
- Close to the data it operates on
- Simple to discover

**Cons:**
- ❌ Still couples GameState package to prompt formatting concerns
- ❌ Less clear than a dedicated package
- ❌ GameState package grows larger

---

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

## Embedding Scenario and Narrator in GameState

### Current Problem

Every chat request requires loading:
```go
scenario := storage.GetScenario(gs.Scenario)  // Loads from disk every time!
narrator := storage.GetNarrator(gs.NarratorID) // Loads from disk every time!
```

This is inefficient and couples chat handlers to storage unnecessarily.

### Solution: Embed Full Objects in GameState

**Change GameState to store full objects instead of just IDs:**

```go
// pkg/state/gamestate.go
type GameState struct {
    ID               uuid.UUID                    `json:"id"`
    ModelName        string                       `json:"model_name,omitempty"`
    
    // BEFORE: Just filenames/IDs
    // Scenario         string                       `json:"scenario,omitempty"`
    // NarratorID       string                       `json:"narrator_id,omitempty"`
    
    // AFTER: Full embedded objects
    Scenario         *scenario.Scenario           `json:"scenario,omitempty"`
    Narrator         *scenario.Narrator           `json:"narrator,omitempty"`
    
    SceneName        string                       `json:"scene_name,omitempty"`
    PC               *actor.PC                    `json:"pc,omitempty"`
    // ... rest of fields ...
}
```

**Benefits:**
1. ✅ **Load once at creation time** - scenario and narrator loaded when gamestate is created
2. ✅ **No storage lookups during chat** - everything needed is in memory
3. ✅ **Faster chat responses** - no I/O on hot path
4. ✅ **Self-contained gamestate** - everything you need is in one object
5. ✅ **Simpler handlers** - no need to coordinate multiple storage calls

**Tradeoffs:**
- ⚠️ Larger gamestate in Redis (but still small, ~few KB)
- ⚠️ Scenario updates don't propagate to running games (this is actually desirable - games should be immutable)

---

## Recommendation: Option 3 (Dedicated Prompt Builder)

### Why This is Best

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

### Phase 0: Embed Scenario and Narrator
- [ ] Update `GameState` struct to store `*scenario.Scenario` and `*scenario.Narrator`
- [ ] Update `NewGameState()` constructor signature
- [ ] Update `GetStatePrompt()` to use embedded scenario (remove parameter)
- [ ] Update `GetContingencyPrompts()` to use embedded scenario (remove parameter)
- [ ] Update `LoadScene()` to use embedded scenario (remove parameter)
- [ ] Update gamestate creation handler to load and embed scenario/narrator
- [ ] Update all handler calls that pass scenario as parameter
- [ ] Remove scenario/narrator storage lookups from chat handlers
- [ ] Test gamestate serialization/deserialization with embedded objects
- [ ] Verify Redis storage size is acceptable

### Phase 1: Create Prompt Builder
- [ ] Create `pkg/prompts` package
- [ ] Implement `Builder` with `BuildChatMessages()`
- [ ] Implement `buildSystemPrompt()` helper
- [ ] Implement `buildFinalPrompt()` helper
- [ ] Implement `windowHistory()` helper
- [ ] Add comprehensive unit tests

### Phase 2: Update Handlers
- [ ] Update chat handler to use `prompts.Builder`
- [ ] Remove `GetChatMessages()` from GameState
- [ ] Remove scenario/narrator parameters from handler methods
- [ ] Verify all storage lookups removed from chat flow
- [ ] Update handler tests

### Phase 3: Testing & Validation
- [ ] Run all unit tests
- [ ] Run integration tests
- [ ] Test with Anthropic (verify splitChatMessages still works)
- [ ] Test with Venice
- [ ] Test with Ollama
- [ ] Performance testing (no I/O in hot path)
- [ ] Verify gamestate save/load works correctly

---

## Migration Plan

### Phase 0: Embed Scenario and Narrator in GameState

**Step 0.1: Update GameState structure**

```go
// pkg/state/gamestate.go
type GameState struct {
    ID               uuid.UUID                    `json:"id"`
    ModelName        string                       `json:"model_name,omitempty"`
    
    // Full objects instead of IDs
    Scenario         *scenario.Scenario           `json:"scenario,omitempty"`
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
    
    // Load scenario from storage
    scenario, err := h.storage.GetScenario(r.Context(), req.Scenario)
    if err != nil {
        // handle error
    }
    
    // Load narrator from storage (if specified)
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
    
    // Load PC spec from storage
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
    
    // Create gamestate with embedded objects
    gs := state.NewGameState(scenario, narrator, h.modelName)
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

func NewGameState(scenario *scenario.Scenario, narrator *scenario.Narrator, modelName string) *GameState {
    return &GameState{
        ID:                 uuid.New(),
        ModelName:          modelName,
        Scenario:           scenario,  // Store full object
        Narrator:           narrator,  // Store full object
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

**Step 0.4: Update helper methods**

```go
// pkg/state/gamestate.go

// GetStatePrompt now uses embedded scenario
func (gs *GameState) GetStatePrompt() (chat.ChatMessage, error) {
    if gs == nil || gs.Scenario == nil {
        return chat.ChatMessage{}, fmt.Errorf("game state or scenario is nil")
    }

    var scene *scenario.Scene
    if gs.SceneName != "" {
        sc, ok := gs.Scenario.Scenes[gs.SceneName]
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

    story := gs.Scenario.Story
    if scene != nil && scene.Story != "" {
        story += "\n\n" + scene.Story
    }
    
    return chat.ChatMessage{
        Role:    chat.ChatRoleSystem,
        Content: fmt.Sprintf(scenario.StatePromptTemplate, story, jsonScene),
    }, nil
}

// GetContingencyPrompts now uses embedded scenario
func (gs *GameState) GetContingencyPrompts() []string {
    if gs == nil || gs.Scenario == nil {
        return nil
    }

    var prompts []string

    // Filter scenario-level contingency prompts
    scenarioPrompts := scenario.FilterContingencyPrompts(gs.Scenario.ContingencyPrompts, gs)
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
        if scene, ok := gs.Scenario.Scenes[gs.SceneName]; ok {
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

// LoadScene now uses embedded scenario
func (gs *GameState) LoadScene(sceneName string) error {
    if gs.Scenario == nil {
        return fmt.Errorf("scenario is nil")
    }
    
    scene, ok := gs.Scenario.Scenes[sceneName]
    if !ok {
        return fmt.Errorf("scene %s not found", sceneName)
    }
    
    gs.SceneName = sceneName
    gs.SceneTurnCounter = 0

    // ... rest of scene loading logic ...
}
```

**Step 0.5: Update chat handlers**

```go
// internal/handlers/chat.go

func (h *ChatHandler) handleChat(w http.ResponseWriter, r *http.Request) {
    // ... load gamestate ...
    
    // BEFORE: Load scenario and narrator from storage
    // scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)
    // narrator, err := h.storage.GetNarrator(r.Context(), gs.NarratorID)
    
    // AFTER: Use embedded objects
    // No storage calls needed! Everything is in gamestate
    
    // Build messages (Phase 1 will introduce prompt builder)
    messages, err := gs.GetChatMessages(...)
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
- Use embedded scenario and narrator from GameState
- Add comprehensive tests

```go
// pkg/prompts/builder.go
package prompts

// Fluent builder pattern for constructing chat messages
type Builder struct {
    gs           *state.GameState
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

// WithGameState sets the gamestate (contains scenario, narrator, etc.)
func (b *Builder) WithGameState(gs *state.GameState) *Builder {
    b.gs = gs
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
    if b.gs.Scenario == nil {
        return nil, fmt.Errorf("scenario is nil in gamestate")
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
// - addSystemPrompt(): builds from narrator, rating, state, contingency prompts
// - addHistory(): windows chat history to limit
// - addUserMessage(): adds the current user message
// - addStoryEvents(): adds queued story events if present
// - addFinalPrompt(): adds game-end or standard reminders
```

**Handler Usage (Fluent Style):**
```go
// internal/handlers/chat.go
messages, err := prompts.New().
    WithGameState(gs).
    WithUserMessage(cmdResult.Message, cmdResult.Role).
    WithHistoryLimit(PromptHistoryLimit).
    Build()
```

**Alternative: Keep Simple Constructor for Common Case:**
```go
// pkg/prompts/builder.go

// BuildMessages is a convenience function for the common case
func BuildMessages(gs *state.GameState, message string, role string, historyLimit int) ([]chat.ChatMessage, error) {
    return New().
        WithGameState(gs).
        WithUserMessage(message, role).
        WithHistoryLimit(historyLimit).
        Build()
}

// Handler can use either style:
messages, err := prompts.BuildMessages(gs, msg, role, PromptHistoryLimit)
// OR
messages, err := prompts.New().WithGameState(gs).WithUserMessage(msg, role).Build()
```


**Step 1.3: GameState cleanup**
- Keep these helper methods in GameState:
  - `GetStatePrompt()` - returns state as prompt (uses embedded scenario)
  - `GetContingencyPrompts()` - returns filtered prompts (uses embedded scenario)
  - `GetStoryEvents()` - returns queued events string
  - `LoadScene(sceneName)` - uses embedded scenario
- Remove:
  - `GetChatMessages()` - deprecated/removed (replaced by prompts.Builder)
  - Any parameters that were `*scenario.Scenario` (now uses embedded)
  - Any direct I/O operations

### Phase 2: Update Handlers

**Step 2.1: Update chat handler**
```go
// internal/handlers/chat.go

// Before:
messages, err := gs.GetChatMessages(cmdResult.Message, cmdResult.Role, scenario, 
    PromptHistoryLimit, storyEventPrompt)

// After (Fluent Style):
messages, err := prompts.New().
    WithGameState(gs).
    WithUserMessage(cmdResult.Message, cmdResult.Role).
    WithHistoryLimit(PromptHistoryLimit).
    Build()

// Or (Simple Function Style):
messages, err := prompts.BuildMessages(gs, cmdResult.Message, cmdResult.Role, PromptHistoryLimit)
```

**Step 2.2: Verify no scenario/narrator lookups in chat flow**
```go
// internal/handlers/chat.go

// REMOVE these lines:
// scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)
// narrator, err := h.storage.GetNarrator(r.Context(), gs.NarratorID)

// Everything comes from gamestate now!
```

### Phase 3: Testing & Validation

**Step 3.1: Unit tests for prompt builder**
- Test system prompt construction
- Test history windowing
- Test story event injection
- Test game-end prompts
- Test with/without narrator

**Step 3.2: Integration tests**
- Verify Anthropic still works (splitChatMessages)
- Verify Venice still works
- Verify Ollama still works

**Step 3.3: Verify no regressions**
- Run full test suite
- Manual testing of chat flows

## Key Architectural Improvements

### 1. Self-Contained GameState
- ✅ **Everything in one place**: Scenario, Narrator, PC, NPCs, Locations all in GameState
- ✅ **Load once at creation**: Storage I/O only when creating/loading gamestate
- ✅ **No hot-path I/O**: Chat requests have zero storage lookups
- ✅ **Immutable games**: Running games don't see scenario changes (correct behavior)
- ✅ **Easier testing**: Mock storage only for create/load, not every chat

### 2. Clean Package Structure
- `pkg/state`: Pure state management, embedded dependencies
- `pkg/prompts`: Pure transformation logic, no I/O
- `internal/storage`: I/O operations only
- `internal/handlers`: Orchestration only

### 3. Performance Benefits
- **Before**: Every chat = 2 file reads (scenario + narrator)
- **After**: Every chat = 0 file reads (everything in memory)
- **Faster response times** especially important for streaming

---

## Benefits After Refactor

### GameState (pkg/state/)
```go
// Clean, focused interface with embedded dependencies
type GameState struct {
    Scenario  *scenario.Scenario  // Embedded - loaded once at creation
    Narrator  *scenario.Narrator  // Embedded - loaded once at creation
    // ... other fields ...
}

// Data extraction helpers (no I/O, uses embedded objects)
func (gs *GameState) GetStatePrompt() (chat.ChatMessage, error)
func (gs *GameState) GetContingencyPrompts() []string
func (gs *GameState) GetStoryEvents() string
func (gs *GameState) LoadScene(sceneName string) error
```

### Prompt Builder (pkg/prompts/)
```go
// Fluent builder pattern for LLM messages
type Builder struct {
    gs           *state.GameState
    userMessage  string
    userRole     string
    historyLimit int
}

func New() *Builder
func (b *Builder) WithGameState(gs *state.GameState) *Builder
func (b *Builder) WithUserMessage(message, role string) *Builder
func (b *Builder) WithHistoryLimit(limit int) *Builder
func (b *Builder) Build() ([]chat.ChatMessage, error)

// Convenience function for simple cases
func BuildMessages(gs *state.GameState, message, role string, limit int) ([]chat.ChatMessage, error)
```

### LLM Services (internal/services/)
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
// Orchestrates: storage → prompt building → LLM
// SIMPLER: No scenario/narrator lookups on hot path
func (h *ChatHandler) handleChat(w, r) {
    gs := loadGameState()  // Contains embedded scenario and narrator
    
    // Fluent builder - readable and flexible
    messages, err := prompts.New().
        WithGameState(gs).
        WithUserMessage(msg, role).
        WithHistoryLimit(PromptHistoryLimit).
        Build()
    
    response := llm.Chat(ctx, messages)
}

// Create gamestate - load dependencies once
func (h *GameStateHandler) createGameState(w, r) {
    scenario := loadScenario()    // Load from storage
    narrator := loadNarrator()    // Load from storage
    pc := loadAndBuildPC()        // Load and construct
    
    // Create gamestate with everything embedded
    gs := state.NewGameState(scenario, narrator, modelName)
    gs.PC = pc
    // ... initialize other fields ...
    
    // Save to storage - scenario and narrator are now part of gamestate
    storage.SaveGameState(ctx, gs.ID, gs)
}
```

## Decision Matrix

| Criteria | Option 1 (Handler) | Option 2 (LLM Layer) | **Option 3 (Builder)** | Option 4 (Helper) |
|----------|-------------------|---------------------|----------------------|------------------|
| Separation of Concerns | ⚠️ Mixed | ❌ Violates | ✅ Clean | ⚠️ Mixed |
| Testability | ⚠️ Needs mocks | ❌ Complex | ✅ Easy | ⚠️ Needs mocks |
| Reusability | ❌ Duplicated | ✅ Reusable | ✅ Reusable | ✅ Reusable |
| Maintainability | ⚠️ Scattered | ❌ Wrong layer | ✅ Focused | ⚠️ Growing pkg |
| LLM Independence | ✅ Yes | ❌ Coupled | ✅ Yes | ✅ Yes |
| GameState Purity | ⚠️ Still complex | ⚠️ Still complex | ✅ Clean | ❌ Still complex |
| Complexity | ✅ Simple | ❌ Complex | ⚠️ New abstraction | ✅ Simple |

## Success Criteria

- ✅ `GameState` embeds Scenario and Narrator (loaded once at creation)
- ✅ `GameState` has no I/O operations in helper methods
- ✅ `GameState` has no formatting logic
- ✅ Chat handlers have zero storage lookups (everything from gamestate)
- ✅ Prompt building is in `pkg/prompts` package
- ✅ Prompt building is tested in isolation
- ✅ LLM services remain provider-agnostic
- ✅ All existing tests pass
- ✅ Anthropic system message handling still works
- ✅ Code is easier to understand and modify
- ✅ Performance improved (no I/O on hot path)

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
