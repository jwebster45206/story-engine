package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"github.com/google/uuid"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/textfilter"
	"github.com/muesli/reflow/wordwrap"
)

const (
	AgentName       = "Narrator"
	PlaceHolderText = "Type your message here...\nExamples: Look around. Get the key. Talk to the guard."
)

// smartWrap wraps text at natural break points including spaces, slashes, and dashes
func smartWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	// For simple cases like URLs, use a simpler approach
	if !strings.Contains(text, " ") {
		// This is likely a URL or similar - split on / and -
		var lines []string
		var currentLine strings.Builder

		for _, char := range text {
			currentLine.WriteRune(char)

			// Check if we should break after certain characters
			if (char == '/' || char == '-') && currentLine.Len() >= width/2 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			} else if currentLine.Len() >= width {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
		}

		if currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}

		return lines
	}

	// For text with spaces, use the existing wordwrap
	wrapped := wordwrap.String(text, width)
	return strings.Split(wrapped, "\n")
}

// ConsoleUI is the BubbleTea model that runs the UI.
// https://github.com/charmbracelet/bubbletea
type ConsoleUI struct {
	config       *ConsoleConfig
	client       *http.Client
	gameState    *state.GameState
	chatViewport viewport.Model
	metaViewport viewport.Model
	textarea     textarea.Model
	ready        bool
	width        int
	height       int
	err          error
	loading      bool

	// Scenario selection state
	showScenarioModal bool
	scenarios         []string
	scenarioMap       map[string]string
	selectedScenario  int
	loadingScenarios  bool
	contentRating     string

	// Profanity filter for family-friendly content
	profanityFilter *textfilter.ProfanityFilter

	// Quit confirmation state
	showQuitModal bool

	// New game confirmation state
	showNewGameModal bool

	// Progress bar state
	progressTick int

	// Auto-scroll suppression
	userPinned bool // true when user has scrolled away from bottom

	// Polling state
	pollSeq          int       // incrementing sequence for polls
	activePollSeq    int       // sequence number of poll in flight
	pollInFlight     bool      // whether a poll HTTP request is active
	pollingActive    bool      // whether we're actively waiting for an updated gamestate
	pollingStartedAt time.Time // timestamp when we started waiting for updates

	// Game ending state
	finalMessageSent bool // whether we've already sent the final message after game end

	// Pending user messages not yet confirmed in server game state
	// Pending user messages awaiting server echo (assistant responses are applied only on chatResponse)
	pendingUserMessages []chat.ChatMessage
}

// mergeServerGameState reconciles the authoritative server game state with any locally
// pending messages (both user and assistant) that have been optimistically appended
// but not yet observed from the server. Matching heuristic: role+content.
// If chat history changes, rewrites chat viewport content while preserving scroll pin.
func (m *ConsoleUI) mergeServerGameState(serverGS *state.GameState) {
	if serverGS == nil {
		return
	}
	origLen := 0
	if m.gameState != nil {
		origLen = len(m.gameState.ChatHistory)
	}
	if m.gameState == nil {
		m.gameState = serverGS
	} else {
		m.gameState.ID = serverGS.ID
		m.gameState.ModelName = serverGS.ModelName
		m.gameState.Scenario = serverGS.Scenario
		m.gameState.SceneName = serverGS.SceneName
		m.gameState.NPCs = serverGS.NPCs
		m.gameState.WorldLocations = serverGS.WorldLocations
		m.gameState.Location = serverGS.Location
		m.gameState.Inventory = serverGS.Inventory
		m.gameState.TurnCounter = serverGS.TurnCounter
		m.gameState.SceneTurnCounter = serverGS.SceneTurnCounter
		m.gameState.Vars = serverGS.Vars
		m.gameState.IsEnded = serverGS.IsEnded
		m.gameState.ContingencyPrompts = serverGS.ContingencyPrompts
		m.gameState.ChatHistory = make([]chat.ChatMessage, len(serverGS.ChatHistory))
		copy(m.gameState.ChatHistory, serverGS.ChatHistory)

		// Stop polling if game has ended
		if serverGS.IsEnded {
			m.pollingActive = false
		}
	}

	if len(m.pendingUserMessages) > 0 {
		for _, pm := range m.pendingUserMessages {
			found := false
			for _, existing := range m.gameState.ChatHistory {
				if existing.Role == pm.Role && existing.Content == pm.Content {
					found = true
					break
				}
			}
			if !found {
				// Append pending messages at the end to maintain proper order
				m.gameState.ChatHistory = append(m.gameState.ChatHistory, pm)
			}
		}
		// Filter out any pending messages that have now appeared in server data
		var stillPending []chat.ChatMessage
		for _, pm := range m.pendingUserMessages {
			present := false
			for _, existing := range m.gameState.ChatHistory {
				if existing.Role == pm.Role && existing.Content == pm.Content {
					present = true
					break
				}
			}
			if !present { // Should rarely happen; keep if somehow not present
				stillPending = append(stillPending, pm)
			}
		}
		m.pendingUserMessages = stillPending
	}

	// If chat history length changed or pending merges occurred, re-render chat (but don't lose scroll pin state)
	if len(m.gameState.ChatHistory) != origLen {
		m.writeChatContent()
	}
}

