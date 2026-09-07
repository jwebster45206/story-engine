package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jwebster45206/story-engine/pkg/chat"
)

func (m ConsoleUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Stream lifecycle must run even while a modal is open, or a dropped
	// connection during quit/new-game confirm would never reconnect.
	switch msg := msg.(type) {
	case sseDisconnectedMsg:
		return m.handleSSEDisconnected(msg)
	case sseReconnectMsg:
		return m.handleSSEReconnect(msg)
	}

	// Handle scenario modal first
	if m.showScenarioModal {
		return m.updateScenarioModal(msg)
	}

	// Handle PC modal second
	if m.showPCModal {
		return m.updatePCModal(msg)
	}

	// Handle play style modal third
	if m.showPlayStyleModal {
		return m.updatePlayStyleModal(msg)
	}

	// Handle provider modal fourth
	if m.showProviderModal {
		return m.updateProviderModal(msg)
	}

	// Handle quit modal
	if m.showQuitModal {
		return m.updateQuitModal(msg)
	}

	// Handle new game modal
	if m.showNewGameModal {
		return m.updateNewGameModal(msg)
	}

	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		mvCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Only update chat UI if we're not showing the modal
		if !m.showScenarioModal {
			chatWidth := int(float64(m.width)*0.75) - 4
			metaWidth := m.width - chatWidth - 6

			// Update viewport dimensions
			m.chatViewport.Width = chatWidth - 2
			m.chatViewport.Height = m.height - 7 // Reduced by 1 for spacing
			m.metaViewport.Width = metaWidth - 2
			m.metaViewport.Height = m.height - 4
			m.textarea.SetWidth(chatWidth - 4)

			if !m.ready {
				m.ready = true
				// Initial content setup
				m.writeChatContent()
			} else {
				// Window was resized - reformat all content for new width
				m.writeChatContent()
			}

			// Update metadata panel content as well
			if m.gameState != nil {
				m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))
			}
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.err = nil // Clear any stale errors when opening quit modal
			m.showQuitModal = true
			return m, nil

		case tea.KeyCtrlN:
			// Show new game confirmation modal
			m.err = nil // Clear any stale errors when opening new game modal
			m.showNewGameModal = true
			return m, nil

		case tea.KeyCtrlY:
			// Copy GameState ID to system clipboard (assume non-nil per user instruction)
			if m.gameState != nil {
				_ = clipboard.WriteAll(m.gameState.ID.String())
				// Optionally append a tiny notice to metadata (non-intrusive)
				m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))
			}
			return m, nil

		case tea.KeyCtrlZ:
			// Clear the text area
			m.textarea.Reset()
			return m, nil

		case tea.KeyCtrlE:
			// Export chat history to markdown
			return m.handleExport()

		case tea.KeyCtrlS:
			// Save game state JSON to working directory
			return m.handleSave()

		case tea.KeyCtrlR:
			// Fetch fresh game state from server, then force a full chat re-render
			if m.gameState != nil {
				m.forceRerender = true
				return m, m.refreshGameState()
			}
			return m, nil

		case tea.KeyEnter:
			if m.loading || m.isStreaming {
				return m, nil
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			// Clear any previous errors when attempting a new chat
			m.err = nil

			// Prevent multiple messages after game end
			if m.gameState != nil && m.gameState.IsEnded && m.finalMessageSent {
				return m, nil
			}

			m.textarea.Reset()
			m.loading = true
			m.progressTick = 0   // Reset progress animation
			m.userPinned = false // user intent to append at bottom

			// Mark that we've sent the final message if game is ended
			if m.gameState != nil && m.gameState.IsEnded {
				m.finalMessageSent = true
			}

			// Don't add user message here - it will be added when we receive the request.processing event
			// This allows us to handle external chat messages as well

			return m, tea.Batch(m.sendChatMessage(input), progressTick())
		}

		// scrolling/navigation keys for the chat viewport
		keyStr := msg.String()
		if keyStr == "up" || keyStr == "down" || keyStr == "pgup" || keyStr == "pgdown" || keyStr == "home" || keyStr == "end" {
			prevAtBottom := m.chatViewport.AtBottom()
			prevOffset := m.chatViewport.YOffset
			m.chatViewport, vpCmd = m.chatViewport.Update(msg)
			if m.chatViewport.YOffset != prevOffset { // user navigated
				if !m.chatViewport.AtBottom() {
					m.userPinned = true
				} else if !prevAtBottom && m.chatViewport.AtBottom() {
					m.userPinned = false
				}
			}
		}

	case tea.MouseMsg:
		// Route mouse scroll wheel to the chat viewport only; sidebar is intentionally locked.
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.chatViewport.ScrollUp(3)
			if !m.chatViewport.AtBottom() {
				m.userPinned = true
			}
		case tea.MouseButtonWheelDown:
			prevAtBottom := m.chatViewport.AtBottom()
			m.chatViewport.ScrollDown(3)
			if !prevAtBottom && m.chatViewport.AtBottom() {
				m.userPinned = false
			}
		}
		// Pass mouse events to textarea (handles click-to-focus etc.) but NOT to metaViewport.
		m.textarea, tiCmd = m.textarea.Update(msg)
		return m, tiCmd

	case chatErrorMsg:
		m.loading = false
		m.err = msg.err

		// Remove the failed user message from pending messages and chat history
		if len(m.pendingUserMessages) > 0 {
			m.pendingUserMessages = m.pendingUserMessages[:len(m.pendingUserMessages)-1]
		}
		if len(m.gameState.ChatHistory) > 0 {
			lastMsg := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
			if lastMsg.Role == "user" {
				m.gameState.ChatHistory = m.gameState.ChatHistory[:len(m.gameState.ChatHistory)-1]
			}
		}

		// Add error message as a system message in chat history
		errorMessage := chat.ChatMessage{
			Role:    "system",
			Content: errorStyle.Render("Error: " + msg.err.Error()),
		}
		m.gameState.ChatHistory = append(m.gameState.ChatHistory, errorMessage)

		// Rewrite chat content with the error message included
		m.writeChatContent()
		if !m.userPinned {
			m.chatViewport.GotoBottom()
		}
		return m, nil

	case pollTickMsg:
		// Don't poll if the game has ended
		if m.gameState != nil && m.gameState.IsEnded {
			return m, nil
		}

		// Time to initiate a poll (if we have a game state and are actively waiting for updates)
		if m.gameState != nil && m.pollingActive {
			if m.pollInFlight {
				// Start a fresh poll by bumping the sequence; result from older poll will be ignored when it arrives
				m.pollSeq++
				m.activePollSeq = m.pollSeq
				m.pollInFlight = true
				return m, tea.Batch(startPoll(m.activePollSeq, m.client, m.config.APIBaseURL, m.gameState.ID), schedulePoll())
			}
			// No poll in flight; start one
			m.pollSeq++
			m.activePollSeq = m.pollSeq
			m.pollInFlight = true
			return m, tea.Batch(startPoll(m.activePollSeq, m.client, m.config.APIBaseURL, m.gameState.ID), schedulePoll())
		} else if m.gameState != nil {
			// We have a game state but aren't actively waiting - reschedule with a longer interval
			return m, tea.Tick(30*time.Second, func(time.Time) tea.Msg { return pollTickMsg{} })
		}
		// No game state yet; just reschedule
		return m, schedulePoll()

	case pollResultMsg:
		// Only apply if this is the latest active sequence
		if msg.seq == m.activePollSeq {
			m.pollInFlight = false
			if msg.err == nil && msg.gameState != nil && m.gameState != nil {
				// Check if the game has ended and stop polling
				if msg.gameState.IsEnded {
					m.pollingActive = false
					m.mergeServerGameState(msg.gameState)
					m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))
				} else if m.pollingActive && msg.gameState.UpdatedAt.After(m.pollingStartedAt) {
					// Check if we got an updated timestamp and should stop active polling
					m.pollingActive = false
					// Apply the full updated gamestate
					m.mergeServerGameState(msg.gameState)
					m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))
				} else {
					// Just refresh metadata fields to avoid reordering chat mid-turn
					m.gameState.ID = msg.gameState.ID
					m.gameState.ModelName = msg.gameState.ModelName
					m.gameState.Scenario = msg.gameState.Scenario
					m.gameState.SceneName = msg.gameState.SceneName
					m.gameState.NPCs = msg.gameState.NPCs
					m.gameState.WorldLocations = msg.gameState.WorldLocations
					m.gameState.Location = msg.gameState.Location
					m.gameState.Inventory = msg.gameState.Inventory
					m.gameState.TurnCounter = msg.gameState.TurnCounter
					m.gameState.SceneTurnCounter = msg.gameState.SceneTurnCounter
					m.gameState.Vars = msg.gameState.Vars
					m.gameState.IsEnded = msg.gameState.IsEnded
					m.gameState.ContingencyPrompts = msg.gameState.ContingencyPrompts
					m.gameState.UpdatedAt = msg.gameState.UpdatedAt
					m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))
				}
			}
		}
		return m, nil

	case sseEventMsg:
		if msg.event.GameID != m.sseGameID {
			return m, nil
		}
		// Handle SSE events from the async request processing
		switch msg.event.Type {
		case "request.processing":
			// Request has been picked up by worker - can stop showing progress bar
			m.loading = false

			// Add the user message from the event data (if present)
			if userMsg, ok := msg.event.Data["user_message"].(string); ok && userMsg != "" {
				userMessage := chat.ChatMessage{
					Role:    "user",
					Content: userMsg,
				}
				m.gameState.ChatHistory = append(m.gameState.ChatHistory, userMessage)
				m.pendingUserMessages = append(m.pendingUserMessages, userMessage)

				// Reformat content to include the new user message
				m.writeChatContent()
				if !m.userPinned {
					m.chatViewport.GotoBottom()
				}
			}

		case "chat.chunk":
			// Get content from the data map
			content, ok := msg.event.Data["content"].(string)
			if ok {
				// If we're not currently streaming, start a new assistant message
				if !m.isStreaming {
					m.isStreaming = true
					m.streamingContent = ""
					assistantMessage := chat.ChatMessage{Role: "assistant", Content: ""}
					m.gameState.ChatHistory = append(m.gameState.ChatHistory, assistantMessage)
					m.streamingMessageIdx = len(m.gameState.ChatHistory) - 1
				}

				// Append content to current streaming response
				m.streamingContent += content
				if m.streamingMessageIdx < len(m.gameState.ChatHistory) {
					m.gameState.ChatHistory[m.streamingMessageIdx].Content = m.streamingContent
				}
				// Refresh display with new content
				m.writeChatContent()
				if !m.userPinned {
					m.chatViewport.GotoBottom()
				}
			}

		case "request.completed":
			// Streaming complete
			m.isStreaming = false
			m.loading = false

			// Start polling now that request is complete (only if game hasn't ended)
			var startPollingCmd tea.Cmd
			if m.gameState != nil && !m.gameState.IsEnded {
				wasPollingActive := m.pollingActive
				m.pollingActive = true
				m.pollingStartedAt = time.Now()

				if !wasPollingActive {
					startPollingCmd = schedulePoll()
				}
			}

			// Update metadata to show polling indicator
			m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))

			// Continue consuming SSE events while also refreshing gamestate
			var sseCmd tea.Cmd
			if m.eventChan != nil {
				sseCmd = m.consumeSSEEvents(m.eventChan)
			}
			return m, tea.Batch(m.refreshGameState(), startPollingCmd, sseCmd)

		case "request.failed":
			// Request failed
			m.isStreaming = false
			m.loading = false

			// Get error message from the data map
			errorMsg := "Request failed"
			if errStr, ok := msg.event.Data["error"].(string); ok {
				errorMsg = errStr
			}

			// Remove the failed user message from pending messages and chat history
			if len(m.pendingUserMessages) > 0 {
				m.pendingUserMessages = m.pendingUserMessages[:len(m.pendingUserMessages)-1]
			}
			if len(m.gameState.ChatHistory) > 0 {
				lastMsg := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
				if lastMsg.Role == "user" {
					m.gameState.ChatHistory = m.gameState.ChatHistory[:len(m.gameState.ChatHistory)-1]
				}
			}

			// Add error message as a system message in chat history
			errorMessage := chat.ChatMessage{
				Role:    "system",
				Content: errorStyle.Render("Error: " + errorMsg),
			}
			m.gameState.ChatHistory = append(m.gameState.ChatHistory, errorMessage)

			// Rewrite chat content with the error message included
			m.writeChatContent()
			if !m.userPinned {
				m.chatViewport.GotoBottom()
			}
		}

		// Continue consuming SSE events
		if m.eventChan != nil {
			return m, m.consumeSSEEvents(m.eventChan)
		}
		return m, nil

	case gameStateMsg:
		if msg.err == nil && msg.gameState != nil {
			m.mergeServerGameState(msg.gameState)
			m.metaViewport.SetContent(writeSidebar(m.gameState, m.scenarioDisplayName(), m.pollingActive))
			if m.forceRerender {
				m.forceRerender = false
				m.writeChatContent()
				m.chatViewport.GotoBottom()
			}
		}

	case progressTickMsg:
		if m.loading {
			m.progressTick++
			m.writeChatContent()     // Refresh the chat content to update the progress bar
			return m, progressTick() // Continue the animation
		}
	}

	// Update components for non-mouse events (textarea & metadata). Chat viewport already handled for key scroll above.
	// Mouse events are handled above and return early, so metaViewport.Update is never called for them.
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.metaViewport, mvCmd = m.metaViewport.Update(msg)
	m.metaViewport.YOffset = 0 // keep sidebar locked at top
	return m, tea.Batch(tiCmd, vpCmd, mvCmd)
}

