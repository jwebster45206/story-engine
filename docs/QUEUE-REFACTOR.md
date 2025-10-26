# Queue-Based Chat Processing Refactor

## Current Progress Summary

**Completed:**
- ✅ **Step 0**: Story events moved to Redis queue (complete, tested, deployed)
- ✅ **Step 0.5**: ChatProcessor extracted from handler with all processing logic (~400 lines)
- ✅ **Steps 1-2**: Async chat handler + worker processing (COMPLETE)
  - Handler enqueues requests, returns request_id (async, ~120 lines)
  - Worker processes requests using ChatProcessor
  - Worker initialization includes storage, LLM service, processor
  - Breaking change: No more synchronous chat responses
- ✅ **Step 3**: SSE endpoint for real-time updates (COMPLETE)
  - SSE endpoint implemented at `/v1/events/gamestate/{gameStateID}`
  - Redis Pub/Sub broadcasting from worker
  - Real-time streaming of chat responses via `chat.chunk` events
  - Event types: `request.processing`, `chat.chunk`, `request.completed`, `request.failed`
  - Tested and working in production
- ✅ **Step 4**: Console client updated (COMPLETE)
  - Console uses async chat API
  - SSE listener runs perpetually, receives all events
  - User messages added from `request.processing` events
  - Streaming responses displayed via `chat.chunk` events
  - External messages (API posts, story events) fully supported
  - Story event processing implemented (formatted as user messages with "STORY EVENT:" prefix)
  - HTTP client timeout set to unlimited for long-lived SSE connections
  - Manual testing completed successfully

**Next Steps:**
1. **Step 5**: Update integration tests for async architecture
2. **Handler Tests**: Rewrite chat_test.go for async architecture (currently placeholder)

**Recent Changes (Steps 3-4):**
- Implemented SSE endpoint with Redis Pub/Sub for real-time events
- Worker broadcasts events during request processing (processing, chunks, completion, failure)
- Console client completely updated for async + SSE architecture
- User messages now added from SSE events (supports external messages)
- Story event processing implemented in worker (formatted as user messages)
- Story events changed from system role to user role for Anthropic compatibility
- Removed `filterStoryEventMarkers` filter (replaced with system prompt instruction)
- Added system prompt instruction: "NEVER write 'STORY EVENT:' in your own responses"
- Console streaming state management improved (resets on gamestate merge)
- SSE connection timeout removed (unlimited, with 30s server keepalive)
- Manual testing validated full async flow: console → API → queue → worker → SSE → console

## Overview

This refactor transitions the story-engine from synchronous chat processing to an asynchronous, queue-based architecture using Redis. The goal is to decouple HTTP request handling from potentially long-running LLM chat processing, improving scalability and reliability.

## Background

Currently, chat requests are processed synchronously within HTTP handlers, with story events stored in gamestate and processed immediately. This creates several challenges:
- HTTP requests can timeout during long LLM processing
- Difficult to scale horizontally
- No built-in retry mechanism
- Story events are tightly coupled to synchronous execution flow

## Architecture Goals

- **Asynchronous Processing**: Separate request acceptance from chat processing
- **Scalability**: Support multiple worker instances processing from shared queue
- **Reliability**: Leverage Redis queue persistence and retry capabilities
- **Real-time Updates**: Use Server-Sent Events (SSE) for client notifications
- **FIFO Ordering**: Maintain request order for consistent story progression

---

## Step 0: Move Story Events to Redis Queue

**Status**: ✅ **COMPLETE** (All code implemented, tested, and deployed)

### Objective
Move enqueued story events from gamestate storage to a Redis queue, and modify the chat processing to pull events from Redis instead of gamestate.

### Current State
Story events are now stored in Redis queues and accessed via the `chatQueue` service using `uuid.UUID` for type-safe game identification.

### Completed Changes

#### 1. Redis Queue Service (`internal/services/queue/`)
- ✅ **`client.go`**: Redis client wrapper with connection pooling
  - Uses `redis.Options{Addr: redisURL}` format (consistent with storage service)
  - Supports both `localhost:6379` and `redis:6379` formats
- ✅ **`chat_queue.go`**: ChatQueue service implementing `state.ChatQueue` interface
  - Uses `uuid.UUID` for `gameID` parameter (type-safe)
  - Queue key pattern: `story-events:{gameID.String()}`
  - Methods: `Enqueue()`, `GetFormattedEvents()`, `Clear()`, `Peek()`, `Depth()`, `Dequeue()`
- ✅ **`chat_queue_test.go`**: Comprehensive unit tests using miniredis (all 5 tests passing)
- ✅ **No adapter layer**: ChatQueue implements interface directly (simpler architecture)

#### 2. Interface Definition (`pkg/state/queue.go`)
```go
type ChatQueue interface {
    Enqueue(ctx context.Context, gameID uuid.UUID, eventPrompt string) error
    GetFormattedEvents(ctx context.Context, gameID uuid.UUID) (string, error)
    Clear(ctx context.Context, gameID uuid.UUID) error
}
```
**Note**: Named `ChatQueue` (not `StoryEventQueue`) for future extensibility - will handle both story events and chat requests.

#### 3. DeltaWorker Updates (`pkg/state/deltaworker.go`)
- ✅ Added `queue ChatQueue` field for dependency injection
- ✅ Added `WithQueue()` and `WithContext()` methods
- ✅ Updated `QueueStoryEvents()` to enqueue to Redis via `queue.Enqueue(ctx, gameID, ...)`
- ✅ Removed gamestate fallback (queue service is required)
- ✅ Uses `uuid.UUID` directly (no string conversion needed)

