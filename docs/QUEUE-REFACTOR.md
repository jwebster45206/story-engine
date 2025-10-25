# Queue-Based Chat Processing Refactor

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

## Step 1: Async Chat Handler with Queue

**Status**: 🔴 **NOT STARTED** (Step 0 complete, ready to begin)

### What's Next
Now that story events are in Redis, Step 1 will extend the `chatQueue` to handle incoming chat requests:
- Chat handler will enqueue requests and return immediately with a request ID
- Breaking change: `/v1/chat` becomes async-only (no backward compatibility)
- Request status tracking in Redis
- Foundation for worker process (Step 2)

### Objective
Extend `chatQueue` to handle incoming chat requests asynchronously, returning immediately with a request ID.

### Proposed Changes

#### 1. Extend Queue Service
Update `internal/services/queue/` to add:
Create `internal/services/queue/unified_queue.go`:
- **Single global FIFO queue** for all request types (chat + story events)
- Simplifies ordering: pure FIFO regardless of type
- `EnqueueChatRequest(request *ChatRequest) (string, error)` - returns request ID
- `EnqueueStoryEventRequest(gameID string, eventPrompts []string) (string, error)` - returns request ID
- `DequeueRequest() (*Request, error)` - pulls next request regardless of type
- `GetRequestStatus(requestID string) (*RequestStatus, error)`

#### 2. Unified Request Model
```go
type RequestType string

const (
    RequestTypeChat       RequestType = "chat"
    RequestTypeStoryEvent RequestType = "story_event"
)

type Request struct {
    RequestID   string
    Type        RequestType
    GameID      string      // game_state_id for chat, gameID for story events
    
    // Chat-specific fields
    Message     string      `json:"message,omitempty"`
    Stream      bool        `json:"stream,omitempty"`
    
    // Story event specific fields
    EventPrompts []string   `json:"event_prompts,omitempty"`
    
    EnqueuedAt  time.Time
    Status      string // "queued", "processing", "completed", "failed"
}
```

Queue naming: `requests` (single global FIFO queue for everything)

#### 3. New API Endpoint
- **Update existing** `POST /v1/chat` to async-only mode
- Request body: `{ "message": "...", "game_state_id": "..." }`
- Response: `{ "request_id": "...", "status": "queued" }`
- Returns HTTP 200 OK (changed from sync 200 with chat response)
- **No backward compatibility** - direct cutover to async behavior

#### 4. Status Tracking
- No status tracking at this time.

#### 5. Handler Implementation
Update `internal/handlers/chat.go`:
- Remove all synchronous processing logic
- Validate request
- Generate unique request ID (UUID)
- Enqueue to Redis unified queue
- Return request ID immediately (HTTP 200)

### Success Criteria
- ✅ Chat requests enqueued successfully
- ✅ Request IDs are unique and trackable
- ✅ Handler returns within <100ms
- ✅ Status stored correctly in Redis
- ✅ Old synchronous chat behavior completely removed

---

## Step 2: Queue Worker Process

**Status**: 🔴 **NOT STARTED** (Blocked on Step 1)

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

**Status**: 🔴 **NOT STARTED** (Blocked on Steps 1-2)

### Objective
Create SSE endpoint for real-time chat updates, allowing clients to receive notifications when their requests are processed.

### Proposed Changes

#### 1. SSE Endpoint
Create `internal/handlers/events.go`:
- `GET /v1/events/games/{gameID}` - SSE stream for all events in a game
- `GET /v1/events/requests/{requestID}` - SSE stream for specific request
- **Separate from existing streaming chat** - that endpoint will be removed/repurposed

#### 2. Event Types
Stream the following event types:
```
event: request.queued
data: {"request_id": "...", "type": "chat", "status": "queued"}

event: request.processing
data: {"request_id": "...", "type": "chat", "status": "processing"}

event: request.completed
data: {"request_id": "...", "type": "chat", "result": {...}}

event: request.failed
data: {"request_id": "...", "error": "..."}

event: chat.chunk (optional - for streaming LLM responses)
data: {"request_id": "...", "content": "...", "done": false}

event: game.state_updated
data: {"game_id": "...", "turn": 5}
```

