package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/prompts"
	"github.com/jwebster45206/story-engine/pkg/queue"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

const PromptHistoryLimit = 16

// LLMResolver looks up an LLMService by provider name (e.g. *llm.Registry).
type LLMResolver interface {
	Get(name string) (llm.LLMService, error)
}

// ChatOrchestrator orchestrates referee, resolve, narrator, and reducer for the worker (streaming + post-stream state updates).
type ChatOrchestrator struct {
	storage      storage.Storage
	resolver     LLMResolver
	chatQueue    state.ChatQueue
	logger       *slog.Logger
	historyLimit int
	roller       *d20.Roller

	// For background gamestate delta cancellation
	metaCancelMu sync.Mutex
	metaCancel   map[uuid.UUID]context.CancelFunc
}

// NewChatOrchestrator creates a new chat orchestrator
func NewChatOrchestrator(
	storage storage.Storage,
	resolver LLMResolver,
	chatQueue state.ChatQueue,
	logger *slog.Logger,
	historyLimit int,
) *ChatOrchestrator {
	if historyLimit <= 0 {
		historyLimit = PromptHistoryLimit
	}
	return &ChatOrchestrator{
		storage:      storage,
		resolver:     resolver,
		chatQueue:    chatQueue,
		logger:       logger,
		historyLimit: historyLimit,
		roller:       d20.NewRandomRoller(),
		metaCancel:   make(map[uuid.UUID]context.CancelFunc),
	}
}

// ProcessChatStream processes a streaming chat request
func (p *ChatOrchestrator) ProcessChatStream(ctx context.Context, req chat.ChatRequest) (<-chan llm.StreamChunk, []resolvedAttempt, *chat.Ruling, error) {
	// Load game state
	gs, err := p.storage.LoadGameState(ctx, req.GameStateID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load game state: %w", err)
	}

	if gs == nil {
		return nil, nil, nil, fmt.Errorf("game state not found: %s", req.GameStateID.String())
	}

	loadedScenario, err := p.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load scenario: %w", err)
	}

	svc, err := p.resolver.Get(gs.Provider)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to resolve LLM provider %q: %w", gs.Provider, err)
	}

	ruling, err := p.referee(ctx, svc, gs, req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to run referee: %w", err)
	}

	attempts := p.resolveAttempts(gs, ruling)
	messages, err := prompts.BuildNarratorMessages(gs, loadedScenario, req.Message, p.historyLimit, ruling, narratorTexts(attempts)...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build chat messages: %w", err)
	}

	if p.chatQueue != nil {
		if err := p.chatQueue.Clear(ctx, gs.ID); err != nil {
			p.logger.Error("Failed to clear chat queue", "error", err, "game_state_id", gs.ID.String())
		}
	}

	temperature := gs.Temperature
	p.logger.Debug("Sending streaming chat request to LLM", "game_state_id", gs.ID.String(), "provider", gs.Provider, "messages", messages)
	streamChan, err := svc.ChatStream(ctx, messages, temperature)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("LLM chat stream failed: %w", err)
	}

	return streamChan, attempts, ruling, nil
}

// referee runs the pre-chat rules pass. LLM failures fail open: a nil ruling
// means the narrator prompt is unchanged. Message-build errors are returned.
func (p *ChatOrchestrator) referee(ctx context.Context, svc llm.LLMService, gs *state.GameState, req chat.ChatRequest) (*chat.Ruling, error) {
	if req.Ruling != nil {
		ruling := *req.Ruling
		ruling.Normalize()
		p.logRuling(gs, &ruling, 0)
		return &ruling, nil
	}
	if !req.UseReferee {
		return nil, nil
	}

	refMessages, err := prompts.BuildRefereeMessages(gs, req.Message, p.historyLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to build referee messages: %w", err)
	}

	start := time.Now()
	refCtx, cancel := context.WithTimeout(ctx, llmRequestTimeout)
	defer cancel()
	ruling, usage, err := svc.GetRuling(refCtx, refMessages)
	p.logger.Info("llm usage",
		"type", "referee",
		"game_state_id", gs.ID,
		"principal_id", gs.PrincipalID,
		"usage", usage,
	)
	if err != nil {
		p.logger.Error("Referee failure", "error", err, "game_state_id", gs.ID.String(), "duration_ms", time.Since(start).Milliseconds())
		return nil, nil
	}
	if ruling == nil {
		p.logger.Warn("Referee returned empty ruling", "game_state_id", gs.ID.String(), "duration_ms", time.Since(start).Milliseconds())
		return nil, nil
	}
	if ruling.NarratorText() == "" {
		p.logger.Warn("Referee returned empty ruling", "game_state_id", gs.ID.String(), "duration_ms", time.Since(start).Milliseconds())
		return nil, nil
	}
	p.logRuling(gs, ruling, time.Since(start).Milliseconds())
	return ruling, nil
}