type chatResponseMsg struct {
	response *chat.ChatResponse
	err      error
}

type gameStateMsg struct {
	gameState *state.GameState
	err       error
}

type scenariosLoadedMsg struct {
	scenarios   []string
	scenarioMap map[string]string
	err         error
}

type gameStateCreatedMsg struct {
	gameState *state.GameState
	err       error
}

type progressTickMsg struct{}
type pollTickMsg struct{}
type pollResultMsg struct {
	seq       int
	gameState *state.GameState
	err       error
}

var (
	chatPanelStyle = lipgloss.NewStyle().
			PaddingTop(2).
			PaddingBottom(1).
			PaddingLeft(3).
			PaddingRight(0)

	metaPanelStyle = lipgloss.NewStyle().
			PaddingTop(2).
			PaddingBottom(0).
			PaddingLeft(0).
			PaddingRight(2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")). // pink
			Bold(true)

	speakerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")). // purple
			Bold(true)

	narratorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // green

	metaStyle = narratorStyle // copy narrator style for now

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // teal

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // red

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // yellow

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // dark grey

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255"))

	modalTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Align(lipgloss.Center)

	modalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	modalSelectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("205")).
				Bold(true)
)

var separatorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240")) // dark grey

func NewConsoleUI(cfg *ConsoleConfig, client *http.Client) ConsoleUI {
	ta := textarea.New()
	ta.Placeholder = PlaceHolderText
	ta.Focus()
	ta.Prompt = promptStyle.Render(":: ")
	ta.CharLimit = 1000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	// Style the textarea to match user text color (teal)
	tealColor := lipgloss.Color("39")
	ta.FocusedStyle.Text = ta.FocusedStyle.Text.Foreground(tealColor)
	ta.BlurredStyle.Text = ta.BlurredStyle.Text.Foreground(tealColor)
	ta.FocusedStyle.Base = ta.FocusedStyle.Base.Foreground(tealColor)
	ta.BlurredStyle.Base = ta.BlurredStyle.Base.Foreground(tealColor)

	chatVp := viewport.New(50, 20)
	chatVp.MouseWheelEnabled = false // enabling causes text to jump around

	metaVp := viewport.New(20, 20)

	return ConsoleUI{
		config:            cfg,
		client:            client,
		textarea:          ta,
		chatViewport:      chatVp,
		metaViewport:      metaVp,
		ready:             false,
		showScenarioModal: true,
		loadingScenarios:  true,
		selectedScenario:  0,
		profanityFilter:   textfilter.NewProfanityFilter(),
	}
}

func writeInitialContent(gs *state.GameState, scenarioName string, chatWidth int) string {
	var content strings.Builder
	content.WriteString("Welcome to " + titleStyle.Render(scenarioName) + "...\n\n")
	// content.WriteString("Type your messages below to interact with the story.\n\n")
	content.WriteString(separatorStyle.Render(strings.Repeat("─ ", chatWidth/2-6)) + "\n\n")

	if gs != nil && len(gs.ChatHistory) > 0 {
		// Use the same formatting as writeChatContent for consistency
		formattedMsg := formatNarratorResponse(gs.ChatHistory[0].Content, chatWidth)
		content.WriteString(formattedMsg + "\n\n")
	}
	return content.String()
}

