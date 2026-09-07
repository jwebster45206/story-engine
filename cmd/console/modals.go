package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m ConsoleUI) updateScenarioModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case scenariosLoadedMsg:
		m.loadingScenarios = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.scenarios = msg.scenarios
			m.scenarioMap = msg.scenarioMap
		}

	case tea.KeyMsg:
		if m.loadingScenarios {
			if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.err != nil {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.showQuitModal = true
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			m.showQuitModal = true
			return m, nil
		case tea.KeyUp:
			if m.selectedScenario > 0 {
				m.selectedScenario--
			}
		case tea.KeyDown:
			if m.selectedScenario < len(m.scenarios)-1 {
				m.selectedScenario++
			}
		case tea.KeyEnter:
			if len(m.scenarios) > 0 {
				scenarioName := m.scenarios[m.selectedScenario]
				scenarioFile := m.scenarioMap[scenarioName]
				// First fetch scenario details to get the content rating
				s, err := getScenario(m.client, m.config.APIBaseURL, scenarioFile)
				if err != nil {
					m.err = fmt.Errorf("failed to fetch scenario details: %w", err)
					return m, nil
				}
				m.contentRating = s.Rating
				m.selectedScenarioFile = scenarioFile
				// Store the default PC ID from the scenario
				m.defaultPCID = s.DefaultPC
				// Transition to PC selection modal
				m.showScenarioModal = false
				m.showPCModal = true
				m.loadingPCs = true
				return m, m.loadPCs()
			}
		}
	}

	return m, nil
}

func (m ConsoleUI) updatePCModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case pcsLoadedMsg:
		m.loadingPCs = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.pcs = msg.pcs
			m.pcMap = msg.pcMap
			// Auto-select the default PC if available
			m.selectedPC = m.findDefaultPCIndex()
		}

	case gameStateCreatedMsg:
		return m.handleGameStateCreated(msg)

	case tea.KeyMsg:
		if m.loadingPCs {
			if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.err != nil {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.showQuitModal = true
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			// Go back to scenario selection
			m.showPCModal = false
			m.showScenarioModal = true
			m.pcs = nil
			m.pcMap = nil
			m.selectedPC = 0
			m.defaultPCID = ""
			m.selectedPCID = ""
			m.err = nil
			return m, nil
		case tea.KeyUp:
			if m.selectedPC > 0 {
				m.selectedPC--
			}
		case tea.KeyDown:
			if m.selectedPC < len(m.pcs)-1 {
				m.selectedPC++
			}
		case tea.KeyEnter:
			if len(m.pcs) > 0 {
				pcName := m.pcs[m.selectedPC]
				m.selectedPCID = m.pcMap[pcName]
				m.showPCModal = false
				m.showPlayStyleModal = true
				m.selectedPlayStyle = defaultPlayStyleIndex
				return m, nil
			}
		}
	}

	return m, nil
}

func (m ConsoleUI) handleGameStateCreated(msg gameStateCreatedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.err = msg.err
		return m, textarea.Blink
	}

	m.gameState = msg.gameState
	m.showPlayStyleModal = false
	m.showPCModal = false
	m.showProviderModal = false
	m.loadingProviders = false
	// Set up viewport dimensions now that we have a game state
	if m.width > 0 && m.height > 0 {
		chatWidth := int(float64(m.width)*0.75) - 4
		metaWidth := m.width - chatWidth - 6
		m.chatViewport.Width = chatWidth - 2
		m.chatViewport.Height = m.height - 7
		m.metaViewport.Width = metaWidth - 2
		m.metaViewport.Height = m.height - 4
		m.textarea.SetWidth(chatWidth - 4)
	}
	// Use display name instead of raw file name
	m.chatViewport.SetContent(writeInitialContent(m.gameState, m.scenarioDisplayName(), m.chatViewport.Width-6))
	m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive, m.chatLatencies))
	m.textarea.Focus() // Ensure textarea gets focus when modal closes
	m.ready = true

	// Start SSE listener for this game
	eventChan := make(chan SSEEvent, 10)
	m.eventChan = eventChan
	go func() {
		ctx := context.Background()
		// listenToSSE blocks until connection closes or error occurs
		// When it returns, just close the channel gracefully
		_ = listenToSSE(ctx, m.client, m.config.APIBaseURL, m.gameState.ID, eventChan)
		close(eventChan)
	}()
	return m, tea.Batch(textarea.Blink, m.consumeSSEEvents(eventChan))
}

