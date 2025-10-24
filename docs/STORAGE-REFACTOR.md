# Storage Layer Refactor Plan

## Current State

### Problems
1. **Storage logic scattered across packages**: File loading functions (`LoadNarrator`, `LoadPC`) exist in domain packages (`pkg/scenario`, `pkg/actor`) rather than a dedicated storage layer
2. **Mixed responsibilities**: The `internal/services` package combines LLM services with storage interfaces
3. **Tight coupling**: Domain objects like `PC` are tightly coupled with their loading logic
4. **Complex constructors**: `LoadPC` does both data loading AND domain object construction (building `d20.Actor`)
5. **TODO markers**: Code has `// TODO: Move to storage layer` comments indicating known tech debt

### Current Structure
```
internal/services/
  ├── storage.go          # Storage interface definition
  ├── redis.go            # Redis implementation of Storage
  ├── mock_storage.go     # Mock for testing
  ├── llm.go             # LLM interface (mixed concern)
  ├── anthropic.go       # LLM implementations
  └── venice.go          # LLM implementations

pkg/scenario/
  └── narrator.go        # Contains LoadNarrator (file I/O logic)

pkg/actor/
  └── pc.go              # Contains LoadPC (file I/O + construction logic)
```

## Proposed Architecture

### Goals
1. **Clean separation**: Storage, domain logic, and handlers each in their own layer
2. **Resource-based organization**: One storage file per resource type
3. **Simplified constructors**: Storage returns specs/DTOs, constructors build domain objects
4. **Testability**: Easy to mock storage without complex dependencies
5. **Single Responsibility**: Each package has one clear purpose

### New Structure
```
internal/
  ├── storage/
  │   ├── storage.go         # Core interfaces and types
  │   ├── gamestate.go       # GameState storage operations
  │   ├── scenario.go        # Scenario storage operations
  │   ├── narrator.go        # Narrator storage operations
  │   ├── pc.go              # PC storage operations (PCSpec only)
  │   ├── redis.go           # Redis implementation
  │   └── mock.go            # Mock storage for testing
  │
  ├── services/
  │   ├── llm.go             # LLM interface
  │   ├── anthropic.go       # Anthropic implementation
  │   ├── venice.go          # Venice implementation
  │   ├── ollama.go          # Ollama implementation
  │   └── mock_llm.go        # Mock LLM for testing
  │
  └── handlers/
      ├── gamestate.go       # Uses storage layer
      ├── chat.go            # Uses storage layer
      └── scenario.go        # Uses storage layer

pkg/
  ├── scenario/
  │   ├── scenario.go        # Domain model (no I/O)
  │   ├── narrator.go        # Domain model (no I/O)
  │   └── scene.go           # Domain model (no I/O)
  │
  ├── actor/
  │   └── pc.go              # Domain model + constructor from PCSpec
  │
  └── state/
      └── gamestate.go       # Domain model (no I/O)
```

## Detailed Migration Plan

### Phase 1: Create Storage Package Structure

**Step 1.1: Create base storage package**
- Create `internal/storage/storage.go` with unified storage interface

```go
// internal/storage/storage.go
package storage

import (
    "context"
    "github.com/google/uuid"
    "github.com/jwebster45206/story-engine/pkg/state"
    "github.com/jwebster45206/story-engine/pkg/scenario"
    "github.com/jwebster45206/story-engine/pkg/actor"
)

// Storage defines a unified interface for all storage operations
type Storage interface {
    // Health and lifecycle
    Ping(ctx context.Context) error
    Close() error

    // GameState operations
    SaveGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error
    LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error)
    DeleteGameState(ctx context.Context, id uuid.UUID) error

    // Scenario operations
    ListScenarios(ctx context.Context) (map[string]string, error)
    GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error)

    // Narrator operations
    GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error)
    ListNarrators(ctx context.Context) ([]string, error)

    // PC operations (returns PCSpec, not PC)
    GetPCSpec(ctx context.Context, path string) (*actor.PCSpec, error)
    ListPCs(ctx context.Context) ([]string, error)
}
```

### Phase 2: Migrate Resource-Specific Storage