func (m *ConsoleUI) scenarioDisplayName() string {
	if m == nil || m.gameState == nil {
		return ""
	}
	file := m.gameState.Scenario
	// scenarioMap maps displayName -> file; reverse lookup
	for display, f := range m.scenarioMap {
		if f == file {
			return display
		}
	}
	return file // fallback to file name
}

func writeSidebar(gs *state.GameState, width int, scenarioDisplay string, pollingActive bool) string {
	var content strings.Builder

	//castle := " _   |>  _\n[_]--'--[_]\n|'|\"\"`\"\"|'|\n| | /^\\ | |\n|_|_|I|_|_|"
	castle := " _   |>  _\n"
	castle += "[_]--'--[_]   STORY ENGINE\n"
	castle += "|'|\"\"`\"\"|'|   LLM-Powered Text\n"
	castle += "| | /^\\ | |   Adventure Game\n"
	castle += "|_|_|I|_|_|  "

	content.WriteString("\n" + titleStyle.Render(castle) + "\n\n")

	content.WriteString(scenarioDisplay + "\n")
	if gs.SceneName != "" {
		content.WriteString(metaStyle.Render("Scene: "))
		content.WriteString(gs.SceneName + "\n")
	}
	content.WriteString(metaStyle.Render("Location: "))
	content.WriteString(gs.Location + "\n")
	content.WriteString(metaStyle.Render("Turn: "))
	content.WriteString(fmt.Sprintf("%d", gs.TurnCounter) + "\n\n")

	content.WriteString(metaStyle.Render("Inventory: ") + "\n")
	if len(gs.Inventory) == 0 {
		content.WriteString("None\n\n")
	} else {
		for i := range gs.Inventory {
			content.WriteString(fmt.Sprintf("• %s\n", gs.Inventory[i]))
		}
	}

	// content.WriteString("\n")
	// content.WriteString(metaStyle.Render("Commands:") + "\n")
	// content.WriteString("• Ctrl+C: Quit\n")
	// content.WriteString("• Ctrl+N: New Game\n")
	// content.WriteString("• Ctrl+Y: Copy GameState ID\n")
	// content.WriteString("• Ctrl+Z: Clear Text\n")
	// content.WriteString("• Enter: Send\n")

	if gs.IsEnded {
		content.WriteString("\n" + titleStyle.Render("GAME ENDED") + "\n")
	}

	if pollingActive {
		content.WriteString("\n" + loadingStyle.Render("Syncing game state...") + "\n")
	}

	content.WriteString("\n")
	content.WriteString(promptStyle.Render(gs.ModelName) + "\n\n")
	width = max(8, width) // min width of 8

	// Format the UUID to wrap nicely
	idStr := gs.ID.String()
	wrappedIDLines := smartWrap(idStr, width)
	for _, line := range wrappedIDLines {
		content.WriteString(promptStyle.Render(line) + "\n")
	}
	content.WriteString("\n")

	content.WriteString(promptStyle.Render("© 2025 Joseph Webster"))

	return content.String()
}

// writeChatContent builds the chat content from game state for the current viewport width
func (m *ConsoleUI) writeChatContent() {
	chatWidth := m.chatViewport.Width - 6 // Account for left(3) + right(3) padding

	// Determine if we should auto-scroll (only if user was already at bottom)
	wasBottom := m.chatViewport.AtBottom()
	prevOffset := m.chatViewport.YOffset

	if m.gameState == nil || len(m.gameState.ChatHistory) == 0 {
		m.chatViewport.SetContent(writeInitialContent(m.gameState, m.scenarioDisplayName(), chatWidth))
		if wasBottom {
			m.chatViewport.GotoBottom()
		} else {
			m.chatViewport.YOffset = prevOffset
		}
		return
	}

	var content strings.Builder

	// Always include the welcome message
	content.WriteString("Welcome to " + titleStyle.Render(m.scenarioDisplayName()) + "...\n\n")
	content.WriteString(separatorStyle.Render(strings.Repeat("─ ", chatWidth/2-6)) + "\n\n")

	for _, msg := range m.gameState.ChatHistory {
		switch msg.Role {
		case "assistant", "system":
			formattedMsg := formatNarratorResponse(msg.Content, chatWidth)
			content.WriteString(formattedMsg + "\n\n")
		case "user":
			userMsg := userStyle.Render(wordwrap.String(msg.Content, chatWidth-3))
			content.WriteString(userMsg + "\n\n")
		}
	}

	if m.loading {
		content.WriteString(m.renderProgressBar())
	}

	m.chatViewport.SetContent(content.String())
	if !m.userPinned && wasBottom {
		m.chatViewport.GotoBottom()
	} else {
		// Restore previous offset (viewport clamps internally). If pinned, stay pinned; if not at bottom before, preserve context.
		m.chatViewport.YOffset = prevOffset
	}
}