func (m ConsoleUI) updatePlayStyleModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case gameStateCreatedMsg:
		return m.handleGameStateCreated(msg)

	case tea.KeyMsg:
		if m.loading {
			if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.err != nil {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.showQuitModal = true
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			m.showPlayStyleModal = false
			m.showPCModal = true
			m.err = nil
			return m, nil
		case tea.KeyUp:
			if m.selectedPlayStyle > 0 {
				m.selectedPlayStyle--
			}
		case tea.KeyDown:
			if m.selectedPlayStyle < len(playStyles)-1 {
				m.selectedPlayStyle++
			}
		case tea.KeyEnter:
			style := playStyles[m.selectedPlayStyle]
			m.selectedRules = string(style.rules)
			m.selectedTemp = style.temperature
			m.showPlayStyleModal = false
			m.showProviderModal = true
			m.loadingProviders = true
			m.err = nil
			return m, m.loadProviders()
		}
	}

	return m, nil
}

func (m ConsoleUI) updateProviderModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case providersLoadedMsg:
		m.loadingProviders = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.providers = msg.providers
		m.defaultProvider = msg.defaultProvider
		m.selectedProvider = 0
		for i, p := range m.providers {
			if p.Name == m.defaultProvider {
				m.selectedProvider = i
				break
			}
		}
		// Skip the picker when only one provider is configured, but keep the
		// modal flag set so gameStateCreatedMsg is still routed here.
		if len(m.providers) <= 1 {
			providerID := m.defaultProvider
			if len(m.providers) == 1 {
				providerID = m.providers[0].Name
			}
			m.selectedProviderID = providerID
			m.loading = true
			return m, m.createGameStateFromScenario(
				m.selectedScenarioFile,
				m.selectedPCID,
				providerID,
				m.selectedRules,
				m.selectedTemp,
			)
		}

	case gameStateCreatedMsg:
		return m.handleGameStateCreated(msg)

	case tea.KeyMsg:
		if m.loadingProviders || m.loading {
			if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.err != nil {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.showProviderModal = false
				m.showPlayStyleModal = true
				m.err = nil
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			m.showProviderModal = false
			m.showPlayStyleModal = true
			m.err = nil
			return m, nil
		case tea.KeyUp:
			if m.selectedProvider > 0 {
				m.selectedProvider--
			}
		case tea.KeyDown:
			if m.selectedProvider < len(m.providers)-1 {
				m.selectedProvider++
			}
		case tea.KeyEnter:
			if len(m.providers) == 0 {
				return m, nil
			}
			m.selectedProviderID = m.providers[m.selectedProvider].Name
			m.loading = true
			return m, m.createGameStateFromScenario(
				m.selectedScenarioFile,
				m.selectedPCID,
				m.selectedProviderID,
				m.selectedRules,
				m.selectedTemp,
			)
		}
	}

	return m, nil
}

func (m ConsoleUI) updateQuitModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			return m, tea.Quit
		default:
			switch msg.String() {
			case "y", "Y":
				return m, tea.Quit
			case "n", "N":
				m.showQuitModal = false
				m.err = nil // Clear any stale errors when canceling quit
				// Return focus to the appropriate component
				if m.showScenarioModal {
					// We're in scenario selection, no need to focus textarea
					return m, nil
				} else {
					// We're in the main game, focus the textarea
					m.textarea.Focus()
					return m, textarea.Blink
				}
			}
		}
	}

	return m, nil
}

// updateNewGameModal handles confirmation for starting a new game
func (m ConsoleUI) updateNewGameModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.showNewGameModal = false
			m.err = nil // Clear any stale errors when canceling new game
			return m, nil
		case tea.KeyEnter:
			return m.startNewGame()
		default:
			switch msg.String() {
			case "y", "Y":
				return m.startNewGame()
			case "n", "N":
				m.showNewGameModal = false
				m.err = nil // Clear any stale errors when canceling new game
				return m, nil
			}
		}
	}
	return m, nil
}