func (m ConsoleUI) sendChatMessage(message string) tea.Cmd {
	return func() tea.Msg {
		// Use the async endpoint - it will return immediately with a request_id
		// and we'll receive updates via SSE
		_, err := sendChatAsync(m.client, m.config.APIBaseURL, m.gameState.ID, message)
		if err != nil {
			return chatErrorMsg{err: fmt.Errorf("failed to send chat message: %w", err)}
		}
		// Return nil - we'll get updates via SSE
		return nil
	}
}
func (m ConsoleUI) refreshGameState() tea.Cmd {
	return func() tea.Msg {
		gs, err := getGameState(m.client, m.config.APIBaseURL, m.gameState.ID)
		return gameStateMsg{gs, err}
	}
}

func (m ConsoleUI) consumeSSEEvents(events <-chan SSEEvent) tea.Cmd {
	gameID := m.sseGameID
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return sseDisconnectedMsg{gameID: gameID}
		}
		return sseEventMsg{event: event}
	}
}

func (m ConsoleUI) handleSSEDisconnected(msg sseDisconnectedMsg) (tea.Model, tea.Cmd) {
	if msg.gameID != m.sseGameID || m.gameState == nil || m.gameState.ID != msg.gameID {
		return m, nil
	}
	wasWaiting := m.loading || m.isStreaming
	m.loading = false
	m.isStreaming = false
	if wasWaiting {
		m.gameState.ChatHistory = append(m.gameState.ChatHistory, chat.ChatMessage{
			Role:    "system",
			Content: errorStyle.Render("Error: lost connection to event stream; try sending again"),
		})
		m.writeChatContent()
		if !m.userPinned {
			m.chatViewport.GotoBottom()
		}
	}
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return sseReconnectMsg(msg)
	})
}

