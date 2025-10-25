package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/prompts"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

const PromptHistoryLimit = 6

// ChatProcessor handles the core chat processing logic
// It's used by both the HTTP handler (synchronously) and the worker (asynchronously)
type ChatProcessor struct {
	storage    storage.Storage
	llmService services.LLMService
	chatQueue  state.ChatQueue
	logger     *slog.Logger

	// For background gamestate delta cancellation
	metaCancelMu sync.Mutex
	metaCancel   map[uuid.UUID]context.CancelFunc
}

// NewChatProcessor creates a new chat processor
func NewChatProcessor(
	storage storage.Storage,
	llmService services.LLMService,
	chatQueue state.ChatQueue,
	logger *slog.Logger,
) *ChatProcessor {
	return &ChatProcessor{
		storage:    storage,
		llmService: llmService,
		chatQueue:  chatQueue,
		logger:     logger,
		metaCancel: make(map[uuid.UUID]context.CancelFunc),
	}
}

// ProcessChatRequest processes a chat request and returns the response
func (p *ChatProcessor) ProcessChatRequest(ctx context.Context, req chat.ChatRequest) (*chat.ChatResponse, error) {
	// Load game state
	gs, err := p.storage.LoadGameState(ctx, req.GameStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load game state: %w", err)
	}

	if gs == nil {
		return nil, fmt.Errorf("game state not found: %s", req.GameStateID.String())
	}

	// Get Scenario for the chat
	loadedScenario, err := p.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		return nil, fmt.Errorf("failed to load scenario: %w", err)
	}

	// Check for queued story events from Redis queue
	storyEventPrompt := ""
	if p.chatQueue != nil {
		var err error
		storyEventPrompt, err = p.chatQueue.GetFormattedEvents(ctx, gs.ID)
		if err != nil {
			p.logger.Error("Error getting story events from queue", "error", err, "game_id", gs.ID.String())
			// Continue without story events on error
		}
	}
	if storyEventPrompt != "" {
		p.logger.Debug("Story events will be injected", "game_state_id", gs.ID, "events", storyEventPrompt)
	}

	// Build chat messages using the prompt builder
	messages, err := prompts.New().
		WithGameState(gs).
		WithScenario(loadedScenario).
		WithUserMessage(req.Message, chat.ChatRoleUser).
		WithHistoryLimit(PromptHistoryLimit).
		WithStoryEvents(storyEventPrompt).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build chat messages: %w", err)
	}

	// Clear story events after building messages
	if storyEventPrompt != "" && p.chatQueue != nil {
		if err := p.chatQueue.Clear(ctx, gs.ID); err != nil {
			p.logger.Error("Failed to clear chat queue", "error", err, "game_id", gs.ID.String())
		}
	}

	chatCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p.logger.Debug("Sending chat request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	response, err := p.llmService.Chat(chatCtx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// Cancel any in-process gamestate delta for this game state
	p.metaCancelMu.Lock()
	if cancel, ok := p.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	p.metaCancel[gs.ID] = metaCancel
	p.metaCancelMu.Unlock()

	if !gs.IsEnded {
		// Make a deep copy for the background goroutine to avoid data races
		gsCopy, err := gs.DeepCopy()
		if err != nil {
			p.logger.Error("Failed to copy game state for background sync", "error", err, "game_state_id", gs.ID.String())
		} else {
			// Start background goroutine to update game meta (PromptState)
			go p.syncGameState(metaCtx, gsCopy, req.Message, response.Message, storyEventPrompt)
		}
	}

	// Update game state with new chat message
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: req.Message,
	})

	// Filter out "STORY EVENT:" markers from LLM response and add to game state
	response.Message = strings.TrimRight(response.Message, "\n")
	response.Message = filterStoryEventMarkers(response.Message)
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: response.Message,
	})

	// Save the updated game state
	if err := p.storage.SaveGameState(ctx, gs.ID, gs); err != nil {
		return nil, fmt.Errorf("failed to save game state: %w", err)
	}

	response.GameStateID = gs.ID
	return response, nil
}