func (m ConsoleUI) Init() tea.Cmd {
	if m.showScenarioModal {
		return m.loadScenarios()
	}
	// Start polling even before game state; scheduler will requeue until game state exists
	return tea.Batch(textarea.Blink, schedulePoll())
}

func (m ConsoleUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle scenario modal first
	if m.showScenarioModal {
		return m.updateScenarioModal(msg)
	}

	// Handle quit modal second
	if m.showQuitModal {
		return m.updateQuitModal(msg)
	}

	// Handle new game modal third
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
				m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
			}
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.showQuitModal = true
			return m, nil

		case tea.KeyCtrlN:
			// Show new game confirmation modal
			m.showNewGameModal = true
			return m, nil

		case tea.KeyCtrlY:
			// Copy GameState ID to system clipboard (assume non-nil per user instruction)
			if m.gameState != nil {
				_ = clipboard.WriteAll(m.gameState.ID.String())
				// Optionally append a tiny notice to metadata (non-intrusive)
				m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
			}
			return m, nil

		case tea.KeyCtrlZ:
			// Clear the text area
			m.textarea.Reset()
			return m, nil

		case tea.KeyEnter:
			if m.loading {
				return m, nil
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			// Apply profanity filtering if the scenario's content rating requires it
			if textfilter.ShouldFilterContent(m.contentRating) {
				input = m.profanityFilter.FilterText(input)
			}

			if strings.HasPrefix(input, "/") {
				return m.handleCommand(input)
			}

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

			// Add user message to game state first
			userMessage := chat.ChatMessage{
				Role:    "user",
				Content: input,
			}
			m.gameState.ChatHistory = append(m.gameState.ChatHistory, userMessage)
			m.pendingUserMessages = append(m.pendingUserMessages, userMessage)

			// Reformat content to include the new user message
			m.writeChatContent()

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

	case chatResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			// Remove loading message and add error by reformatting
			m.writeChatContent()
			currentContent := m.chatViewport.View()
			errorMsg := errorStyle.Render("Error: "+msg.err.Error()) + "\n\n"
			m.chatViewport.SetContent(currentContent + errorMsg)
			// After an error, only scroll if user was already at bottom
			if m.chatViewport.AtBottom() {
				m.chatViewport.GotoBottom()
			}
			// Don't start polling on error
		} else {
			// Add assistant response to game state (server will echo later; we'll de-dupe then)
			assistantMessage := chat.ChatMessage{Role: "assistant", Content: msg.response.Message}
			m.gameState.ChatHistory = append(m.gameState.ChatHistory, assistantMessage)

			// Start polling now that we have the chat response (only if game hasn't ended)
			if m.gameState != nil && !m.gameState.IsEnded {
				m.pollingActive = true
				m.pollingStartedAt = time.Now() // Record time AFTER chat response, look for updates after this
			}
			// Update metadata to show polling indicator
			m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))

			m.writeChatContent()
			if !m.userPinned {
				m.chatViewport.GotoBottom()
			}
			// Continue polling to detect when the server has updated the gamestate with our changes
		}
		return m, tea.Batch(m.refreshGameState(), schedulePoll())

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
					m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
				} else if m.pollingActive && msg.gameState.UpdatedAt.After(m.pollingStartedAt) {
					// Check if we got an updated timestamp and should stop active polling
					m.pollingActive = false
					// Apply the full updated gamestate
					m.mergeServerGameState(msg.gameState)
					m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
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
					m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
				}
			}
		}
		return m, nil

	case gameStateMsg:
		if msg.err == nil && msg.gameState != nil {
			m.mergeServerGameState(msg.gameState)
			m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
		}

	case progressTickMsg:
		if m.loading {
			m.progressTick++
			m.writeChatContent()     // Refresh the chat content to update the progress bar
			return m, progressTick() // Continue the animation
		}
	}

	// Update components for non-mouse events (textarea & metadata). Chat viewport already handled for key scroll above.
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.metaViewport, mvCmd = m.metaViewport.Update(msg)
	return m, tea.Batch(tiCmd, vpCmd, mvCmd)
}