func (m ConsoleUI) handleSSEReconnect(msg sseReconnectMsg) (tea.Model, tea.Cmd) {
	if m.gameState == nil || m.gameState.ID != msg.gameID {
		return m, nil
	}
	return m, m.startSSE()
}

func (m ConsoleUI) loadScenarios() tea.Cmd {
	return func() tea.Msg {
		orderedNames, scenarioMap, err := listScenarios(m.client, m.config.APIBaseURL)
		return scenariosLoadedMsg{orderedNames, scenarioMap, err}
	}
}

func (m ConsoleUI) loadPCs() tea.Cmd {
	return func() tea.Msg {
		orderedNames, pcMap, err := listPCs(m.client, m.config.APIBaseURL)
		return pcsLoadedMsg{orderedNames, pcMap, err}
	}
}

func (m ConsoleUI) createGameStateFromScenario(scenarioFile string, pcID string, provider string, rules string, temperature float64) tea.Cmd {
	return func() tea.Msg {
		gs, err := createGameState(m.client, m.config.APIBaseURL, scenarioFile, pcID, provider, rules, temperature)
		return gameStateCreatedMsg{gs, err}
	}
}

func (m ConsoleUI) loadProviders() tea.Cmd {
	return func() tea.Msg {
		def, providers, err := getProviders(m.client, m.config.APIBaseURL)
		return providersLoadedMsg{def, providers, err}
	}
}

// progressTick creates a command that sends a progress tick message
func progressTick() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(time.Time) tea.Msg {
		return progressTickMsg{}
	})
}

// schedulePoll returns a command that triggers a pollTickMsg after the interval
func schedulePoll() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg { return pollTickMsg{} })
}

// startPoll begins an HTTP fetch for the latest game state; old sequences are ignored
func startPoll(seq int, client *http.Client, baseURL string, id uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		gs, err := getGameState(client, baseURL, id)
		return pollResultMsg{seq: seq, gameState: gs, err: err}
	}
}