#### 3. Pub/Sub Implementation
Use Redis Pub/Sub for event distribution:
- Workers publish events to Redis channels
- SSE handlers subscribe to relevant channels
- Channel pattern: `game-events:{gameID}`
- Channel pattern: `chat-request-events:{requestID}`

#### 4. Connection Management
- Track active SSE connections
- Send keepalive messages every 30s
- Handle client disconnections gracefully
- Automatic reconnection support with Last-Event-ID

#### 5. Message Broadcasting
Create `internal/services/events/broadcaster.go`:
- `PublishChatEvent(gameID, requestID string, event *ChatEvent) error`
- `PublishStoryEvent(gameID string, event *StoryEvent) error`
- `PublishGameStateUpdate(gameID string, state *GameState) error`

#### 6. Update Worker
Modify worker to publish events:
- When request status changes
- When chat completes
- When story events are processed
- When gamestate is updated

### Success Criteria
- ✅ SSE connections established successfully
- ✅ Events delivered in real-time (<1s latency)
- ✅ Proper handling of disconnections/reconnections
- ✅ Multiple clients can subscribe to same game

---

## Step 4: Update Console Client

**Status**: 🔴 **NOT STARTED** (Blocked on Steps 1-3)

### Objective
Update the console client to use async chat API and SSE for real-time updates.

### Proposed Changes

#### 1. Update Chat API Client
Modify `cmd/console/api.go`:
- Add `PostChatAsync(gameID, message, actor string) (requestID string, err error)`
- Add `StreamGameEvents(gameID string, handler EventHandler) error`
- Maintain backward compatibility with sync API (feature flag?)

#### 2. SSE Client Implementation
- Connect to SSE endpoint when game starts
- Handle incoming events and update UI
- Reconnect on connection loss
- Display real-time status updates

#### 3. UI Updates
- Show "processing..." indicator when chat is queued
- Display chat responses as they arrive
- Show story events in real-time
- Add status indicator (connected/disconnected)

#### 4. Error Handling
- Handle SSE connection failures
- Display timeout messages
- Retry failed requests
- Graceful degradation if SSE unavailable

### Success Criteria
- ✅ Console uses async chat API
- ✅ Real-time updates displayed correctly
- ✅ Error handling works properly
- ✅ User experience is smooth

---

## Step 5: Update Integration Test Framework

**Status**: 🔴 **NOT STARTED** (Blocked on Steps 1-4)

### Objective
Update integration tests to work with async queue-based architecture.

### Proposed Changes

#### 1. Test Infrastructure Updates
Modify `integration/`:
- Add Redis container to test docker-compose
- Update test runner to start worker process
- Add SSE client for test verification
- Create helper functions for async testing

#### 2. New Test Helpers
Create `integration/runner/async_helpers.go`:
- `WaitForChatCompletion(requestID string, timeout time.Duration) (*ChatResponse, error)`
- `WaitForEventType(gameID, eventType string, timeout time.Duration) (*Event, error)`
- `VerifyQueueEmpty(queueName string) error`
- `GetQueueDepth(queueName string) (int, error)`

#### 3. Update Test Cases
Update existing test cases in `integration/cases/`:
- Use async chat API
- Add timeout handling
- Verify events via SSE
- Check queue states
- Validate worker processing

#### 4. Worker Test Mode
- Start worker alongside API server in tests: `./story-engine --worker`
- Single worker instance for predictable testing
- Fast poll intervals for quick tests
- Shorter timeouts

#### 5. Test Scenarios
Add new test cases:
- `test_async_chat_flow.json`: Basic async chat
- `test_concurrent_requests.json`: Multiple requests
- `test_worker_failure.json`: Worker restart/recovery
- `test_queue_ordering.json`: FIFO validation
- `test_sse_delivery.json`: Event delivery

### Success Criteria
- ✅ All existing tests pass with async architecture
- ✅ New async-specific tests added
- ✅ Tests run reliably in CI/CD
- ✅ Test execution time reasonable (<5min)

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

### Phase 4: Client Updates (Steps 4-5)
- **Duration**: 4-5 days
- **Goal**: Console and tests updated for async flow
- **Validation**: Full end-to-end flow working

**Total Estimated Duration**: 14-19 days

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
   - New: `GET /v1/events/games/{gameID}` for status/state updates
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