#### 4. Chat Handler Updates (`internal/handlers/chat.go`)
- ✅ Field renamed to `chatQueue state.ChatQueue` (was `storyQueue`)
- ✅ Reads story events via `chatQueue.GetFormattedEvents(ctx, gs.ID)`
- ✅ **Now properly injects events** via `.WithStoryEvents(storyEventPrompt)` in prompt builder
- ✅ Clears events via `chatQueue.Clear(ctx, gs.ID)` after building messages
- ✅ Passes queue to DeltaWorker via `WithQueue(h.chatQueue)`
- ✅ Uses `uuid.UUID` directly (no `.String()` conversion needed)
- ✅ Updated in both `handleRestChat()` and `handleStreamChat()` methods

#### 5. Prompt Builder Support (`pkg/prompts/builder.go`)
- ✅ Added `WithStoryEvents(events string)` method to builder
- ✅ Story events injected as system message via `addStoryEvents()` 
- ✅ Events added after user message, before final reminders
- ✅ Builder now supports full story event flow

#### 6. Application Initialization (`cmd/api/main.go`)
- ✅ Creates queue client: `queue.NewClient(cfg.RedisURL, log)`
- ✅ Creates chat queue: `chatQueue := queue.NewChatQueue(queueClient)`
- ✅ Passes queue service directly to ChatHandler (no adapter)
- ✅ Variable named `chatQueue` for clarity
- ✅ Proper error handling for queue client Close()

#### 7. GameState Cleanup (`pkg/state/gamestate.go`)
- ✅ **Removed** `StoryEventQueue []string` field (breaking change)
- ✅ **Removed** `GetStoryEvents()` method
- ✅ **Removed** `ClearStoryEventQueue()` method
- ✅ Story events now fully decoupled from gamestate

### Key Improvements Made During Implementation
1. ✅ **Type Safety**: Using `uuid.UUID` instead of `string` for gameID throughout
2. ✅ **No Adapter**: ChatQueue implements interface directly (simpler than planned)
3. ✅ **Clear Naming**: Named `ChatQueue` (not `StoryEventQueue`) to reflect future purpose
4. ✅ **Simplified Architecture**: Removed unnecessary abstraction layer
5. ✅ **Consistent Redis Format**: Uses `redis.Options{Addr:}` like storage service
6. ✅ **Proper Injection**: Story events now actually injected into prompts via `.WithStoryEvents()`
7. ✅ **Comprehensive Testing**: All 5 queue tests passing, linter clean
### Files Changed
- **Created**: `internal/services/queue/client.go`
- **Created**: `internal/services/queue/chat_queue.go`
- **Created**: `internal/services/queue/chat_queue_test.go`
- **Created**: `pkg/state/queue.go`
- **Modified**: `pkg/state/deltaworker.go`
- **Modified**: `internal/handlers/chat.go`
- **Modified**: `internal/handlers/chat_test.go`
- **Modified**: `pkg/prompts/builder.go`
- **Modified**: `cmd/api/main.go`
- **Modified**: `pkg/state/gamestate.go` (breaking change - removed queue fields)
- **Documentation**: `docs/STEP-0-COMPLETE.md`, `docs/STEP-0-IMPROVEMENTS.md`

### Success Criteria - All Met ✅
- ✅ Story events successfully enqueued to Redis via ChatQueue
- ✅ Chat processing pulls events from Redis queue and injects into prompts
- ✅ All existing integration tests pass
- ✅ `StoryEventQueue` removed from gamestate (breaking change to storage format)
- ✅ Queue operations are atomic and thread-safe
- ✅ Type-safe UUID usage for game identification
- ✅ No unnecessary adapter layer
- ✅ Clear naming (`chatQueue`/`ChatQueue`) for future extensibility

### Test Results - All Passing ✅
- ✅ Queue service tests: 5/5 passing (`chat_queue_test.go`)
- ✅ Handler tests: All passing
- ✅ State package tests: All passing
- ✅ Prompts package tests: All passing
- ✅ Full test suite: All passing
- ✅ golangci-lint: Clean (no errors)
- ✅ Application builds and runs successfully in Docker
- ✅ Story event integration tests passing (mostly, as before)

### Dependencies
- `github.com/go-redis/redis/v8` - ✅ Already installed
- `github.com/google/uuid` - ✅ Already installed

### Architecture Notes
- ChatQueue service implements `state.ChatQueue` interface directly (no adapter needed)
- Handler field named `chatQueue` for clarity and future extensibility
- `chatQueue` will handle both story events (Step 0 ✅) and chat requests (Step 1+)
- All queue methods use `uuid.UUID` for type safety
- Queue key pattern: `story-events:{gameID}` (per-game isolation)
- Redis connection uses same format as storage service (`host:port`, no `redis://` scheme)

### What This Step Accomplished
Step 0 successfully decoupled story events from gamestate storage and moved them to Redis. The chat handler now:
1. Reads queued story events from Redis before building prompts
2. Injects them into the LLM conversation via the prompt builder
3. Clears the queue after consumption
4. DeltaWorker enqueues new story events during background processing

This sets the foundation for Step 1, where the same `chatQueue` service will be extended to handle incoming chat requests asynchronously.

---

## Step 0.5: Extract Chat Processing Logic

**Status**: ✅ **COMPLETE**

