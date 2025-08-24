package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/muesli/reflow/wordwrap"
)

const (
	AgentName       = "Narrator"
	PlaceHolderText = "Type your message here..."
)

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
}

type chatResponseMsg struct {
	response *chat.ChatResponse
	err      error
}

type gameStateMsg struct {
	gameState *state.GameState
	err       error
}

var (
	chatPanelStyle = lipgloss.NewStyle().
			PaddingTop(2).
			PaddingBottom(0).
			PaddingLeft(3).
			PaddingRight(3)

	metaPanelStyle = lipgloss.NewStyle().
			Padding(2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")). // pink
			Bold(true)

	speakerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")). // purple
			Bold(true)

	narratorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // green

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // teal

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // red

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // yellow

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // dark grey
)

func NewConsoleUI(cfg *ConsoleConfig, client *http.Client, gs *state.GameState) ConsoleUI {
	ta := textarea.New()
	ta.Placeholder = PlaceHolderText
	ta.Focus()
	ta.Prompt = promptStyle.Render(":: ")
	ta.CharLimit = 1000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	chatVp := viewport.New(50, 20)
	chatVp.SetContent(writeInitialContent(gs))

	// Enable scrollbar
	chatVp.MouseWheelEnabled = true

	metaVp := viewport.New(20, 20)
	metaVp.SetContent(writeMetadata(gs))

	return ConsoleUI{
		config:       cfg,
		client:       client,
		gameState:    gs,
		textarea:     ta,
		chatViewport: chatVp,
		metaViewport: metaVp,
		ready:        false,
	}
}

func writeInitialContent(gs *state.GameState) string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("STORY ENGINE") + "\n\n")
	content.WriteString("Type your messages below to interact with the story.\n\n")
	content.WriteString(strings.Repeat("─", 50) + "\n\n")

	if len(gs.ChatHistory) > 0 {
		content.WriteString(narratorStyle.Render("Narrator: "))
		content.WriteString((gs.ChatHistory[0].Content))
		content.WriteString("\n\n")
	}
	return content.String()
}

func writeMetadata(gs *state.GameState) string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("GAME STATE") + "\n\n")

	content.WriteString("Game ID:\n")
	content.WriteString(gs.ID.String()[:8] + "...\n\n")

	content.WriteString("Scenario:\n")
	content.WriteString(gs.Scenario + "\n\n")

	content.WriteString("Messages:\n")
	content.WriteString(fmt.Sprintf("%d total\n\n", len(gs.ChatHistory)))

	if len(gs.Vars) > 0 {
		content.WriteString("Variables:\n")
		for k, v := range gs.Vars {
			content.WriteString(fmt.Sprintf("• %s: %v\n", k, v))
		}
	} else {
		content.WriteString("Variables:\nNone set\n")
	}

	content.WriteString("\n")
	content.WriteString("Commands:\n")
	content.WriteString("• Ctrl+C: Quit\n")
	content.WriteString("• Enter: Send\n")
	content.WriteString("• /help: Help\n")
	content.WriteString("• /vars: Variables\n")

	return content.String()
}

// writeChatContent builds the chat content from game state for the current viewport width
func (m *ConsoleUI) writeChatContent() {
	if m.gameState == nil || len(m.gameState.ChatHistory) == 0 {
		// No chat history, just show initial content
		m.chatViewport.SetContent(writeInitialContent(m.gameState))
		return
	}

	var content strings.Builder
	chatWidth := m.chatViewport.Width - 6 // Account for left(3) + right(3) padding

	// Add title and intro
	content.WriteString(titleStyle.Render("STORY ENGINE") + "\n\n")
	content.WriteString("Welcome to your text-based adventure!\n")
	content.WriteString("Type your messages below to interact with the story.\n\n")
	content.WriteString(strings.Repeat("─", min(50, chatWidth)) + "\n\n")

	// Reformat all chat history for the new width
	for _, msg := range m.gameState.ChatHistory {
		switch msg.Role {
		case "assistant", "system":
			formattedMsg := formatNarratorResponse(msg.Content, chatWidth)
			content.WriteString(formattedMsg + "\n\n")
		case "user":
			// User messages should also be reformatted if needed
			userMsg := userStyle.Render("You: ") + wordwrap.String(msg.Content, chatWidth-6) + "\n\n"
			content.WriteString(userMsg)
		}
	}

	// If currently loading, add the loading message
	if m.loading {
		content.WriteString(loadingStyle.Render("Narrator is thinking..."))
	}

	m.chatViewport.SetContent(content.String())
	m.chatViewport.GotoBottom()
}

func (m ConsoleUI) Init() tea.Cmd {
	return textarea.Blink
}

func (m ConsoleUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		mvCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Handle mouse events first, before updating other components
		// Check if the mouse event is in the chat viewport area
		// For now, pass all mouse events to chat viewport for text selection
		// The viewport component will ignore events outside its bounds
		m.chatViewport, vpCmd = m.chatViewport.Update(msg)

		// Also update textarea and meta viewport in case they need mouse events
		m.textarea, tiCmd = m.textarea.Update(msg)
		m.metaViewport, mvCmd = m.metaViewport.Update(msg)

		return m, tea.Batch(tiCmd, vpCmd, mvCmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

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
		m.metaViewport.SetContent(writeMetadata(m.gameState))

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.loading {
				return m, nil
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			if strings.HasPrefix(input, "/") {
				return m.handleCommand(input)
			}

			m.textarea.Reset()
			m.loading = true

			// Add user message to game state first
			userMessage := chat.ChatMessage{
				Role:    "user",
				Content: input,
			}
			m.gameState.ChatHistory = append(m.gameState.ChatHistory, userMessage)

			// Reformat content to include the new user message
			m.writeChatContent()

			return m, m.sendChatMessage(input)
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
		} else {
			// Add assistant response to game state
			assistantMessage := chat.ChatMessage{
				Role:    "assistant",
				Content: msg.response.Message,
			}
			m.gameState.ChatHistory = append(m.gameState.ChatHistory, assistantMessage)

			// Reformat all content including the new response
			m.writeChatContent()
		}
		m.chatViewport.GotoBottom()
		return m, m.refreshGameState()

	case gameStateMsg:
		if msg.err == nil && msg.gameState != nil {
			m.gameState = msg.gameState
			m.metaViewport.SetContent(writeMetadata(m.gameState))
		}
	}

	// Update components for non-mouse events
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.chatViewport, vpCmd = m.chatViewport.Update(msg)
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
	case "/help":
		helpText := `
Commands:
• /help - Show this help
• /vars - Show game variables
• Ctrl+C - Quit game

How to play:
• Type your actions and press Enter
• The narrator will respond to guide the story
• Be descriptive for better responses
`
		currentContent := m.chatViewport.View()
		m.chatViewport.SetContent(currentContent + titleStyle.Render("Help:") + helpText + "\n")
		m.chatViewport.GotoBottom()

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

func (m ConsoleUI) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	chatWidth := int(float64(m.width)*0.75) - 4
	metaWidth := m.width - chatWidth - 6

	chatPanel := chatPanelStyle.Width(chatWidth).Height(m.height - 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.chatViewport.View(),
			"", // Add empty line for spacing
			m.textarea.View(),
		),
	)

	metaPanel := metaPanelStyle.Width(metaWidth).Height(m.height - 2).Render(
		m.metaViewport.View(),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, metaPanel)
}

// Helper function for Go versions that don't have min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