// startNewGame resets state and returns to scenario selection, reloading scenarios
func (m *ConsoleUI) startNewGame() (tea.Model, tea.Cmd) {
	m.gameState = nil
	m.pendingUserMessages = nil
	m.chatViewport.SetContent("")
	m.metaViewport.SetContent("")
	m.textarea.Reset() // Clear the text area
	m.showNewGameModal = false
	m.showScenarioModal = true
	m.loadingScenarios = true
	m.scenarios = nil
	m.scenarioMap = nil
	m.selectedScenario = 0
	// Reset PC selection state
	m.showPCModal = false
	m.pcs = nil
	m.pcMap = nil
	m.selectedPC = 0
	m.loadingPCs = false
	m.selectedScenarioFile = ""
	m.defaultPCID = ""
	m.selectedPCID = ""
	// Reset play style selection state
	m.showPlayStyleModal = false
	m.selectedPlayStyle = defaultPlayStyleIndex
	// Reset provider selection state
	m.showProviderModal = false
	m.providers = nil
	m.selectedProvider = 0
	m.loadingProviders = false
	m.defaultProvider = ""
	m.selectedProviderID = ""
	m.selectedRules = ""
	m.selectedTemp = 0
	// Reset polling state
	m.pollSeq = 0
	m.activePollSeq = 0
	m.pollInFlight = false
	m.pollingActive = false
	m.pollingStartedAt = time.Time{}
	m.finalMessageSent = false
	// Reset latency tracking
	m.lastChatLatency = 0
	m.chatLatencies = nil
	m.chatRequestStartTime = time.Time{}
	m.err = nil // Clear any stale errors when starting new game
	return m, m.loadScenarios()
}

func (m ConsoleUI) renderNewGameModal() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	var content strings.Builder
	content.WriteString(modalTitleStyle.Render("Start a New Game?"))
	content.WriteString("\n\n")
	content.WriteString("This will discard the current session and return to scenario selection.")
	content.WriteString("\n\n")
	content.WriteString(promptStyle.Render("Press Y to confirm, N to cancel, or Esc to go back"))
	modal := modalStyle.Width(58).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}

func (m ConsoleUI) renderQuitModal() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder
	content.WriteString(modalTitleStyle.Render("Quit Game?"))
	content.WriteString("\n\n")
	content.WriteString("Are you sure you want to quit your adventure?")
	content.WriteString("\n\n")
	content.WriteString(promptStyle.Render("Press Y to quit, N to continue, or Ctrl+C to force quit"))

	// Create the modal
	modal := modalStyle.Width(50).Render(content.String())

	// Center the modal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}

func (m ConsoleUI) renderScenarioModal() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder

	if m.loadingScenarios {
		content.WriteString(modalTitleStyle.Render("Loading Scenarios..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Please wait while we fetch available scenarios..."))
	} else if m.err != nil {
		content.WriteString(modalTitleStyle.Render("Error"))
		content.WriteString("\n\n")
		content.WriteString(errorStyle.Render(fmt.Sprintf("Failed to load scenarios: %v", m.err)))
		content.WriteString("\n\n")
		content.WriteString("Press Ctrl+C to force quit, Esc to confirm quit")
	} else if m.loading {
		content.WriteString(modalTitleStyle.Render("Creating Game..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Setting up your adventure..."))
	} else {
		content.WriteString(modalTitleStyle.Render("Select a Scenario"))
		content.WriteString("\n\n")

		for i, scenario := range m.scenarios {
			if i == m.selectedScenario {
				content.WriteString(modalSelectedItemStyle.Render(fmt.Sprintf("▶ %s", scenario)))
			} else {
				content.WriteString(modalItemStyle.Render(fmt.Sprintf("  %s", scenario)))
			}
			content.WriteString("\n")
		}

		content.WriteString("\n")
		content.WriteString(promptStyle.Render("Use ↑/↓ to navigate, Enter to select, Ctrl+C to force quit"))
	}

	// Create the modal
	modal := modalStyle.Width(60).Render(content.String())

	// Center the modal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}