func (p *ChatOrchestrator) logRuling(gs *state.GameState, ruling *chat.Ruling, durationMS int64) {
	if ruling == nil {
		return
	}
	text := ruling.NarratorText()
	p.logger.Debug("Referee ruling",
		"game_state_id", gs.ID.String(),
		"duration_ms", durationMS,
		"scope", ruling.Scope,
		"subject", ruling.Subject,
		"object", ruling.Object,
		"ruling", text,
	)
}

// HandleAfterStream saves chat history after streaming, then runs the background
// reducer and may enqueue a combat reaction.
func (p *ChatOrchestrator) HandleAfterStream(ctx context.Context, gs *state.GameState, userMessage, responseMessage string, isStoryEvent bool, ruling *chat.Ruling) error {
	// Cancel any in-process gamestate delta for this game state
	p.metaCancelMu.Lock()
	if cancel, ok := p.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	p.metaCancel[gs.ID] = metaCancel
	p.metaCancelMu.Unlock()

	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:         chat.ChatRoleUser,
		Content:      userMessage,
		IsStoryEvent: isStoryEvent,
	})

	// Add to game state
	responseMessage = strings.TrimRight(responseMessage, "\n")
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

	if err := p.storage.UpdateGameState(ctx, gs.ID, gs); err != nil {
		return fmt.Errorf("failed to save game state after streaming: %w", err)
	}

	// Start background gamestate delta update if game is not ended
	if !gs.IsEnded {
		gsCopy, err := gs.DeepCopy()
		if err != nil {
			p.logger.Error("Failed to copy game state for background sync", "error", err, "game_state_id", gs.ID.String())
		} else {
			pending := pendingCombatReaction(gs, ruling)
			go func() {
				latest := p.syncGameState(metaCtx, gsCopy, responseMessage)
				p.enqueueReaction(metaCtx, latest, pending)
			}()
		}
	}

	p.logger.Debug("Game state updated after streaming", "game_state_id", gs.ID.String())
	return nil
}

