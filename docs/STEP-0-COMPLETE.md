# Step 0 Complete: Redis Queue for Story Events

**Date Completed:** 2025-10-25  
**Status:** ✅ COMPLETE

## Summary

Successfully migrated story event queuing from in-memory gamestate storage to Redis-backed queues. This is Step 0 of the larger async queue refactor plan (see `QUEUE-REFACTOR.md`).

**Key Improvements:**
- ✅ Type-safe UUID usage for game identification
- ✅ Removed unnecessary adapter layer (direct interface implementation)
- ✅ Renamed to `chatQueue` for clarity and future extensibility
- ✅ Cleaner, simpler architecture

## Changes Implemented

### 1. Queue Service Infrastructure

Created new queue service package at `/internal/services/queue/`:

- **`client.go`**: Redis client wrapper with connection pooling and health checks
- **`story_event_queue.go`**: Story event queue service with per-game FIFO queues
  - Queue key pattern: `story-events:{gameID}`
  - Operations: Enqueue, Dequeue, Peek, Clear, Depth, GetFormattedEvents
  - Implements `state.StoryEventQueue` interface directly
  - **Uses `uuid.UUID` for type-safe game identification**
- **`story_event_queue_test.go`**: Comprehensive unit tests using miniredis (all passing)

### 2. Interface Definition

Created `/pkg/state/queue.go`:
- Defined `StoryEventQueue` interface for dependency injection
- Enables testability and decoupling from Redis implementation
- Three core methods with `uuid.UUID` parameters:
  ```go
  Enqueue(ctx context.Context, gameID uuid.UUID, eventPrompt string) error
  GetFormattedEvents(ctx context.Context, gameID uuid.UUID) (string, error)
  Clear(ctx context.Context, gameID uuid.UUID) error
  ```

### 3. DeltaWorker Updates

Modified `/pkg/state/deltaworker.go`:
- Added `queue StoryEventQueue` field for queue service injection
- Added `WithQueue()` method for fluent configuration
- Added `WithContext()` method for context propagation
- Updated `QueueStoryEvents()` to enqueue to Redis instead of gamestate
- **Uses `uuid.UUID` directly**: `queue.Enqueue(ctx, gs.ID, ...)`
- Removed fallback to gamestate queue (breaking change as planned)

### 4. Chat Handler Updates

Modified `/internal/handlers/chat.go`:
- Added `chatQueue state.StoryEventQueue` field (renamed from `storyQueue`)
- Updated `NewChatHandler()` signature to accept queue service
- **Uses `uuid.UUID` directly**: `chatQueue.GetFormattedEvents(r.Context(), gs.ID)`
- Clear story events: `chatQueue.Clear(r.Context(), gs.ID)`
- Inject queue into DeltaWorker: `WithQueue(h.chatQueue)`

**Note**: Renamed to `chatQueue` to reflect future purpose of handling both story events and chat requests.

### 5. Application Initialization

Modified `/cmd/api/main.go`:
- Initialize Redis client from config
- Create `StoryEventQueue` service (implements interface directly)
- Pass queue service directly to `ChatHandler` as `chatQueue`
- **No adapter needed** - direct interface implementation

### 6. Prompt Builder Updates

Modified `/pkg/prompts/builder.go`:
- Added `storyEvents string` field to Builder
- Added `WithStoryEvents()` method
- Updated `BuildMessages()` to accept story events parameter
- Modified `addStoryEvents()` to use injected string instead of reading from gamestate

### 7. GameState Cleanup (Breaking Changes)

Modified `/pkg/state/gamestate.go`:
- **REMOVED** `StoryEventQueue []string` field from GameState struct
- **REMOVED** `GetStoryEvents()` method
- **REMOVED** `ClearStoryEventQueue()` method
- Removed `strings` import (no longer needed)

### 8. Test Updates

**Handler Tests** (`/internal/handlers/chat_test.go`):
- Created `mockStoryEventQueue` implementation for testing
- Updated all `NewChatHandler` calls to include mock queue parameter
- All handler tests passing ✅

**State Package Tests** (`/pkg/state/gamestate_test.go`):
- Removed `TestGameState_GetStoryEvents`
- Removed `TestGameState_ClearStoryEventQueue`
- Removed `TestGameState_StoryEventQueue_Persistence`
- Removed `TestGameState_StoryEventQueue_EnqueueDequeue`
- Removed `encoding/json` import (no longer needed)
- All state tests passing ✅

**Prompts Package Tests** (`/pkg/prompts/builder_test.go`):
- Updated `TestBuilder_Build_WithStoryEvents` to use `WithStoryEvents()`
- Updated `TestBuildMessages` to pass empty string for story events
- Updated `TestBuildMessages_ErrorHandling` to pass empty string
- All prompts tests passing ✅

## Test Results