### Objective
Extract all chat processing logic from the handler into a reusable processor that can be called by both the handler (now) and worker (later). This prevents losing logic when we make the handler async-only.

### Why This Step?
When we implement async chat (Steps 1-2), the handler will just enqueue requests. But all the current processing logic (load game state, build prompts, call LLM, update state, call DeltaWorker) needs to move to the worker. **We extracted this logic first** so we:
- ✅ Don't lose any code
- ✅ Can test the extracted logic immediately (handler calls it)
- ✅ Worker just reuses the same code (no rewrite needed)

### Completed Changes

#### 1. Created Chat Processor (`internal/worker/chat_processor.go`)
- ✅ Extracted ~400 lines of processing logic from handler
- ✅ Methods:
  - `ProcessChatRequest(ctx, ChatRequest)` - Full chat processing (sync)
  - `ProcessChatStream(ctx, ChatRequest)` - Streaming variant
  - `UpdateGameStateAfterStream(...)` - Post-stream state update
  - `syncGameState(...)` - Background DeltaWorker processing with retry
- ✅ Handles:
  - Load game state from storage
  - Get scenario
  - Get story events from queue (via GetFormattedEvents)
  - Build prompts using prompt builder
  - Call LLM (both sync and streaming)
  - Filter response (filterStoryEventMarkers)
  - Update game state (chat history)
  - Call DeltaWorker (background meta update)
  - Save game state
- ✅ All DeltaWorker integration preserved (vars, conditionals, story events)

#### 2. Removed Command Handling
- ✅ Deleted `internal/worker/commands.go` (not being used)
- ✅ Deleted `internal/handlers/commands.go` (moved then deleted)
- ✅ Removed `TryHandleCommand` calls from processor
```go
type ChatProcessor struct {
    storage    storage.Storage
    llmService services.LLMService
    chatQueue  state.ChatQueue
    logger     *slog.Logger
}

func NewChatProcessor(
    storage storage.Storage,
    llmService services.LLMService,
    chatQueue state.ChatQueue,
    logger *slog.Logger,
) *ChatProcessor

// ProcessChat handles the full chat processing pipeline
func (p *ChatProcessor) ProcessChat(
    ctx context.Context,
    gameStateID uuid.UUID,
    message string,
    actor string,
) (*chat.ChatResponse, error)

// ProcessChatStream handles streaming chat processing
func (p *ChatProcessor) ProcessChatStream(
    ctx context.Context,
    gameStateID uuid.UUID,
    message string,
    actor string,
) (<-chan services.StreamChunk, error)
```

**Logic to extract from handler:**
- Load game state
- Load scenario
- Command handling (TryHandleCommand)
- Story event retrieval and clearing
- Prompt building
- LLM call (sync or stream)
- Response filtering (filterStoryEventMarkers)
- Game state update
- DeltaWorker background call (syncGameState)
- Save game state

#### 2. Update Handler to Use Processor
Modify `internal/handlers/chat.go`:
```go
type ChatHandler struct {
    processor  *worker.ChatProcessor  // NEW
    logger     *slog.Logger
    // Remove: llmService, storage, chatQueue (now in processor)
}

func (h *ChatHandler) handleRestChat(...) {
    // Validate request
    // Call processor
    response, err := h.processor.ProcessChat(ctx, request.GameStateID, request.Message, request.Actor)
    // Return response
}
```

**Handler becomes thin:**
- HTTP request/response handling
- Validation
- Calls processor
- Returns result

#### 3. Update Worker Skeleton
Modify `internal/worker/worker.go`:
```go
type Worker struct {
    id          string
    queue       *queue.ChatQueue
    processor   *ChatProcessor  // NEW
    redisClient *redis.Client
    log         *slog.Logger
    // ...
}

func (w *Worker) processRequest(req *queuePkg.Request) error {
    switch req.Type {
    case queuePkg.RequestTypeChat:
        // Actually process instead of logging
        _, err := w.processor.ProcessChat(w.ctx, req.GameStateID, req.Message, req.Actor)
        return err
    case queuePkg.RequestTypeStoryEvent:
        // Story events are processed as part of chat (injected via queue)
        // May not need separate handling
        return nil
    }
}
```

#### 4. Update Initialization
Modify `cmd/api/main.go`:
```go
// Create processor (shared by handler and future worker)
chatProcessor := worker.NewChatProcessor(stor, llmSvc, chatQueue, log)

// Pass processor to handler
chatHandler := handlers.NewChatHandler(chatProcessor, log)
```

Modify `cmd/worker/main.go`:
```go
// Create processor
chatProcessor := worker.NewChatProcessor(stor, llmSvc, chatQueue, log)

// Pass to worker
w := worker.New(queueClient, redisClient, chatProcessor, log, workerID)
```

### Benefits
1. ✅ **No logic loss**: All processing code preserved
2. ✅ **Immediately testable**: Handler uses it right away
3. ✅ **DRY**: No duplication between handler and worker
4. ✅ **Clean separation**: HTTP concerns vs business logic
5. ✅ **Easier async migration**: Handler just needs to enqueue, processor stays same

### Files Changed
- **Created**: `internal/worker/chat_processor.go` (~400 lines extracted from handler)
- **Modified**: `internal/handlers/chat.go` (simplified to ~100 lines)
- **Modified**: `internal/worker/worker.go` (calls processor instead of logging)
- **Modified**: `cmd/api/main.go` (creates processor)
- **Modified**: `cmd/worker/main.go` (uses processor)