**Step 2.1: Create `internal/storage/scenario.go`**
- Extract scenario operations from `redis.go`
- Move filesystem walking logic here

```go
// internal/storage/scenario.go
package storage

import (
    "context"
    "encoding/json"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "github.com/jwebster45206/story-engine/pkg/scenario"
)

// ScenarioFilesystemLoader loads scenarios from the filesystem
type ScenarioFilesystemLoader struct {
    dataDir string
}

func NewScenarioFilesystemLoader(dataDir string) *ScenarioFilesystemLoader {
    return &ScenarioFilesystemLoader{dataDir: dataDir}
}

func (s *ScenarioFilesystemLoader) ListScenarios(ctx context.Context) (map[string]string, error) {
    scenariosDir := filepath.Join(s.dataDir, "scenarios")
    scenarios := make(map[string]string)

    err := filepath.WalkDir(scenariosDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
            return nil
        }

        file, err := os.ReadFile(path)
        if err != nil {
            return nil // Skip files we can't read
        }

        var s scenario.Scenario
        if err := json.Unmarshal(file, &s); err != nil {
            return nil // Skip invalid JSON
        }

        filename := filepath.Base(path)
        scenarios[s.Name] = filename
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to list scenarios: %w", err)
    }

    return scenarios, nil
}

func (s *ScenarioFilesystemLoader) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
    path := filepath.Join(s.dataDir, "scenarios", filename)
    
    file, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("scenario not found: %s", filename)
        }
        return nil, fmt.Errorf("failed to read scenario file: %w", err)
    }

    var sc scenario.Scenario
    if err := json.Unmarshal(file, &sc); err != nil {
        return nil, fmt.Errorf("failed to unmarshal scenario: %w", err)
    }

    return &sc, nil
}
```

**Step 2.2: Create `internal/storage/narrator.go`**
- Move `LoadNarrator` logic from `pkg/scenario/narrator.go`
- Keep domain model in pkg/scenario

```go
// internal/storage/narrator.go
package storage

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "github.com/jwebster45206/story-engine/pkg/scenario"
)

type NarratorFilesystemLoader struct {
    dataDir string
}

func NewNarratorFilesystemLoader(dataDir string) *NarratorFilesystemLoader {
    return &NarratorFilesystemLoader{dataDir: dataDir}
}

func (n *NarratorFilesystemLoader) GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error) {
    if narratorID == "" {
        return nil, nil // No narrator specified
    }

    narratorPath := filepath.Join(n.dataDir, "narrators", narratorID+".json")
    
    data, err := os.ReadFile(narratorPath)
    if err != nil {
        if os.IsNotExist(err) {
            absPath, _ := filepath.Abs(narratorPath)
            cwd, _ := os.Getwd()
            return nil, fmt.Errorf("narrator not found: %s (tried: %s, cwd: %s)", narratorID, absPath, cwd)
        }
        return nil, fmt.Errorf("failed to read narrator file %s: %w", narratorPath, err)
    }

    var narrator scenario.Narrator
    if err := json.Unmarshal(data, &narrator); err != nil {
        return nil, fmt.Errorf("failed to parse narrator JSON from %s: %w", narratorPath, err)
    }
    narrator.ID = narratorID

    return &narrator, nil
}

func (n *NarratorFilesystemLoader) ListNarrators(ctx context.Context) ([]string, error) {
    narratorsPath := filepath.Join(n.dataDir, "narrators")

    entries, err := os.ReadDir(narratorsPath)
    if err != nil {
        if os.IsNotExist(err) {
            return []string{}, nil
        }
        return nil, fmt.Errorf("failed to read narrators directory: %w", err)
    }

    var narratorIDs []string
    for _, entry := range entries {
        if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
            narratorID := entry.Name()[:len(entry.Name())-5]
            narratorIDs = append(narratorIDs, narratorID)
        }
    }

    return narratorIDs, nil
}
```

**Step 2.3: Create `internal/storage/pc.go`**
- Move file loading from `pkg/actor/pc.go`
- **KEY CHANGE**: Return `PCSpec` only, NOT fully constructed `PC`
- Let domain layer handle actor construction