```bash
# Queue service tests
go test ./internal/services/queue/... -v
# PASS: 5/5 tests (0.403s)

# State package tests
go test ./pkg/state/... -v
# PASS: All tests

# Handler tests
go test ./internal/handlers/... -v
# PASS: All tests (0.384s)

# Prompts package tests
go test ./pkg/prompts/... -v
# PASS: All tests (0.334s)

# Full test suite
go test ./... -short
# PASS: All packages
```

## Application Build Status

```bash
go build ./cmd/api/
# SUCCESS ✅
```

## Architecture Changes

### Before (Gamestate-based)
```
Chat Handler → DeltaWorker → GameState.StoryEventQueue ([]string)
                ↓
           GameState.GetStoryEvents() → Prompt Builder
```

### After (Redis-based with UUID)
```
Chat Handler → DeltaWorker → Redis Queue (story-events:{gameID.String()})
    ↓                             ↓
    └─────→ chatQueue.GetFormattedEvents(ctx, uuid.UUID) → Prompt Builder
```

**Key Improvements:**
- Type-safe `uuid.UUID` instead of string conversion
- Direct interface implementation (no adapter)
- Renamed to `chatQueue` for clarity
- Simpler, cleaner architecture

## Breaking Changes

1. **GameState Storage Format**: The `story_event_queue` field is removed from serialized game states
   - Existing saved games with queued events will lose those events when loaded
   - Acceptable as per user requirement (no backward compatibility needed)

2. **DeltaWorker Fallback**: No longer falls back to gamestate when queue unavailable
   - Queue service is now required for story events to function
   - Story events will be lost if queue service fails (logged as error)

3. **API Signatures**:
   - `NewChatHandler()` now requires `StoryEventQueue` parameter
   - `BuildMessages()` now requires `storyEvents string` parameter
   - All queue methods use `uuid.UUID` instead of `string` for gameID

4. **Variable Naming**:
   - Field renamed from `storyQueue` to `chatQueue` in handlers
   - Reflects future purpose of handling chat requests in addition to story events

## Redis Configuration

Queue service uses existing Redis configuration from `docker-compose.yml`:
```yaml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
```

Connection string from config: `redis://redis:6379`

## Queue Key Naming

Story event queues use the pattern: `story-events:{gameID.String()}`

Example: `story-events:1a6594e3-b1c9-4126-b403-33334a298e71`

**Type Safety**: All queue methods accept `uuid.UUID` and convert to string only within the queue service.

## Error Handling

- Queue failures during enqueue are logged but don't fail game state updates
- Story events may be lost if Redis is unavailable (graceful degradation)
- Queue service logs errors at ERROR level with game_id and event context

## Performance Characteristics

- **Enqueue**: O(1) via Redis RPUSH
- **Dequeue/GetFormattedEvents**: O(N) where N = queue depth
- **Clear**: O(1) via Redis DEL
- **Connection**: Pooled via go-redis/redis/v8 client

## Next Steps

As documented in `QUEUE-REFACTOR.md`, the next phases are:

- **Step 1**: Move chat processing to background queue worker
- **Step 2**: Add status endpoint and SSE for real-time updates
- **Step 3**: Graceful shutdown and queue draining
- **Step 4**: Metrics and monitoring

## Files Changed

**Created:**
- `/internal/services/queue/client.go`
- `/internal/services/queue/story_event_queue.go`
- `/internal/services/queue/story_event_queue_test.go`
- `/pkg/state/queue.go`

**Modified:**
- `/pkg/state/deltaworker.go`
- `/pkg/state/gamestate.go`
- `/pkg/state/gamestate_test.go`
- `/internal/handlers/chat.go`
- `/internal/handlers/chat_test.go`
- `/cmd/api/main.go`
- `/pkg/prompts/builder.go`
- `/pkg/prompts/builder_test.go`

**Deleted:**
- `/internal/services/queue/adapter.go` (unnecessary - `StoryEventQueue` implements interface directly)

**Lines Changed:** ~600 lines added, ~250 lines removed

## Validation Checklist

- ✅ Queue service unit tests pass (5/5)
- ✅ Handler tests pass with mock queue
- ✅ State package tests pass
- ✅ Prompts package tests pass
- ✅ Full test suite passes
- ✅ Application builds successfully
- ✅ No compile errors or warnings
- ✅ StoryEventQueue removed from gamestate
- ✅ DeltaWorker uses queue service
- ✅ Chat handler reads from Redis queue
- ✅ Prompt builder accepts story events parameter
- ✅ Type-safe UUID usage throughout
- ✅ No adapter layer (direct interface implementation)
- ✅ Clear naming with `chatQueue`

## Notes

- Integration tests require `--tags=integration` flag and running services (not executed in this phase)
- Redis must be running for queue operations to succeed
- Story events are now ephemeral (cleared after consumption, not persisted in gamestate)
- Queue service is mandatory for story events feature; no fallback exists
- **Type Safety**: Using `uuid.UUID` eliminates string conversion errors
- **Simplified Architecture**: Removed adapter layer for cleaner code
- **Future-Ready**: `chatQueue` naming prepares for Step 1 (chat request queueing)
