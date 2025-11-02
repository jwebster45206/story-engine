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
	"github.com/jwebster45206/story-engine/pkg/conditionals"
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

	// Build chat messages using the prompt builder
	// Note: req.Message should be pre-formatted with PC name if applicable
	messages, err := prompts.New().
		WithGameState(gs).
		WithScenario(loadedScenario).
		WithUserMessage(req.Message, chat.ChatRoleUser).
		WithHistoryLimit(PromptHistoryLimit).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build chat messages: %w", err)
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
			go p.syncGameState(metaCtx, gsCopy, req.Message, response.Message)
		}
	}

	// Update game state with new chat message
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: req.Message,
	})

	// Add to game state
	response.Message = strings.TrimRight(response.Message, "\n")
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

	// Build chat messages using the prompt builder
	// req.Message is already formatted with PC name if applicable
	messages, err := prompts.New().
		WithGameState(gs).
		WithScenario(loadedScenario).
		WithUserMessage(req.Message, chat.ChatRoleUser).
		WithHistoryLimit(PromptHistoryLimit).
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
	// Use the context passed in from the worker - it will stay alive while consuming the stream
	p.logger.Debug("Sending streaming chat request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	streamChan, err := p.llmService.ChatStream(ctx, messages)
	if err != nil {
		return nil, "", fmt.Errorf("LLM chat stream failed: %w", err)
	}

	// Return the stream channel and additional context for post-processing
	// The caller is responsible for consuming the stream and updating game state
	return streamChan, "", nil
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

	// Add to game state
	responseMessage = strings.TrimRight(responseMessage, "\n")
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

	if err := p.storage.SaveGameState(ctx, gs.ID, gs); err != nil {
		return fmt.Errorf("failed to save game state after streaming: %w", err)
	}

	// Start background gamestate delta update if game is not ended
	if !gs.IsEnded {
		go p.syncGameState(metaCtx, gs, userMessage, responseMessage)
	}

	p.logger.Debug("Game state updated after streaming", "game_state_id", gs.ID.String())
	return nil
}

// syncGameState runs in the background to extract and update the stateful parts of gamestate
func (p *ChatProcessor) syncGameState(ctx context.Context, gs *state.GameState, userMessage string, responseMessage string) {
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

	// Add the narrator response
	messages = append(messages, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

	metaCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send the gamestate delta request to the LLM (with one retry on error)
	var delta *conditionals.GameStateDelta
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
		WithStorage(p.storage).
		WithContext(metaCtx)

	// Apply vars first (before evaluating conditionals)
	worker.ApplyVars()

	// Apply the delta from the LLM reducer to the game state
	if err := worker.Apply(); err != nil {
		p.logger.Error("Failed to apply initial delta", "error", err, "game_state_id", latestGS.ID.String())
		return
	}

	// Now recursively evaluate and apply conditionals until none trigger
	p.applyConditionalsCascade(worker, latestGS.ID)

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

// applyConditionalsCascade recursively evaluates and applies conditionals until none trigger
func (p *ChatProcessor) applyConditionalsCascade(worker *state.DeltaWorker, gameStateID uuid.UUID) {
	const maxConditionalIterations = 10
	allTriggeredConditionals := make(map[string]bool) // Track all triggered conditional IDs

	for iteration := range maxConditionalIterations {
		// Evaluate conditionals based on current game state
		triggeredConditionals := worker.MergeConditionals()

		if len(triggeredConditionals) == 0 {
			// No new conditionals triggered, we're done
			break
		}

		// Check if we've seen any of these before (shouldn't happen, but safety check)
		foundNew := false
		for conditionalID := range triggeredConditionals {
			if !allTriggeredConditionals[conditionalID] {
				allTriggeredConditionals[conditionalID] = true
				foundNew = true
			}
		}

		if !foundNew {
			// All conditionals were already triggered, avoid infinite loop
			p.logger.Warn("Conditionals re-triggered, stopping to avoid loop",
				"game_state_id", gameStateID.String(),
				"iteration", iteration)
			break
		}

		// Apply vars from conditionals before applying other changes
		worker.ApplyVars()

		// Apply the conditional delta to game state
		if err := worker.Apply(); err != nil {
			p.logger.Error("Failed to apply conditional delta",
				"error", err,
				"game_state_id", gameStateID.String(),
				"iteration", iteration)
			return
		}

		// Log triggered conditionals
		for conditionalID, conditional := range triggeredConditionals {
			if conditional.Then.SceneChange != nil && conditional.Then.SceneChange.To != "" {
				p.logger.Info("Conditional scene change",
					"game_state_id", gameStateID.String(),
					"conditional_id", conditionalID,
					"to_scene", conditional.Then.SceneChange.To,
					"iteration", iteration)
			}
			if conditional.Then.GameEnded != nil {
				p.logger.Info("Conditional game ended",
					"game_state_id", gameStateID.String(),
					"conditional_id", conditionalID,
					"ended", *conditional.Then.GameEnded,
					"iteration", iteration)
			}
			if conditional.Then.Prompt != nil {
				previewLen := 50
				prompt := *conditional.Then.Prompt
				if len(prompt) < previewLen {
					previewLen = len(prompt)
				}
				p.logger.Info("Conditional prompt triggered",
					"game_state_id", gameStateID.String(),
					"conditional_id", conditionalID,
					"prompt_preview", prompt[:previewLen]+"...",
					"iteration", iteration)
			}
		}

		if iteration == maxConditionalIterations-1 {
			p.logger.Warn("Max conditional iterations reached",
				"game_state_id", gameStateID.String(),
				"iterations", maxConditionalIterations)
		}
	}
}

// GetGameState loads a game state by ID
func (p *ChatProcessor) GetGameState(ctx context.Context, gameStateID uuid.UUID) (*state.GameState, error) {
	gs, err := p.storage.LoadGameState(ctx, gameStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load game state: %w", err)
	}
	if gs == nil {
		return nil, fmt.Errorf("game state not found: %s", gameStateID.String())
	}
	return gs, nil
}