```go
// internal/storage/pc.go
package storage

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "github.com/jwebster45206/story-engine/pkg/actor"
)

type PCFilesystemLoader struct {
    dataDir string
}

func NewPCFilesystemLoader(dataDir string) *PCFilesystemLoader {
    return &PCFilesystemLoader{dataDir: dataDir}
}

// GetPCSpec loads a PC spec from a JSON file
// Returns only the spec, NOT a fully constructed PC with d20.Actor
func (p *PCFilesystemLoader) GetPCSpec(ctx context.Context, path string) (*actor.PCSpec, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read PC file: %w", err)
    }

    var spec actor.PCSpec
    if err := json.Unmarshal(data, &spec); err != nil {
        return nil, fmt.Errorf("failed to unmarshal PC spec: %w", err)
    }

    // Filename overrides any ID in the JSON
    spec.ID = strings.TrimSuffix(filepath.Base(path), ".json")

    return &spec, nil
}

func (p *PCFilesystemLoader) ListPCs(ctx context.Context) ([]string, error) {
    pcsPath := filepath.Join(p.dataDir, "pcs")

    entries, err := os.ReadDir(pcsPath)
    if err != nil {
        if os.IsNotExist(err) {
            return []string{}, nil
        }
        return nil, fmt.Errorf("failed to read PCs directory: %w", err)
    }

    var pcIDs []string
    for _, entry := range entries {
        if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
            pcID := entry.Name()[:len(entry.Name())-5]
            pcIDs = append(pcIDs, pcID)
        }
    }

    return pcIDs, nil
}
```

### Phase 3: Update Domain Layer

**Step 3.1: Refactor `pkg/actor/pc.go`**
- Remove `LoadPC` function (or deprecate it)
- Add `NewPCFromSpec` constructor
- Keep existing marshaling/unmarshaling for runtime state

```go
// pkg/actor/pc.go

// NewPCFromSpec creates a PC from a PCSpec
// This is the new preferred way to construct PCs after loading from storage
func NewPCFromSpec(spec *PCSpec) (*PC, error) {
    if spec == nil {
        return nil, fmt.Errorf("spec cannot be nil")
    }

    pc := &PC{
        Spec: spec,
    }

    // Build d20.Actor from PCSpec
    allAttrs := spec.Stats.ToAttributes()
    maps.Copy(allAttrs, spec.Attributes)

    actor, err := d20.NewActor(spec.ID).
        WithHP(spec.MaxHP).
        WithAC(spec.AC).
        WithAttributes(allAttrs).
        WithCombatModifiers(spec.CombatModifiers).
        Build()
    if err != nil {
        return nil, fmt.Errorf("failed to build actor: %w", err)
    }

    // Set current HP if different from max
    if spec.HP != spec.MaxHP && spec.HP > 0 {
        if err := actor.SetHP(spec.HP); err != nil {
            return nil, fmt.Errorf("failed to set HP: %w", err)
        }
    }

    pc.Actor = actor
    return pc, nil
}

// LoadPC is DEPRECATED: Use storage.GetPCSpec + NewPCFromSpec instead
// Kept temporarily for backward compatibility
func LoadPC(path string) (*PC, error) {
    // Could delegate to new pattern or mark as deprecated
    // Implementation omitted for brevity
}
```

**Step 3.2: Clean up `pkg/scenario/narrator.go`**
- Remove `LoadNarrator` function
- Keep domain model (`Narrator` struct, `GetPromptsAsString`)
- Remove `ListNarrators` (moves to storage)

```go
// pkg/scenario/narrator.go
package scenario

// Narrator defines the voice and style of the game narrator
type Narrator struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description,omitempty"`
    Prompts     []string `json:"prompts"`
}

func (n *Narrator) GetPromptsAsString() string {
    if len(n.Prompts) == 0 {
        return ""
    }

    result := ""
    for _, prompt := range n.Prompts {
        result += "- " + prompt + "\n"
    }
    return result
}

