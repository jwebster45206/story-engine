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

**Status**: 🔴 Not Started

### Objective
Move enqueued story events from gamestate storage to a Redis queue, and modify the chat processing to pull events from Redis instead of gamestate.

### Current State
- Story events are stored in gamestate as part of the game state structure
- Events are processed during synchronous chat handler execution
- Events are coupled to the gamestate storage layer

### Proposed Changes

#### 1. Redis Client Access & Architecture Layers
- Redis is **already configured and running** (docker-compose.yml)
- Config already has `RedisURL` field
- Storage layer already uses Redis (`github.com/go-redis/redis/v8`)
- **Action**: Create shared Redis client in `internal/services/queue/` (don't reuse storage client)

**Layer Access Policy:**

✅ **Should Access Queue Service:**
- `internal/handlers/chat.go` - Enqueues incoming chat requests only (no longer processes them)
- `internal/worker/` - Dequeues requests, calls DeltaWorker, enqueues story events
- `pkg/state/deltaworker.go` - **Needs queue service injected** to enqueue story events after delta processing
- Integration tests - For validation and assertions

❌ **Should NOT Access Queue Directly:**
- `pkg/prompts/` - Prompt building logic (pure functions)
- `pkg/scenario/` - Scenario/scene models (data structures)
- `pkg/chat/` - Chat models (data structures)
- `pkg/state/gamestate.go` - Game state model (should remain queue-agnostic)
- `internal/storage/` - Keep storage separate from queue concerns
- Console client - Uses HTTP API only, not queue internals

**Rationale:**
- Queue service is infrastructure, accessed via dependency injection
- Keep domain models (`pkg/`) clean and testable
- **DeltaWorker moves to worker context** - handlers no longer call it
- DeltaWorker needs queue service to enqueue story events after processing deltas
- Handlers become thin (validate + enqueue only)

#### 2. New Queue Service
Create `internal/services/queue/` with:
- `story_event_queue.go`: Service for managing story event queues
  - `EnqueueStoryEvent(gameID, eventID string, event StoryEvent) error`
  - `DequeueStoryEvent(gameID string) (*StoryEvent, error)`
  - `PeekStoryEvents(gameID string, limit int) ([]StoryEvent, error)`
  - `ClearStoryEvents(gameID string) error`
  - `GetQueueDepth(gameID string) (int, error)`

#### 3. Queue Data Model
```go
type StoryEventMessage struct {
    GameID      string
    EventPrompt string    // Just the prompt text (matching current behavior)
    EnqueuedAt  time.Time
}
```

Queue naming convention: `story-events:{gameID}`

**Note**: We only store the prompt strings in Redis, matching the current gamestate behavior where `StoryEventQueue` is `[]string`. Conditional evaluation happens before queueing.

#### 4. Update Chat Processing
- Chat handler will NO LONGER call DeltaWorker (this moves to worker in Step 2)
- Update `DeltaWorker.QueueStoryEvents()` signature to accept queue service
- For Step 0, maintain synchronous chat flow but read story events from Redis queue
- DeltaWorker enqueues story events to Redis instead of gamestate

#### 5. Migration Strategy
- **Remove** `StoryEventQueue` field from gamestate struct (breaking change, but acceptable)
- Update `DeltaWorker` constructor to accept queue service as dependency
- Update `DeltaWorker.QueueStoryEvents()` to enqueue directly to Redis via injected queue service
- Update chat handler to read story events from Redis queue (handler still calls DeltaWorker in Step 0)
- **In Step 2**: Move DeltaWorker call from handler to worker completely
- No feature flags needed - direct cutover to Redis

#### 6. Testing
- Unit tests for queue service operations
- Integration tests verifying story events flow through Redis
- Test queue persistence and recovery scenarios
- Existing integration test cases work unchanged (they don't inspect queue directly)
- Redis already available in integration test setup (docker-compose.test.yml)

#### 7. Configuration
Redis is already configured! Current setup:
- `config.RedisURL` field exists
- docker-compose.yml has Redis service
- Integration tests have Redis available

**No additional configuration needed** - use existing Redis connection.

### Success Criteria
- ✅ Story events successfully enqueued to Redis
- ✅ Chat processing pulls events from Redis queue
- ✅ All existing integration tests pass
- ✅ `StoryEventQueue` removed from gamestate (breaking change to storage format, not API)
- ✅ Queue operations are atomic and thread-safe

### Dependencies
- `github.com/go-redis/redis/v8` - **Already installed** ✅
- Consider upgrading to `github.com/redis/go-redis/v9` (optional, for better features)

### Risks & Mitigations
- **Risk**: Redis unavailability breaks story events
  - **Mitigation**: Implement connection retry logic with exponential backoff
  - **Mitigation**: Consider graceful degradation (fallback to in-memory queue)
- **Risk**: Data loss during migration
  - **Mitigation**: Thorough testing of migration logic
  - **Mitigation**: Add logging for all queue operations

---

## Step 1: Async Chat Handler with Queue

**Status**: 🔴 Not Started

### Objective
Create a new chat handler endpoint that accepts chat requests and enqueues them to Redis, returning immediately with a request ID.

### Proposed Changes

#### 1. Unified Queue Strategy
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
- Store request status in Redis with TTL (e.g., 1 hour after completion)
- Key pattern: `chat-request-status:{requestID}`
- Include: status, game_id, enqueued_at, started_at, completed_at, error (if any)

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

**Status**: 🔴 Not Started

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

**Status**: 🔴 Not Started

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

**Status**: 🔴 Not Started

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

**Status**: 🔴 Not Started

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