func (m ConsoleUI) renderPCModal() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder

	if m.loadingPCs {
		content.WriteString(modalTitleStyle.Render("Loading Characters..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Please wait while we fetch available characters..."))
	} else if m.err != nil {
		content.WriteString(modalTitleStyle.Render("Error"))
		content.WriteString("\n\n")
		content.WriteString(errorStyle.Render(fmt.Sprintf("Failed to load characters: %v", m.err)))
		content.WriteString("\n\n")
		content.WriteString("Press Ctrl+C to force quit, Esc to go back")
	} else if m.loading {
		content.WriteString(modalTitleStyle.Render("Creating Game..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Setting up your adventure..."))
	} else {
		content.WriteString(modalTitleStyle.Render("Select Your Character"))
		content.WriteString("\n\n")

		for i, pc := range m.pcs {
			if i == m.selectedPC {
				content.WriteString(modalSelectedItemStyle.Render(fmt.Sprintf("▶ %s", pc)))
			} else {
				content.WriteString(modalItemStyle.Render(fmt.Sprintf("  %s", pc)))
			}
			content.WriteString("\n")
		}

		content.WriteString("\n")
		content.WriteString(promptStyle.Render("Use ↑/↓ to navigate, Enter to select, Esc to go back"))
	}

	// Create the modal
	modal := modalStyle.Width(60).Render(content.String())

	// Center the modal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}

func (m ConsoleUI) renderPlayStyleModal() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder

	if m.err != nil {
		content.WriteString(modalTitleStyle.Render("Error"))
		content.WriteString("\n\n")
		content.WriteString(errorStyle.Render(fmt.Sprintf("Failed to create game: %v", m.err)))
		content.WriteString("\n\n")
		content.WriteString("Press Ctrl+C to force quit, Esc to go back")
	} else if m.loading {
		content.WriteString(modalTitleStyle.Render("Creating Game..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Setting up your adventure..."))
	} else {
		content.WriteString(modalTitleStyle.Render("Select Play Style"))
		content.WriteString("\n\n")

		for i, style := range playStyles {
			if i == m.selectedPlayStyle {
				content.WriteString(modalSelectedItemStyle.Render(fmt.Sprintf("▶ %s", style.label)))
			} else {
				content.WriteString(modalItemStyle.Render(fmt.Sprintf("  %s", style.label)))
			}
			content.WriteString("\n")
		}

		content.WriteString("\n")
		content.WriteString(promptStyle.Render("Use ↑/↓ to navigate, Enter to select, Esc to go back"))
	}

	modal := modalStyle.Width(60).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}

func (m ConsoleUI) renderProviderModal() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder

	if m.loadingProviders {
		content.WriteString(modalTitleStyle.Render("Loading Providers..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Please wait while we fetch available providers..."))
	} else if m.err != nil {
		content.WriteString(modalTitleStyle.Render("Error"))
		content.WriteString("\n\n")
		content.WriteString(errorStyle.Render(fmt.Sprintf("Failed to load providers: %v", m.err)))
		content.WriteString("\n\n")
		content.WriteString("Press Ctrl+C to force quit, Esc to go back")
	} else if m.loading {
		content.WriteString(modalTitleStyle.Render("Creating Game..."))
		content.WriteString("\n\n")
		content.WriteString(loadingStyle.Render("Setting up your adventure..."))
	} else {
		content.WriteString(modalTitleStyle.Render("Select API Provider"))
		content.WriteString("\n\n")

		for i, p := range m.providers {
			label := fmt.Sprintf("%s (%s)", p.DisplayName, p.Model)
			if i == m.selectedProvider {
				content.WriteString(modalSelectedItemStyle.Render(fmt.Sprintf("▶ %s", label)))
			} else {
				content.WriteString(modalItemStyle.Render(fmt.Sprintf("  %s", label)))
			}
			content.WriteString("\n")
		}

		content.WriteString("\n")
		content.WriteString(promptStyle.Render("Use ↑/↓ to navigate, Enter to select, Esc to go back"))
	}

	modal := modalStyle.Width(60).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}