// LoadNarrator REMOVED - use storage.GetNarrator instead
// ListNarrators REMOVED - use storage.ListNarrators instead
```

### Phase 4: Implement Redis Storage

**Step 4.1: Create `internal/storage/redis.go`**
- Move from `internal/services/redis.go`
- Implement unified Storage interface
- Inline resource loading methods (no separate loader types)

```go
// internal/storage/redis.go
package storage

import (
    "context"
    "encoding/json"
    "fmt"
    "io/fs"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
    "github.com/jwebster45206/story-engine/pkg/actor"
    "github.com/jwebster45206/story-engine/pkg/scenario"
    "github.com/jwebster45206/story-engine/pkg/state"
)

// RedisStorage implements the Storage interface using Redis for gamestate
// and filesystem for static resources
type RedisStorage struct {
    client  *redis.Client
    logger  *slog.Logger
    dataDir string
}

// Ensure RedisStorage implements Storage interface
var _ Storage = (*RedisStorage)(nil)

func NewRedisStorage(redisURL string, dataDir string, logger *slog.Logger) *RedisStorage {
    rdb := redis.NewClient(&redis.Options{
        Addr: redisURL,
    })

    if dataDir == "" {
        dataDir = "./data"
    }

    return &RedisStorage{
        client:  rdb,
        logger:  logger,
        dataDir: dataDir,
    }
}

// Health and lifecycle
func (r *RedisStorage) Ping(ctx context.Context) error {
    cmd := r.client.Ping(ctx)
    if err := cmd.Err(); err != nil {
        return fmt.Errorf("redis ping failed: %w", err)
    }
    return nil
}

func (r *RedisStorage) Close() error {
    if err := r.client.Close(); err != nil {
        r.logger.Error("Failed to close Redis connection", "error", err)
        return err
    }
    r.logger.Info("Redis connection closed")
    return nil
}

// GameState operations (keep existing redis.go implementations)
func (r *RedisStorage) SaveGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error {
    // ... existing implementation from services/redis.go
}

func (r *RedisStorage) LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error) {
    // ... existing implementation from services/redis.go
}

func (r *RedisStorage) DeleteGameState(ctx context.Context, id uuid.UUID) error {
    // ... existing implementation from services/redis.go
}

// Scenario operations (move from services/redis.go, keep filesystem loading)
func (r *RedisStorage) ListScenarios(ctx context.Context) (map[string]string, error) {
    scenariosDir := filepath.Join(r.dataDir, "scenarios")
    scenarios := make(map[string]string)

    err := filepath.WalkDir(scenariosDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
            return nil
        }

        file, err := os.ReadFile(path)
        if err != nil {
            r.logger.Warn("Failed to read scenario file", "path", path, "error", err)
            return nil
        }

        var s scenario.Scenario
        if err := json.Unmarshal(file, &s); err != nil {
            r.logger.Warn("Failed to unmarshal scenario file", "path", path, "error", err)
            return nil
        }

        filename := filepath.Base(path)
        scenarios[s.Name] = filename
        return nil
    })

    if err != nil {
        r.logger.Error("Failed to walk scenarios directory", "error", err)
        return nil, fmt.Errorf("failed to list scenarios: %w", err)
    }

    return scenarios, nil
}

func (r *RedisStorage) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
    path := filepath.Join(r.dataDir, "scenarios", filename)
    
    file, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("scenario not found: %s", filename)
        }
        return nil, fmt.Errorf("failed to read scenario file: %w", err)
    }

    var sc scenario.Scenario
    if err := json.Unmarshal(file, &sc); err != nil {
        return nil, fmt.Errorf("failed to unmarshal scenario: %w", err)
    }

    return &sc, nil
}

// Narrator operations (move from pkg/scenario/narrator.go)
func (r *RedisStorage) GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error) {
    if narratorID == "" {
        return nil, nil
    }

    narratorPath := filepath.Join(r.dataDir, "narrators", narratorID+".json")
    
    data, err := os.ReadFile(narratorPath)
    if err != nil {
        if os.IsNotExist(err) {
            absPath, _ := filepath.Abs(narratorPath)
            cwd, _ := os.Getwd()
            return nil, fmt.Errorf("narrator not found: %s (tried: %s, cwd: %s)", narratorID, absPath, cwd)
        }
        return nil, fmt.Errorf("failed to read narrator file %s: %w", narratorPath, err)
    }

    var narrator scenario.Narrator
    if err := json.Unmarshal(data, &narrator); err != nil {
        return nil, fmt.Errorf("failed to parse narrator JSON from %s: %w", narratorPath, err)
    }
    narrator.ID = narratorID

    return &narrator, nil
}