// ProcessChatStream processes a streaming chat request
func (p *ChatProcessor) ProcessChatStream(ctx context.Context, req chat.ChatRequest) (<-chan services.StreamChunk, string, error) {
	// Load game state
	gs, err := p.storage.LoadGameState(ctx, req.GameStateID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load game state: %w", err)
	}

	if gs == nil {
		return nil, "", fmt.Errorf("game state not found: %s", req.GameStateID.String())
	}

	// Get Scenario for the chat
	loadedScenario, err := p.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load scenario: %w", err)
	}

	// Check for queued story events from Redis queue
	storyEventPrompt := ""
	if p.chatQueue != nil {
		var err error
		storyEventPrompt, err = p.chatQueue.GetFormattedEvents(ctx, gs.ID)
		if err != nil {
			p.logger.Error("Error getting story events from queue", "error", err, "game_id", gs.ID.String())
			// Continue without story events on error
		}
	}
	if storyEventPrompt != "" {
		p.logger.Debug("Story events will be injected", "game_state_id", gs.ID.String(), "events", storyEventPrompt)
	}

	// Build chat messages using the prompt builder
	messages, err := prompts.New().
		WithGameState(gs).
		WithScenario(loadedScenario).
		WithUserMessage(req.Message, chat.ChatRoleUser).
		WithHistoryLimit(PromptHistoryLimit).
		WithStoryEvents(storyEventPrompt).
		Build()
	if err != nil {
		return nil, "", fmt.Errorf("failed to build chat messages: %w", err)
	}

	// Clear story events after consumption
	if p.chatQueue != nil {
		if err := p.chatQueue.Clear(ctx, gs.ID); err != nil {
			p.logger.Error("Failed to clear chat queue", "error", err, "game_state_id", gs.ID.String())
		}
	}

	// Initialize LLM streaming
	chatCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p.logger.Debug("Sending streaming chat request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	streamChan, err := p.llmService.ChatStream(chatCtx, messages)
	if err != nil {
		return nil, "", fmt.Errorf("LLM chat stream failed: %w", err)
	}

	// Return the stream channel and additional context for post-processing
	// The caller is responsible for consuming the stream and updating game state
	return streamChan, storyEventPrompt, nil
}

// UpdateGameStateAfterStream updates game state after streaming is complete
// This should be called by the handler after consuming the stream
func (p *ChatProcessor) UpdateGameStateAfterStream(gs *state.GameState, userMessage, responseMessage, storyEventPrompt string) error {
	ctx := context.Background()

	// Cancel any in-process gamestate delta for this game state
	p.metaCancelMu.Lock()
	if cancel, ok := p.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	p.metaCancel[gs.ID] = metaCancel
	p.metaCancelMu.Unlock()

	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: userMessage,
	})

	// Filter out "STORY EVENT:" markers from LLM response and add to game state
	responseMessage = strings.TrimRight(responseMessage, "\n")
	responseMessage = filterStoryEventMarkers(responseMessage)
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

	if err := p.storage.SaveGameState(ctx, gs.ID, gs); err != nil {
		return fmt.Errorf("failed to save game state after streaming: %w", err)
	}

	// Start background gamestate delta update if game is not ended
	if !gs.IsEnded {
		go p.syncGameState(metaCtx, gs, userMessage, responseMessage, storyEventPrompt)
	}

	p.logger.Debug("Game state updated after streaming", "game_state_id", gs.ID.String())
	return nil
}