// syncGameState runs in the background to extract and update the stateful parts of gamestate.
// It returns the saved post-apply state, or nil if the reducer did not apply.
func (p *ChatOrchestrator) syncGameState(ctx context.Context, gs *state.GameState, responseMessage string) *state.GameState {
	start := time.Now()
	p.logger.Debug("Starting background game gamestate delta", "game_state_id", gs.ID.String(), "response", responseMessage)
	defer func() {
		p.metaCancelMu.Lock()
		delete(p.metaCancel, gs.ID)
		p.metaCancelMu.Unlock()
	}()

	s, err := p.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		p.logger.Error("Failed to get scenario from storage", "error", err, "game_state_id", gs.ID.String())
		return nil
	}

	messages, err := prompts.BuildReducerMessages(gs, s, responseMessage)
	if err != nil {
		p.logger.Error("Failed to build reducer messages", "error", err, "game_state_id", gs.ID.String())
		return nil
	}

	// Send the gamestate delta request to the LLM
	var delta *conditionals.GameStateDelta
	var usage llm.Usage
	var deltaErr error

	maxAttempts := 2
	svc, err := p.resolver.Get(gs.Provider)
	if err != nil {
		p.logger.Error("Failed to resolve LLM provider for delta", "error", err, "provider", gs.Provider, "game_state_id", gs.ID.String())
		return nil
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			p.logger.Info("Retrying gamestate delta extraction", "game_state_id", gs.ID.String(), "attempt", attempt)
		}

		p.logger.Debug("Sending gamestate delta request to LLM", "game_state_id", gs.ID.String(), "provider", gs.Provider, "attempt", attempt)
		// deltaCtx is the deadline for this DeltaUpdate attempt.
		deltaCtx, deltaCancel := context.WithTimeout(ctx, llmRequestTimeout)
		delta, usage, deltaErr = svc.DeltaUpdate(deltaCtx, messages)
		deltaCancel()

		switch {
		case ctx.Err() != nil:
			p.logger.Error("Gamestate delta extraction canceled", "error", ctx.Err(), "game_state_id", gs.ID.String(), "attempt", attempt)
			return nil
		case errors.Is(deltaErr, context.DeadlineExceeded):
			// Attempt already used llmRequestTimeout; HTTP client has the same budget.
			p.logger.Error("Gamestate delta extraction timed out", "error", deltaErr, "game_state_id", gs.ID.String(), "attempt", attempt)
			return nil
		case deltaErr != nil && attempt < maxAttempts:
			p.logger.Warn("Gamestate delta extraction failed, will retry", "error", deltaErr, "game_state_id", gs.ID.String(), "attempt", attempt)
			continue
		case deltaErr != nil:
			p.logger.Error("Failed to get meta extraction response from LLM after retries", "error", deltaErr, "game_state_id", gs.ID.String(), "attempts", maxAttempts)
			return nil
		}

		p.logger.Info("llm usage",
			"type", "reducer",
			"game_state_id", gs.ID,
			"principal_id", gs.PrincipalID,
			"usage", usage,
		)

		p.logger.Debug("Received gamestate delta from LLM", "game_state_id", gs.ID.String(), "delta", delta, "backend_model", usage.Model)
		break
	}

	if delta == nil {
		return nil
	}

	latestGS, err := p.storage.LoadGameState(ctx, gs.ID)
	if err != nil {
		p.logger.Error("Failed to load latest game state for gamestate delta", "error", err, "game_state_id", gs.ID.String())
		return nil
	}
	if latestGS == nil {
		p.logger.Warn("Game state not found during gamestate delta", "game_state_id", gs.ID.String())
		return nil
	}

	// Increment turn counters on the latest game state
	if !latestGS.IsEnded {
		latestGS.IncrementTurnCounters()
	}

	applier := state.NewApplier(latestGS, delta, s, p.logger).
		WithQueue(p.chatQueue).
		WithStorage(p.storage).
		WithContext(ctx)

	applier.ApplyVars()
	if err := applier.Apply(); err != nil {
		p.logger.Error("Failed to apply initial delta", "error", err, "game_state_id", latestGS.ID.String())
		return nil
	}

	p.applyConditionalsCascade(applier, latestGS.ID)

	// Save the updated game state
	if err := p.storage.UpdateGameState(ctx, latestGS.ID, latestGS); err != nil {
		p.logger.Error("Failed to save updated game state after meta extraction", "error", err, "game_state_id", latestGS.ID.String())
		return nil
	}

	p.logger.Debug("Updated game meta",
		"game_state_id", gs.ID.String(),
		"delta", delta,
		"duration_s", time.Since(start).Seconds(),
		"backend_model", usage.Model,
	)

	return latestGS
}

// applyConditionalsCascade recursively evaluates and applies conditionals until none trigger
func (p *ChatOrchestrator) applyConditionalsCascade(applier *state.Applier, gameStateID uuid.UUID) {
	const maxConditionalIterations = 10
	allTriggeredConditionals := make(map[string]bool) // Track all triggered conditional IDs

	for iteration := range maxConditionalIterations {
		// Evaluate conditionals based on current game state
		triggeredConditionals := applier.MergeConditionals()

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
		applier.ApplyVars()

		if err := applier.Apply(); err != nil {
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
func (p *ChatOrchestrator) GetGameState(ctx context.Context, gameStateID uuid.UUID) (*state.GameState, error) {
	gs, err := p.storage.LoadGameState(ctx, gameStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load game state: %w", err)
	}
	if gs == nil {
		return nil, fmt.Errorf("game state not found: %s", gameStateID.String())
	}
	return gs, nil
}

func (p *ChatOrchestrator) enqueueReaction(ctx context.Context, gs *state.GameState, pending *chat.Ruling) {
	if pending == nil || p.chatQueue == nil || gs == nil {
		return
	}
	if ctx.Err() != nil || gs.IsEnded {
		return
	}
	if !actorPresentAtLocation(gs, pending.Subject) {
		return
	}
	req := &queue.Request{
		RequestID:   uuid.New().String(),
		Type:        queue.RequestTypeStoryEvent,
		GameStateID: gs.ID,
		EventPrompt: fmt.Sprintf("%s strikes %s.", pending.Subject, pending.Object),
		Ruling:      pending,
		EnqueuedAt:  time.Now(),
	}
	if err := p.chatQueue.EnqueueRequest(ctx, req); err != nil {
		p.logger.Error("Failed to enqueue reaction",
			"error", err,
			"game_state_id", gs.ID.String(),
			"subject", pending.Subject,
			"object", pending.Object,
		)
		return
	}
	p.logger.Info("Reaction enqueued",
		"game_state_id", gs.ID.String(),
		"request_id", req.RequestID,
		"subject", pending.Subject,
		"object", pending.Object,
	)
}