func formatNarratorResponse(response string, width int) string {
	// Check if response already has a speaker prefix
	hasPrefix := false
	if idx := strings.Index(response, ":"); idx > 0 && idx <= 20 {
		speaker := response[:idx]
		if len(strings.Fields(speaker)) <= 2 {
			hasPrefix = true
		}
	}

	// If no prefix, we'll add "Narrator: " so reduce available width
	wrapWidth := width
	if !hasPrefix {
		narratorPrefix := AgentName + ": "
		wrapWidth = width - len(narratorPrefix)
	}

	// Wrap the text to the available width
	wrappedResponse := wordwrap.String(response, wrapWidth)
	lines := strings.Split(wrappedResponse, "\n")
	var formattedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			formattedLines = append(formattedLines, "")
			continue
		}

		if idx := strings.Index(trimmed, ":"); idx > 0 && idx <= 20 {
			speaker := trimmed[:idx]
			rest := trimmed[idx+1:]
			if len(strings.Fields(speaker)) <= 2 {
				formattedLines = append(formattedLines, speakerStyle.Render(speaker+":")+rest)
				continue
			}
		}

		formattedLines = append(formattedLines, line)
	}

	result := strings.Join(formattedLines, "\n")
	if !hasPrefix && !strings.HasPrefix(strings.TrimSpace(result), speakerStyle.Render("")) {
		result = narratorStyle.Render(AgentName+": ") + result
	}

	return result
}

func (m ConsoleUI) handleCommand(input string) (tea.Model, tea.Cmd) {
	cmd := strings.ToLower(strings.TrimSpace(input))

	switch cmd {
	case "/vars":
		var varsText strings.Builder
		varsText.WriteString(titleStyle.Render("Variables:") + "\n")
		if len(m.gameState.Vars) == 0 {
			varsText.WriteString("No variables are set.\n")
		} else {
			for k, v := range m.gameState.Vars {
				varsText.WriteString(fmt.Sprintf("• %s = %v\n", k, v))
			}
		}
		varsText.WriteString("\n")

		currentContent := m.chatViewport.View()
		m.chatViewport.SetContent(currentContent + varsText.String())
		m.chatViewport.GotoBottom()
	}

	m.textarea.Reset()
	return m, nil
}

func (m ConsoleUI) sendChatMessage(message string) tea.Cmd {
	return func() tea.Msg {
		chatReq := chat.ChatRequest{
			GameStateID: m.gameState.ID,
			Message:     message,
		}

		jsonData, err := json.Marshal(chatReq)
		if err != nil {
			return chatResponseMsg{nil, fmt.Errorf("failed to marshal request: %w", err)}
		}

		resp, err := m.client.Post(
			m.config.APIBaseURL+"/v1/chat",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return chatResponseMsg{nil, fmt.Errorf("failed to send request: %w", err)}
		}
		defer func() {
			_ = resp.Body.Close() // Ignore error in defer
		}()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return chatResponseMsg{nil, fmt.Errorf("failed to read response: %w", err)}
		}
		// Update metadata in case server mutated something quickly (turn counter etc.)
		if m.gameState != nil {
			m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
		}

		if resp.StatusCode != http.StatusOK {
			var errorResp ErrorResponse
			if err := json.Unmarshal(body, &errorResp); err != nil {
				return chatResponseMsg{nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))}
			}
			return chatResponseMsg{nil, fmt.Errorf("chat request failed: %s", errorResp.Error)}
		}

		var chatResp chat.ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return chatResponseMsg{nil, fmt.Errorf("failed to parse response: %w", err)}
		}

		return chatResponseMsg{&chatResp, nil}
	}
}

func (m ConsoleUI) refreshGameState() tea.Cmd {
	return func() tea.Msg {
		gs, err := getGameState(m.client, m.config.APIBaseURL, m.gameState.ID)
		return gameStateMsg{gs, err}
	}
}