// syncGameState runs in the background to extract and update the stateful parts of gamestate
func (p *ChatProcessor) syncGameState(ctx context.Context, gs *state.GameState, userMessage string, responseMessage string, storyEventPrompt string) {
	start := time.Now()
	p.logger.Debug("Starting background game gamestate delta", "game_state_id", gs.ID.String(), "response", responseMessage)
	defer func() {
		p.metaCancelMu.Lock()
		delete(p.metaCancel, gs.ID)
		p.metaCancelMu.Unlock()
	}()

	currentStateJSON, err := json.Marshal(prompts.ToBackgroundPromptState(gs))
	if err != nil {
		p.logger.Error("Failed to marshal current game state for gamestate delta", "error", err, "game_state_id", gs.ID.String())
		return
	}

	s, err := p.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		p.logger.Error("Failed to get scenario from storage", "error", err, "game_state_id", gs.ID.String())
		return
	}

	contingencyRules := prompts.GlobalContingencyRules
	contingencyRules = append(contingencyRules, s.ContingencyRules...)
	if gs.SceneName != "" {
		contingencyRules = append(contingencyRules, s.Scenes[gs.SceneName].ContingencyRules...)
	}

	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: fmt.Sprintf(prompts.ReducerPrompt, strings.Join(contingencyRules, "\n- ")),
		},
		{
			Role:    chat.ChatRoleSystem,
			Content: fmt.Sprintf("BEFORE game state: %s", string(currentStateJSON)),
		},
		{
			Role:    chat.ChatRoleUser,
			Content: userMessage,
		},
	}

	// Add story event message if it exists
	if storyEventPrompt != "" {
		messages = append(messages, chat.ChatMessage{
			Role:    chat.ChatRoleSystem,
			Content: storyEventPrompt,
		})
	}

	// Add the narrator response
	messages = append(messages, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

	metaCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send the gamestate delta request to the LLM (with one retry on error)
	var delta *state.GameStateDelta
	var backendModel string
	var deltaErr error

	maxAttempts := 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			p.logger.Info("Retrying gamestate delta extraction", "game_state_id", gs.ID.String(), "attempt", attempt)
		}

		p.logger.Debug("Sending gamestate delta request to LLM", "game_state_id", gs.ID.String(), "attempt", attempt)
		delta, backendModel, deltaErr = p.llmService.DeltaUpdate(metaCtx, messages)

		if deltaErr == nil {
			p.logger.Debug("Received gamestate delta from LLM", "game_state_id", gs.ID.String(), "delta", delta, "backend_model", backendModel)
			break
		}

		// Log error and retry if not the last attempt
		if attempt < maxAttempts {
			p.logger.Warn("Gamestate delta extraction failed, will retry", "error", deltaErr, "game_state_id", gs.ID.String(), "attempt", attempt)
		} else {
			p.logger.Error("Failed to get meta extraction response from LLM after retries", "error", deltaErr, "game_state_id", gs.ID.String(), "attempts", maxAttempts)
			return
		}
	}

	if delta == nil {
		return
	}

	latestGS, err := p.storage.LoadGameState(metaCtx, gs.ID)
	if err != nil {
		p.logger.Error("Failed to load latest game state for gamestate delta", "error", err, "game_state_id", gs.ID.String())
		return
	}
	if latestGS == nil {
		p.logger.Warn("Game state not found during gamestate delta", "game_state_id", gs.ID.String())
		return
	}

	// Increment turn counters on the latest game state
	if !latestGS.IsEnded {
		latestGS.IncrementTurnCounters()
	}

	// Use DeltaWorker to handle all delta application logic
	worker := state.NewDeltaWorker(latestGS, delta, s, p.logger).
		WithQueue(p.chatQueue).
		WithContext(metaCtx)

	// Apply vars first (before evaluating conditionals)
	worker.ApplyVars()

	// Evaluate conditionals and override delta based on results
	triggeredConditionals := worker.ApplyConditionalOverrides()

	// Log triggered conditionals
	if len(triggeredConditionals) > 0 {
		for conditionalID, conditional := range triggeredConditionals {
			if conditional.Then.Scene != "" {
				p.logger.Info("Conditional scene change", "game_state_id", latestGS.ID.String(), "conditional_id", conditionalID, "to_scene", conditional.Then.Scene)
			}
			if conditional.Then.GameEnded != nil {
				p.logger.Info("Conditional game ended", "game_state_id", latestGS.ID.String(), "conditional_id", conditionalID, "ended", *conditional.Then.GameEnded)
			}
		}
	}

	// Queue story events for next turn
	triggeredEvents := worker.QueueStoryEvents()
	if len(triggeredEvents) > 0 {
		for eventKey, event := range triggeredEvents {
			previewLen := 50
			if len(event.Prompt) < previewLen {
				previewLen = len(event.Prompt)
			}
			p.logger.Info("Story event queued", "game_state_id", latestGS.ID.String(), "event_key", eventKey, "prompt_preview", event.Prompt[:previewLen]+"...")
		}
	}

	// Apply the final delta to the game state
	if err := worker.Apply(); err != nil {
		p.logger.Error("Failed to apply delta", "error", err, "game_state_id", latestGS.ID.String())
		return
	}

	// Save the updated game state
	if err := p.storage.SaveGameState(metaCtx, latestGS.ID, latestGS); err != nil {
		p.logger.Error("Failed to save updated game state after meta extraction", "error", err, "game_state_id", latestGS.ID.String())
		return
	}

	p.logger.Debug("Updated game meta",
		"game_state_id", gs.ID.String(),
		"delta", delta,
		"duration_s", time.Since(start).Seconds(),
		"backend_model", backendModel,
	)
}

// filterStoryEventMarkers removes "STORY EVENT:" markers from LLM responses
// The LLM sometimes includes these markers despite instructions not to
func filterStoryEventMarkers(text string) string {
	// Remove "STORY EVENT:" at the start of lines (case-insensitive)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for "STORY EVENT:" prefix (case-insensitive)
		if len(trimmed) >= 12 {
			prefix := strings.ToUpper(trimmed[:12])
			if prefix == "STORY EVENT:" {
				// Remove the prefix and preserve the rest
				lines[i] = strings.TrimSpace(trimmed[12:])
			}
		}
	}
	return strings.Join(lines, "\n")
}