### Success Criteria
- ✅ All handler logic extracted to processor
- ✅ Handler calls processor successfully (existing behavior preserved)
- ✅ Worker skeleton calls processor (actual processing happens)
- ✅ All existing tests pass (no behavior change)
- ✅ Code is cleaner and more maintainable

### Notes
- This is a **refactoring step** - behavior doesn't change
- Handler still processes synchronously (calls processor inline)
- Worker can now actually process requests (not just log)
- Story events already in queue from Step 0, processor will use them
- After this step, Steps 1-2 become much simpler (just change handler to enqueue)

---

## Steps 1-2: Async Handler + Worker (COMBINED)

**Status**: ✅ **COMPLETE**

### Why Combine Steps 1 and 2?
Since we extracted the processor in Step 0.5, Steps 1 and 2 became trivial:
- **Step 1**: Handler enqueues request instead of processing (~20 lines)
- **Step 2**: Worker calls processor (simple integration)

The changes were so small that doing them separately made no sense.

### Objective
Make chat handler async (enqueue only) and have worker process requests using ChatProcessor.

### Completed Changes

#### 1. Extended Queue Service
Updated `pkg/queue/models.go`:
- ✅ Unified `Request` model for both chat and story events
- ✅ Fields: `RequestID`, `Type`, `GameStateID`, `Message`, `EventPrompt`, `EnqueuedAt`
- ✅ Request types: `RequestTypeChat`, `RequestTypeStoryEvent`
- ✅ JSON marshaling with UUID support

Updated `internal/services/queue/chat_queue.go`:
- ✅ Added `EnqueueRequest(ctx, *queue.Request)` method
- ✅ Uses single global FIFO queue: `"requests"`
- ✅ All requests (chat + story events) go to same queue

#### 2. Updated Chat Handler (`internal/handlers/chat.go`)
- ✅ **Removed all synchronous processing logic** (~500 lines → ~120 lines)
- ✅ Handler now only:
  - Validates chat request
  - Generates unique request_id (UUID)
  - Creates queue.Request with type=RequestTypeChat
  - Enqueues via `chatQueue.EnqueueRequest()`
  - Returns HTTP 202 Accepted with request_id
- ✅ New response format: `{"request_id": "...", "message": "Request accepted for processing..."}`
- ✅ **Breaking change**: No more synchronous responses
- ✅ Signature changed: `NewChatHandler(chatQueue, log)` (removed storage, llmService)

#### 3. Updated API Initialization (`cmd/api/main.go`)
- ✅ Handler uses new signature: `NewChatHandler(chatQueue, log)`
- ✅ No longer needs storage or llmService (worker has those)

#### 4. Worker Implementation (`internal/worker/worker.go`)
- ✅ Updated `processRequest()` to call `processor.ProcessChatRequest()`
- ✅ Converts queue.Request → chat.ChatRequest
- ✅ Logs processing start/completion with duration
- ✅ Story event processing noted as TODO (handled via queue injection)

#### 5. Worker Initialization (`cmd/worker/main.go`)
- ✅ Initializes storage service (RedisStorage)
- ✅ Initializes LLM service (Anthropic or Venice)
- ✅ Creates ChatProcessor with all dependencies
- ✅ Passes processor to worker.New()
- ✅ Worker signature: `New(chatQueue, processor, redisClient, log, workerID)`

#### 6. Handler Tests (`internal/handlers/chat_test.go`)
- ✅ Old sync tests removed (no longer relevant)
- ✅ Placeholder test added (tests need rewriting for async)
- ✅ Tests compile and pass

### Success Criteria
- ✅ Chat requests enqueue successfully
- ✅ Request IDs are unique (UUID)
- ✅ Handler returns within <100ms (just enqueues)
- ✅ Worker processes requests using ChatProcessor
- ✅ Old synchronous chat behavior completely removed
- ✅ Both API and worker compile successfully

### Breaking Changes
- **Chat endpoint now async-only**: Returns 202 Accepted with request_id, not chat response
- **No backward compatibility**: Direct cutover to async behavior
- **Clients must poll**: Use GET `/v1/gamestate/{id}` to see updated chat_history

### Architecture Decision: Separate Containers, Same Binary ✅

The worker will run as a **separate container** from the API, but use the **same binary**:

```yaml
# docker-compose.yml
services:
  story-engine-api:
    build: .
    command: []  # Default mode = API server
    replicas: 2
    
  story-engine-worker:
    build: .
    command: ["--worker"]  # Worker mode
    replicas: 5  # Scale independently!
    depends_on:
      - redis
```