func (r *RedisStorage) ListNarrators(ctx context.Context) ([]string, error) {
    narratorsPath := filepath.Join(r.dataDir, "narrators")

    entries, err := os.ReadDir(narratorsPath)
    if err != nil {
        if os.IsNotExist(err) {
            return []string{}, nil
        }
        return nil, fmt.Errorf("failed to read narrators directory: %w", err)
    }

    var narratorIDs []string
    for _, entry := range entries {
        if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
            narratorID := entry.Name()[:len(entry.Name())-5]
            narratorIDs = append(narratorIDs, narratorID)
        }
    }

    return narratorIDs, nil
}

// PC operations (move from pkg/actor/pc.go, return PCSpec only)
func (r *RedisStorage) GetPCSpec(ctx context.Context, path string) (*actor.PCSpec, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read PC file: %w", err)
    }

    var spec actor.PCSpec
    if err := json.Unmarshal(data, &spec); err != nil {
        return nil, fmt.Errorf("failed to unmarshal PC spec: %w", err)
    }

    // Filename overrides any ID in the JSON
    spec.ID = strings.TrimSuffix(filepath.Base(path), ".json")

    return &spec, nil
}

func (r *RedisStorage) ListPCs(ctx context.Context) ([]string, error) {
    pcsPath := filepath.Join(r.dataDir, "pcs")

    entries, err := os.ReadDir(pcsPath)
    if err != nil {
        if os.IsNotExist(err) {
            return []string{}, nil
        }
        return nil, fmt.Errorf("failed to read PCs directory: %w", err)
    }

    var pcIDs []string
    for _, entry := range entries {
        if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
            pcID := entry.Name()[:len(entry.Name())-5]
            pcIDs = append(pcIDs, pcID)
        }
    }

    return pcIDs, nil
}
```

**Step 4.2: Create `internal/storage/mock.go`**
- Consolidate mock storage for testing
- Move from `internal/services/mock_storage.go`
- Implement all Storage interface methods

### Phase 5: Update Handlers

**Step 5.1: Update `internal/handlers/gamestate.go`**
- Replace `actor.LoadPC` with `storage.GetPCSpec` + `actor.NewPCFromSpec`
- Example change:

```go
// BEFORE:
loadedPC, pcErr := actor.LoadPC(pcPath)

// AFTER:
pcSpec, pcErr := h.storage.GetPCSpec(r.Context(), pcPath)
if pcErr != nil {
    // handle error
}
loadedPC, pcErr := actor.NewPCFromSpec(pcSpec)
```

**Step 5.2: Update `internal/handlers/chat.go`**
- Replace `scenario.LoadNarrator` with `storage.GetNarrator`
- Example change:

```go
// BEFORE (in pkg/state/gamestate.go):
narrator, err := scenario.LoadNarrator(narratorID)