func (m ConsoleUI) loadScenarios() tea.Cmd {
	return func() tea.Msg {
		orderedNames, scenarioMap, err := listScenarios(m.client, m.config.APIBaseURL)
		return scenariosLoadedMsg{orderedNames, scenarioMap, err}
	}
}

func (m ConsoleUI) createGameStateFromScenario(scenarioFile string) tea.Cmd {
	return func() tea.Msg {
		gs, err := createGameState(m.client, m.config.APIBaseURL, scenarioFile)
		return gameStateCreatedMsg{gs, err}
	}
}

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

	case gameStateCreatedMsg:
		// Regardless of outcome, we're no longer in the create-game loading phase
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.gameState = msg.gameState
			m.showScenarioModal = false
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
			m.metaViewport.SetContent(writeSidebar(m.gameState, m.metaViewport.Width, m.scenarioDisplayName(), m.pollingActive))
			m.textarea.Focus() // Ensure textarea gets focus when modal closes
			m.ready = true
		}
		return m, textarea.Blink // Return focus command

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
				m.loading = true
				// First fetch scenario details to get the content rating
				s, err := getScenario(m.client, m.config.APIBaseURL, scenarioFile)
				if err != nil {
					m.err = fmt.Errorf("failed to fetch scenario details: %w", err)
					return m, nil
				}
				m.contentRating = s.Rating
				return m, m.createGameStateFromScenario(scenarioFile)
			}
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
			return m, nil
		case tea.KeyEnter:
			return m.startNewGame()
		default:
			switch msg.String() {
			case "y", "Y":
				return m.startNewGame()
			case "n", "N":
				m.showNewGameModal = false
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
	m.pollSeq = 0
	m.activePollSeq = 0
	m.pollInFlight = false
	m.pollingActive = false
	m.pollingStartedAt = time.Time{}
	m.finalMessageSent = false
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

func (m ConsoleUI) View() string {
	if m.showScenarioModal {
		return m.renderScenarioModal()
	}

	if m.showQuitModal {
		return m.renderQuitModal()
	}

	if m.showNewGameModal {
		return m.renderNewGameModal()
	}

	if !m.ready {
		return "\n  Initializing..."
	}

	chatWidth := int(float64(m.width)*0.75) - 4
	metaWidth := m.width - chatWidth - 6

	chatPanel := chatPanelStyle.Width(chatWidth).Height(m.height - 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.chatViewport.View(),
			"", // Add empty line for spacing
			separatorStyle.Render(strings.Repeat("─", chatWidth-8)),
			m.textarea.View(),
		),
	)
	metaPanel := metaPanelStyle.Width(metaWidth).Height(m.height - 2).Render(
		m.metaViewport.View(),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, metaPanel)
}

// renderProgressBar creates an animated progress bar for loading states
func (m ConsoleUI) renderProgressBar() string {
	// Determine usable content width (viewport width minus padding used elsewhere: 3 left + 3 right)
	usable := m.chatViewport.Width - 6
	if usable <= 0 {
		usable = 30 // fallback before sizing
	}

	// Clamp bar width to a sensible range
	if usable > 80 {
		usable = 80 // avoid overly wide bars
	} else if usable < 10 {
		usable = 10 // minimum visible bar
	}

	const totalFrames = 40
	frame := m.progressTick % totalFrames
	filled := (frame * usable) / totalFrames

	var bar strings.Builder
	for i := 0; i < usable; i++ {
		if i < filled {
			bar.WriteString("█")
		} else if i == filled && frame%4 < 2 {
			bar.WriteString("▓") // Blinking effect at the progress point
		} else {
			bar.WriteString("░")
		}
	}
	return separatorStyle.Render(bar.String())
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
		resp, err := client.Get(fmt.Sprintf("%s/v1/gamestate/%s", baseURL, id))
		if err != nil {
			return pollResultMsg{seq: seq, gameState: nil, err: err}
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return pollResultMsg{seq: seq, gameState: nil, err: err}
		}
		if resp.StatusCode != http.StatusOK {
			return pollResultMsg{seq: seq, gameState: nil, err: fmt.Errorf("poll status %d", resp.StatusCode)}
		}
		var gs state.GameState
		if err := json.Unmarshal(body, &gs); err != nil {
			return pollResultMsg{seq: seq, gameState: nil, err: err}
		}
		return pollResultMsg{seq: seq, gameState: &gs, err: nil}
	}
}