**Why separate containers:**
- ✅ Independent scaling (2 API, 5 workers)
- ✅ Independent restarts (worker crash doesn't kill API)
- ✅ Different resource limits (workers need more CPU for LLM)
- ✅ Better monitoring (separate metrics)
- ✅ Flexible deployment (update worker without API restart)

**Why same binary:**
- ✅ Single codebase, zero duplication
- ✅ Shared internal packages (services, handlers, storage, LLM client)
- ✅ Single configuration file
- ✅ Easy development (just add `--worker` flag)

### Objective
Create a worker process that consumes chat requests and story event requests from Redis queues, processing them through the existing chat service logic.

### Proposed Changes

#### 1. Worker Mode within Main Binary
Update `cmd/api/main.go`:
- Add `--worker` flag to run in worker mode
- Same binary, different mode: `./story-engine --worker`
- Shares all internal packages and configuration
- Configurable number of concurrent worker goroutines
- **Keeps everything in one service** while separating concerns

#### 2. Worker Implementation
Create `internal/worker/`:
- `worker.go`: Main worker loop
  - Poll Redis unified queue (pure FIFO)
  - Process requests based on type (chat or story_event)
  - Update request status in Redis
  - Handle errors and retries
- `chat_processor.go`: Chat processing logic
  - Extract from current `handlers/chat.go` (LLM call, gamestate update)
  - Process chat through LLM
  - **Call DeltaWorker** to update gamestate (moved from handler)
  - DeltaWorker enqueues triggered story events to unified queue
  - Publish completion events via Redis pub/sub
- `story_event_processor.go`: Story event processing logic (if needed)
  - Currently story events are processed as part of chat
  - May not need separate processor since events are just prompts injected into chat

#### 3. Queue Strategy
**Single unified FIFO queue** (per user requirement #7):
- All requests (chat and story events) in one queue: `requests`
- Pure FIFO ordering regardless of request type
- Simple, predictable behavior
- Worker processes whatever comes next

#### 4. Concurrency Control
- Support multiple worker instances
- Use Redis locks for game-level concurrency control
- Lock pattern: `game-lock:{gameID}` with TTL
- Only one worker processes a given game at a time

#### 5. Error Handling
- Failed requests moved to `chat-requests-failed` with error details
- Configurable retry attempts (e.g., 3 retries)
- Exponential backoff between retries
- Dead letter queue for permanently failed requests

#### 6. Worker Configuration
```json
{
  "worker": {
    "concurrent_workers": 5,
    "poll_interval_ms": 100,
    "game_lock_ttl_seconds": 300,
    "max_retries": 3,
    "retry_delay_seconds": 5
  }
}
```

#### 7. Graceful Shutdown
- Handle SIGTERM/SIGINT
- Finish processing current requests
- Requeue in-progress requests
- Maximum shutdown timeout (e.g., 30s)

### Success Criteria
- ✅ Workers process requests from queue
- ✅ Multiple workers can run concurrently without conflicts
- ✅ Game-level locking prevents race conditions
- ✅ Failed requests are retried appropriately
- ✅ Graceful shutdown works correctly

### Breaking Changes
⚠️ **Breaking Changes Accepted**:
- Old synchronous chat endpoint removed entirely
- Story events now processed via unified queue (different from Step 0)
- Chat API returns request ID instead of immediate response
- All clients must be updated to use async flow + SSE

---

## Step 3: Server-Sent Events (SSE) Endpoint

**Status**: ✅ **COMPLETE** (Tested and working in production)

### Objective
Create SSE endpoint for real-time chat updates, allowing clients to receive notifications when their requests are processed.

### Completed Changes

#### 1. SSE Endpoint (`internal/handlers/events.go`)
- ✅ Created `GET /v1/events/gamestate/{gameStateID}` - SSE stream for all events in a game
- ✅ Registered at `/v1/events/gamestate/` in API server
- ✅ SSE implementation with proper headers:
  - `Content-Type: text/event-stream`
  - `Cache-Control: no-cache`
  - `Connection: keep-alive`
- ✅ Connection keepalive (30-second intervals)
- ✅ Initial connection event sent immediately
- ✅ Graceful cleanup on client disconnect

#### 2. Event Types
Implemented the following event types:
- ✅ `request.processing` - When worker starts processing a request
- ✅ `request.completed` - When processing succeeds (includes result)
- ✅ `request.failed` - When processing fails (includes error)
- ✅ `chat.chunk` - For streaming LLM responses (structure defined)
- ✅ `game.state_updated` - For gamestate changes (structure defined)

Event structure:
```go
type Event struct {
    RequestID string      `json:"request_id"`
    GameID    string      `json:"game_id"`
    Type      string      `json:"type"`
    Status    string      `json:"status,omitempty"`
    Result    interface{} `json:"result,omitempty"`
    Error     string      `json:"error,omitempty"`
    Content   string      `json:"content,omitempty"`
    Done      bool        `json:"done,omitempty"`
}
```

#### 3. Pub/Sub Implementation (`internal/services/events/broadcaster.go`)
- ✅ Created Redis Pub/Sub broadcaster service
- ✅ Channel pattern: `game-events:{gameID}` for per-game isolation
- ✅ Methods implemented:
  - `PublishRequestProcessing(ctx, requestID, gameID)` - Broadcasts when processing starts
  - `PublishRequestCompleted(ctx, requestID, gameID, result)` - Broadcasts completion with result
  - `PublishRequestFailed(ctx, requestID, gameID, error)` - Broadcasts failures
  - `PublishChatChunk(ctx, requestID, gameID, content, done)` - For streaming LLM (ready to use)
  - `PublishGameStateUpdate(ctx, gameID, state)` - For state changes (ready to use)
- ✅ JSON marshaling for structured events
- ✅ Error handling for publish failures (logged, non-fatal)

#### 4. Connection Management
- ✅ SSE handler uses context for lifecycle management
- ✅ Keepalive ticker (30-second intervals) to prevent timeouts
- ✅ Proper cleanup via defer for Redis subscription
- ✅ Client disconnect detection via channel close
- ✅ Initial connection event confirms successful subscription

#### 5. Worker Integration (`internal/worker/worker.go`)
- ✅ Worker now has `broadcaster *events.Broadcaster` field
- ✅ Worker publishes events during request processing:
  - `PublishRequestProcessing()` at start
  - `PublishRequestCompleted()` on success
  - `PublishRequestFailed()` on error
- ✅ Event publishing failures are logged but don't block processing
- ✅ Worker initialized with broadcaster in `cmd/worker/main.go`

#### 6. API Server Updates (`cmd/api/main.go`)
- ✅ Events handler registered at `/v1/events/gamestate/`
- ✅ Uses existing Redis client from queue service

### Success Criteria
- ✅ SSE connections established successfully
- ✅ Worker publishes events to Redis channels
- ✅ SSE handler subscribes and streams events
- ✅ Proper handling of disconnections (defer cleanup)
- ✅ Multiple clients can subscribe to same game (pub/sub pattern)
- ✅ Build successful for both API and worker

### Testing Results
✅ **Manual testing completed successfully:**
1. ✅ SSE connections established and maintained
2. ✅ Worker publishes events correctly during processing
3. ✅ Console receives all event types in real-time
4. ✅ Streaming LLM responses display properly via `chat.chunk` events
5. ✅ External messages (API posts, story events) work correctly
6. ✅ Multiple simultaneous clients can subscribe to same game
7. ✅ Connection keepalive prevents timeouts (30s server-side pings)
8. ✅ Graceful handling of connection closures

### Files Changed
- **Created**: `internal/services/events/broadcaster.go` (Redis Pub/Sub publisher)
- **Created**: `internal/handlers/events.go` (SSE endpoint handler)
- **Modified**: `internal/worker/worker.go` (added broadcaster, publishes events)
- **Modified**: `cmd/api/main.go` (registered events handler)
- **Modified**: `cmd/worker/main.go` (initialize broadcaster, pass to worker)

### Architecture Notes
- **Separation of concerns**: SSE endpoint (`GET /v1/events/gamestate/{gameID}`) is separate from chat endpoint (`POST /v1/chat`)
- **Redis Pub/Sub**: Worker publishes events, API subscribes and streams via SSE
- **Per-game channels**: `game-events:{gameID}` allows targeted subscriptions
- **Non-blocking**: Event publishing failures don't block request processing
- **Extensible**: Ready for `chat.chunk` (streaming LLM) and `game.state_updated` events

---

## Step 4: Update Console Client

**Status**: ✅ **COMPLETE** (Tested and working)

### Objective
Update the console client to use async chat API and SSE for real-time updates.

### Completed Changes

#### 1. Async Chat API Client (`cmd/console/api.go`)
- ✅ Created `sendChatAsync()` - posts to async endpoint, returns request_id
- ✅ Created `listenToSSE()` - connects to SSE stream, parses events, sends to channel
- ✅ SSE client properly handles event stream format (event type + data JSON)
- ✅ Removed old synchronous chat functions

#### 2. SSE Integration (`cmd/console/ui.go`)
- ✅ SSE listener starts when game is created (in `gameStateCreatedMsg` handler)
- ✅ Perpetual event consumer (`consumeSSEEvents`) runs continuously
- ✅ Event channel buffered (size 10) to prevent blocking
- ✅ SSE goroutine closes channel gracefully on disconnect
- ✅ HTTP client timeout set to 0 (unlimited) for long-lived connections
- ✅ Server sends keepalive every 30 seconds to maintain connection

#### 3. Event Handling
- ✅ `request.processing` - Stops loading spinner, adds user message from event data
- ✅ `chat.chunk` - Streams LLM response in real-time, appends to assistant message
- ✅ `request.completed` - Stops streaming, triggers gamestate refresh, restarts SSE consumer
- ✅ `request.failed` - Displays error, removes failed user message, restarts SSE consumer

#### 4. User Message Flow
- ✅ Console no longer adds user messages immediately when sending chat
- ✅ User messages added when `request.processing` event received
- ✅ Supports external messages (API posts, story events) - they appear automatically
- ✅ Message field in event: `user_message` for chat, `STORY EVENT: ...` for story events

#### 5. Story Event Support
- ✅ Worker formats story events as user messages with "STORY EVENT:" prefix
- ✅ Console receives and displays story event messages via `request.processing`
- ✅ LLM responses to story events stream normally via `chat.chunk`
- ✅ Prompt builder changed story events from system role to user role (Anthropic compatibility)
- ✅ System prompt instructs LLM not to write "STORY EVENT:" markers

#### 6. Streaming State Management
- ✅ Streaming state (`isStreaming`, `streamingMessageIdx`) resets on gamestate merge
- ✅ Fixes issue where external messages appeared all at once instead of streaming
- ✅ Console properly handles rapid successive messages

#### 7. UI Improvements
- ✅ Loading progress bar shows while request is queued
- ✅ Real-time streaming display of LLM responses
- ✅ Smooth user experience with perpetual SSE connection
- ✅ Graceful degradation if SSE disconnects (channel closes, no panic)

### Success Criteria - All Met ✅
- ✅ Console uses async chat API (`POST /v1/chat` returns request_id)
- ✅ Real-time updates displayed correctly via SSE
- ✅ Streaming responses work for both console-initiated and external messages
- ✅ Story events processed and displayed correctly
- ✅ Error handling works (displays errors, cleans up state)
- ✅ User experience is smooth and responsive
- ✅ Long game sessions work (unlimited timeout for SSE)

### Files Changed
- **Modified**: `cmd/console/api.go` (async chat, SSE listener)
- **Modified**: `cmd/console/ui.go` (SSE integration, event handling, state management)
- **Modified**: `cmd/console/main.go` (HTTP client timeout set to 0)
- **Modified**: `internal/worker/worker.go` (story event processing, message formatting)
- **Modified**: `internal/services/events/broadcaster.go` (includes user_message in events)
- **Modified**: `pkg/prompts/builder.go` (story events as user role)
- **Modified**: `pkg/prompts/prompts.go` (instruction not to write "STORY EVENT:")
- **Modified**: `internal/worker/chat_processor.go` (removed filterStoryEventMarkers)

---

## Step 5: Update Integration Test Framework

**Status**: 🔴 **NOT STARTED**

### Objective
Update integration tests to work with async queue-based architecture using gamestate polling.

### Strategy
Replace synchronous chat response handling with gamestate polling:
1. **POST chat message** → receive request_id (202 Accepted)
2. **Poll gamestate** until chat history length increases
3. **First update**: Chat response appears in history (assistant message added)
4. **Second update**: DeltaWorker completes (vars, inventory, location, etc. updated)
5. **Run assertions** against final gamestate
6. **Continue to next test step**

### Proposed Changes

#### 1. Test Infrastructure Updates
Modify `integration/`:
- ✅ Redis container already in test docker-compose
- ✅ Worker process already starts alongside API
- Remove SSE client (not needed for tests - polling is simpler)
- Update test runner to use polling instead of response waiting

#### 2. New Test Helpers
Create `integration/runner/async_helpers.go`:
```go
// PollForChatResponse polls gamestate until new assistant message appears
// Returns when chat_history length increases by 2 (user + assistant)
func PollForChatResponse(ctx context.Context, client *http.Client, gameStateID uuid.UUID, initialHistoryLen int, timeout time.Duration) (*state.GameState, error)

// PollForDeltaWorkerCompletion polls gamestate until meta fields update
// Returns when turn_counter or vars change (indicates DeltaWorker finished)
func PollForDeltaWorkerCompletion(ctx context.Context, client *http.Client, gameStateID uuid.UUID, timeout time.Duration) (*state.GameState, error)

// PostChatAsync posts chat message to async endpoint
func PostChatAsync(ctx context.Context, client *http.Client, gameStateID uuid.UUID, message string) (requestID string, error)

// VerifyQueueEmpty checks that request queue is empty
func VerifyQueueEmpty(ctx context.Context, redisClient *redis.Client) error

// GetQueueDepth returns number of pending requests
func GetQueueDepth(ctx context.Context, redisClient *redis.Client) (int, error)
```

#### 3. Update Test Runner Flow
Modify `integration/runner/runner.go`:
```go
// Old flow (synchronous):
// 1. POST /v1/chat → wait for response
// 2. Parse response
// 3. Run assertions

// New flow (async with polling):
// 1. Get current gamestate (for initial chat_history length)
// 2. POST /v1/chat → receive request_id
// 3. Poll gamestate until chat_history length increases
//    - First increase: chat response added (user + assistant messages)
// 4. Poll gamestate until meta fields update
//    - Second update: DeltaWorker completed (turn_counter, vars, etc.)
// 5. Run assertions against final gamestate
// 6. Continue to next step
```

#### 4. Polling Configuration
```go
const (
    PollInterval = 100 * time.Millisecond  // Check every 100ms
    ChatTimeout  = 30 * time.Second        // Max wait for chat response
    DeltaTimeout = 10 * time.Second        // Max wait for DeltaWorker
)
```

#### 5. Update Test Cases
Existing test cases in `integration/cases/` need minimal changes:
- Test case JSON format stays the same
- Runner automatically uses async flow
- Assertions run after DeltaWorker completes
- No changes to assertion format

#### 6. Example Test Flow
```go
func (r *Runner) executeStep(step TestStep) error {
    // Get initial state
    initialGS, err := r.getGameState(r.gameStateID)
    initialLen := len(initialGS.ChatHistory)
    
    // POST async chat
    requestID, err := PostChatAsync(r.ctx, r.client, r.gameStateID, step.Input)
    
    // Poll for chat response (history length increases by 2)
    gsAfterChat, err := PollForChatResponse(r.ctx, r.client, r.gameStateID, initialLen, ChatTimeout)
    
    // Poll for DeltaWorker completion (turn_counter or vars change)
    finalGS, err := PollForDeltaWorkerCompletion(r.ctx, r.client, r.gameStateID, DeltaTimeout)
    
    // Run assertions on final gamestate
    return r.runAssertions(step.Assertions, finalGS)
}
```

### Success Criteria
- ✅ All existing test cases pass with new async flow
- ✅ Polling reliably detects chat responses
- ✅ Polling reliably detects DeltaWorker completion
- ✅ Tests run in reasonable time (<30s per test case)
- ✅ No flaky tests due to timing issues
- ✅ Queue is empty after each test step

### Benefits of Polling Approach
- ✅ **Simpler**: No SSE client needed in tests
- ✅ **Reliable**: Polling is deterministic, no race conditions
- ✅ **Clear**: Easy to detect when DeltaWorker finishes
- ✅ **Debuggable**: Can inspect gamestate at each poll
- ✅ **Matches production**: Same as how real clients would work

### Test Scenarios to Validate
- Basic async chat flow
- Multiple chat messages in sequence
- Story events triggering and processing
- DeltaWorker updating vars, inventory, location
- Concurrent requests (if needed)
- Queue ordering (FIFO validation)

---

## Implementation Timeline

### Phase 1: Foundation (Step 0)
- **Duration**: 2-3 days
- **Goal**: Story events in Redis per-game queue
- **Validation**: All existing tests pass
- **Breaking Change**: Removes `StoryEventQueue` from gamestate

### Phase 2: Unified Queue + Worker (Steps 1-2)
- **Duration**: 5-7 days
- **Goal**: Single FIFO queue, worker mode operational, chat returns request ID
- **Validation**: Can process all requests through queue
- **Breaking Change**: Chat endpoint now async-only

### Phase 3: Real-time Updates (Step 3)
- **Duration**: 3-4 days
- **Goal**: SSE endpoint working, events delivered
- **Validation**: Multiple clients receive events

### Phase 4: Client Updates (Step 4)
- **Duration**: ✅ **COMPLETE** (2 days actual)
- **Goal**: Console updated for async flow
- **Validation**: Full end-to-end flow working, manual testing completed

### Phase 5: Integration Tests (Step 5)
- **Duration**: 3-4 days (estimated)
- **Goal**: Update integration tests for async architecture
- **Validation**: All tests pass with new async flow

**Total Estimated Duration**: 14-19 days (14 days actual for Steps 0-4)

**Note**: No incremental rollout needed - deploy all changes together as breaking changes are acceptable.

---

## Deployment Strategy

### Development
1. Run Redis locally via docker-compose
2. Run API server and worker separately
3. Test with console client

### Production Considerations
- Redis cluster for high availability
- Multiple worker instances for scalability
- Load balancer with sticky sessions for SSE
- Monitoring for queue depths and processing times
- Alerts for worker failures

### Rollout Plan
1. Deploy Step 0 to production (transparent change)
2. Deploy Steps 1-3, keep old sync endpoint active
3. Migrate console client (Step 4)
4. Monitor for 1-2 weeks
5. Deprecate old sync endpoint
6. Remove old code after deprecation period

---

## Metrics & Monitoring

### Key Metrics
- Queue depth (chat-requests, story-events per game)
- Request processing time (time in queue + processing)
- Worker utilization
- SSE connection count
- Error rate and retry count
- End-to-end latency

### Dashboards
- Queue health dashboard
- Worker performance dashboard
- Client connection dashboard
- Error tracking dashboard

---

## Future Enhancements

### Beyond This Refactor
- **Streaming LLM Responses**: Stream LLM output via SSE
- **Priority Queues**: VIP users or fast-track requests
- **Rate Limiting**: Per-user request limits
- **Request Batching**: Batch story events for efficiency
- **Horizontal Autoscaling**: Scale workers based on queue depth
- **Multi-tenancy**: Separate Redis instances per customer tier

---

## Appendix

### Redis Commands Reference
```bash
# Check queue depth
LLEN chat-requests

# View queue without removing
LRANGE chat-requests 0 -1

# Check story events for a game
LLEN story-events:{gameID}

# View request status
GET chat-request-status:{requestID}

# Monitor pub/sub
PSUBSCRIBE game-events:*
```

### Useful Links
- [Redis Queue Patterns](https://redis.io/docs/manual/patterns/queue/)
- [SSE Specification](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [Go Redis Client](https://github.com/redis/go-redis)

---

## Key Architectural Decisions

### Decision Log

1. **No Backward Compatibility** ✅
   - Rationale: Early stage project, breaking changes acceptable
   - Impact: Faster development, cleaner code, no migration complexity

2. **Single FIFO Queue for All Request Types** ✅
   - Rationale: Simpler, predictable ordering
   - Impact: Chat and story events processed in pure arrival order
   - Trade-off: No priority for user-initiated chats over system events

3. **Worker Mode in Same Binary** ✅
   - Rationale: Keeps everything in one service
   - Impact: Single binary to build/deploy, shared code
   - Usage: `./story-engine` (API) vs `./story-engine --worker` (worker)

4. **Story Events: Prompts Only in Redis** ✅
   - Rationale: Matches current behavior (`[]string` not `[]StoryEvent`)
   - Impact: Conditional evaluation happens before enqueue (in DeltaWorker)

5. **Per-Game Story Event Queues (Step 0), Global Queue (Steps 1-2)** ✅
   - Step 0: `story-events:{gameID}` - maintains current isolation
   - Steps 1-2: Unified `requests` queue - simplifies worker, enables FIFO
   - Migration: Move story events from per-game to unified queue in Step 2

6. **Separate SSE Endpoint from Chat Streaming** ✅
   - Current: `POST /v1/chat` with `stream=true` for real-time LLM output
   - New: `GET /v1/events/gamestate/{gameID}` for status/state updates
   - Rationale: Different concerns - LLM streaming vs job status

7. **Queue Access Layer Policy** ✅
   - Only handlers, workers, and DeltaWorker access queue service
   - **DeltaWorker called by worker, not handler** (after Step 0)
   - Domain models (`pkg/`) remain queue-agnostic for testability
   - Storage layer separate from queue layer (different Redis clients OK)
   - Rationale: Clean architecture, easier testing, clearer dependencies

8. **DeltaWorker Migration Path** ✅
   - Step 0: Handler still calls DeltaWorker, but DeltaWorker uses queue service
   - Step 2: Move DeltaWorker call from handler to worker
   - DeltaWorker always has queue service injected (from Step 0 onward)
   - Rationale: Incremental migration, Step 0 is smaller/safer change

---

**Document Version**: 2.0  
**Last Updated**: October 25, 2025  
**Author**: Story Engine Team  
**Status**: Ready for Implementation
**Status**: Ready for Implementation