// AFTER:
// Pass storage to GetChatMessages or load narrator in handler before calling it
```

**Step 5.3: Update handler constructors**
- Ensure handlers receive storage interface
- Update dependency injection

### Phase 6: Update Services

**Step 6.1: Move LLM services**
- Keep LLM-related files in `internal/services`
- Remove storage interface from services package
- Files remaining in services:
  - `llm.go`
  - `anthropic.go`
  - `venice.go`
  - `ollama.go`
  - `mock_llm.go`

**Step 6.2: Update imports across codebase**
- Change `services.Storage` → `storage.Storage`
- Update all handler constructors and tests

### Phase 7: Testing & Validation

**Step 7.1: Update tests**
- Update all mock storage usage
- Update handler tests
- Update integration tests

**Step 7.2: Verify no regressions**
- Run full test suite
- Manual testing of key flows:
  - Create new gamestate with PC
  - Load scenario
  - Chat interactions with narrator
  - PC loading

## Migration Checklist

### Pre-Migration
- [x] Document current architecture
- [x] Identify all storage touchpoints
- [x] Design new package structure
- [ ] Create feature branch (`git checkout -b storage-refactor`)

### Phase 1: Foundation
- [ ] Create `internal/storage/storage.go` with interfaces
- [ ] Create package structure
- [ ] Add basic tests for interfaces

### Phase 2: Resource Storage (Helper Functions)
- [ ] Implement `internal/storage/scenario.go` (helper functions)
- [ ] Implement `internal/storage/narrator.go` (helper functions)
- [ ] Implement `internal/storage/pc.go` (helper functions)
- [ ] Add unit tests for each helper

### Phase 3: Domain Updates
- [ ] Add `actor.NewPCFromSpec` constructor
- [ ] Deprecate `actor.LoadPC`
- [ ] Remove `scenario.LoadNarrator`
- [ ] Remove `scenario.ListNarrators`
- [ ] Update domain package tests

### Phase 4: Redis Storage Implementation
- [ ] Create `internal/storage/redis.go` with all Storage methods
- [ ] Move gamestate operations from `services/redis.go`
- [ ] Move scenario operations from `services/redis.go`
- [ ] Add narrator operations (from `pkg/scenario/narrator.go`)
- [ ] Add PC spec operations (from `pkg/actor/pc.go`)
- [ ] Create `internal/storage/mock.go`
- [ ] Test storage implementation

### Phase 5: Handler Updates
- [ ] Update `gamestate.go` handler
- [ ] Update `chat.go` handler
- [ ] Update `scenario.go` handler
- [ ] Update `pc.go` handler
- [ ] Update handler tests

### Phase 6: Services Cleanup
- [ ] Remove storage from `internal/services`
- [ ] Keep only LLM services
- [ ] Update imports throughout codebase

### Phase 7: Testing
- [ ] Run all unit tests
- [ ] Run integration tests
- [ ] Manual testing
- [ ] Performance validation
- [ ] Update documentation

### Post-Migration
- [ ] Remove deprecated functions
- [ ] Clean up TODOs
- [ ] Update README if needed
- [ ] Merge to main

## Benefits After Refactor

1. **Clear Separation of Concerns**
   - Storage layer: File/DB I/O operations
   - Domain layer: Business logic and models
   - Handler layer: HTTP/API logic

2. **Improved Testability**
   - Easy to mock storage without complex setup
   - Domain constructors testable in isolation
   - Clearer test boundaries

3. **Better Maintainability**
   - One file per resource type
   - Easy to find storage logic
   - Consistent patterns across resources

4. **Flexibility**
   - Easy to swap storage backends
   - Can add caching without changing handlers
   - Domain objects not coupled to I/O

5. **Reduced Complexity**
   - `PC` construction separated from loading
   - No mixed concerns in packages
   - Clearer dependency graph

## Risks & Mitigations

### Risk: Breaking existing code
**Mitigation**: 
- Implement in phases
- Keep deprecated functions temporarily
- Comprehensive testing at each phase

### Risk: Performance regression
**Mitigation**:
- Benchmark critical paths
- Monitor load times
- Consider caching layer if needed

### Risk: Missing edge cases
**Mitigation**:
- Review all usages of `LoadPC`, `LoadNarrator`
- Integration tests covering main flows
- Gradual rollout

## Open Questions

1. **Should we cache loaded resources?**
   - Recommendation: Add later if performance metrics indicate need

2. **How to handle data directory configuration?**
   - Recommendation: Pass dataDir to storage constructors, default to "./data"

3. **Should we extract filesystem loading into helper functions?**
   - Recommendation: Optional - can keep methods inline in RedisStorage for simplicity, or extract to separate files for organization

## Success Criteria

- ✅ All storage logic moved to `internal/storage`
- ✅ Domain packages (`pkg/*`) have no file I/O
- ✅ Handlers use storage interfaces only
- ✅ All tests passing
- ✅ No performance regressions
- ✅ Code coverage maintained or improved
- ✅ Documentation updated
