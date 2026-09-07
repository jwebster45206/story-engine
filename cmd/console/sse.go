package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"uuid"

	tea "github.com/charmbracelet/bubbletea"
)

// SSEEvent represents an event from the SSE stream
type SSEEvent struct {
	Type   string         `json:"type"`
	Data   map[string]any `json:"data"`
	GameID uuid.UUID
}

func (m *ConsoleUI) stopSSE() {
	if m.sseCancel != nil {
		m.sseCancel()
		m.sseCancel = nil
	}
	m.eventChan = nil
	m.sseGameID = uuid.Nil()
}

// startSSE opens a new event stream for the current game and returns a command
// that consumes it. stopSSE is called first so a prior listener is cancelled.
func (m *ConsoleUI) startSSE() tea.Cmd {
	if m.gameState == nil || m.client == nil || m.config == nil {
		return nil
	}
	m.stopSSE()
	ctx, cancel := context.WithCancel(context.Background())
	m.sseCancel = cancel
	id := m.gameState.ID
	m.sseGameID = id
	eventChan := make(chan SSEEvent, 10)
	m.eventChan = eventChan
	client := m.client
	baseURL := m.config.APIBaseURL
	go func() {
		_ = listenToSSE(ctx, client, baseURL, id, eventChan)
		close(eventChan)
	}()
	return m.consumeSSEEvents(eventChan)
}

// feedSSELine consumes one SSE wire line. A blank line completes the current
// event when it has a type. Comment lines (keepalive) are ignored.
func feedSSELine(current SSEEvent, line string) (SSEEvent, *SSEEvent) {
	if line == "" {
		if current.Type != "" {
			done := current
			return SSEEvent{}, &done
		}
		return current, nil
	}
	if eventType, ok := strings.CutPrefix(line, "event: "); ok {
		current.Type = eventType
		return current, nil
	}
	if dataJSON, ok := strings.CutPrefix(line, "data: "); ok {
		var data map[string]any
		if err := json.Unmarshal([]byte(dataJSON), &data); err == nil {
			current.Data = data
		}
		return current, nil
	}
	return current, nil
}

// listenToSSE connects to the SSE endpoint and streams events to a channel
func listenToSSE(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, eventChan chan<- SSEEvent) error {
	url := fmt.Sprintf("%s/v1/events/gamestate/%s", baseURL, gameStateID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("SSE connection failed with status %d: %s", resp.StatusCode, string(body))
	}

	return parseSSEStream(ctx, resp.Body, gameStateID, eventChan)
}

func parseSSEStream(ctx context.Context, r io.Reader, gameStateID uuid.UUID, eventChan chan<- SSEEvent) error {
	scanner := bufio.NewScanner(r)
	var current SSEEvent

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		next, complete := feedSSELine(current, scanner.Text())
		if complete != nil {
			complete.GameID = gameStateID
			select {
			case <-ctx.Done():
				return ctx.Err()
			case eventChan <- *complete:
			}
		}
		current = next
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SSE stream: %w", err)
	}

	return nil
}
