package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const (
	AgentName = "Narrator"
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

type model struct {
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
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1)

	metaPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	speakerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	narratorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
)

func initialModel(cfg *ConsoleConfig, client *http.Client, gs *state.GameState) model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()
	ta.Prompt = "You: "
	ta.CharLimit = 1000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	chatVp := viewport.New(50, 20)
	chatVp.SetContent(formatInitialContent(gs))

	metaVp := viewport.New(20, 20)
	metaVp.SetContent(formatMetadata(gs))

	return model{
		config:       cfg,
		client:       client,
		gameState:    gs,
		textarea:     ta,
		chatViewport: chatVp,
		metaViewport: metaVp,
		ready:        false,
	}
}

func formatInitialContent(gs *state.GameState) string {
	var content strings.Builder
	
	content.WriteString(titleStyle.Render("🎭 STORY ENGINE") + "\n\n")
	content.WriteString("Welcome to your text-based adventure!\n")
	content.WriteString("Type your messages below to interact with the story.\n\n")
	content.WriteString(strings.Repeat("─", 50) + "\n\n")

	if len(gs.ChatHistory) > 0 {
		content.WriteString(narratorStyle.Render("Narrator: "))
		content.WriteString(wrapText(gs.ChatHistory[0].Content, 45))
		content.WriteString("\n\n")
	}

	return content.String()
}

func formatMetadata(gs *state.GameState) string {
	var content strings.Builder
	
	content.WriteString(titleStyle.Render("📊 METADATA") + "\n\n")
	
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

func wrapText(text string, width int) string {
	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		if len(line) <= width {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		var currentLine strings.Builder
		for _, word := range words {
			testLine := currentLine.String()
			if testLine != "" {
				testLine += " "
			}
			testLine += word

			if len(testLine) <= width {
				if currentLine.Len() > 0 {
					currentLine.WriteString(" ")
				}
				currentLine.WriteString(word)
			} else {
				if currentLine.Len() > 0 {
					wrappedLines = append(wrappedLines, currentLine.String())
					currentLine.Reset()
				}
				currentLine.WriteString(word)
			}
		}

		if currentLine.Len() > 0 {
			wrappedLines = append(wrappedLines, currentLine.String())
		}
	}

	return strings.Join(wrappedLines, "\n")
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		mvCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.chatViewport, vpCmd = m.chatViewport.Update(msg)
	m.metaViewport, mvCmd = m.metaViewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			chatWidth := int(float64(m.width) * 0.75) - 4
			metaWidth := m.width - chatWidth - 6
			
			m.chatViewport.Width = chatWidth - 2
			m.chatViewport.Height = m.height - 8
			
			m.metaViewport.Width = metaWidth - 2
			m.metaViewport.Height = m.height - 4
			
			m.textarea.SetWidth(chatWidth - 4)
			m.ready = true
		} else {
			chatWidth := int(float64(m.width) * 0.75) - 4
			metaWidth := m.width - chatWidth - 6
			
			m.chatViewport.Width = chatWidth - 2
			m.chatViewport.Height = m.height - 8
			
			m.metaViewport.Width = metaWidth - 2
			m.metaViewport.Height = m.height - 4
			
			m.textarea.SetWidth(chatWidth - 4)
		}

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
			
			currentContent := m.chatViewport.View()
			userMsg := userStyle.Render("You: ") + input + "\n\n"
			loadingMsg := loadingStyle.Render("Narrator is thinking...")
			m.chatViewport.SetContent(currentContent + userMsg + loadingMsg)
			m.chatViewport.GotoBottom()

			return m, m.sendChatMessage(input)
		}

	case chatResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			currentContent := m.chatViewport.View()
			currentContent = strings.TrimSuffix(currentContent, loadingStyle.Render("Narrator is thinking..."))
			errorMsg := errorStyle.Render("Error: " + msg.err.Error()) + "\n\n"
			m.chatViewport.SetContent(currentContent + errorMsg)
		} else {
			currentContent := m.chatViewport.View()
			currentContent = strings.TrimSuffix(currentContent, loadingStyle.Render("Narrator is thinking..."))
			
			response := msg.response.Message
			formattedResponse := formatNarratorResponse(response)
			m.chatViewport.SetContent(currentContent + formattedResponse + "\n\n")
		}
		m.chatViewport.GotoBottom()
		return m, m.refreshGameState()

	case gameStateMsg:
		if msg.err == nil && msg.gameState != nil {
			m.gameState = msg.gameState
			m.metaViewport.SetContent(formatMetadata(m.gameState))
		}
	}

	return m, tea.Batch(tiCmd, vpCmd, mvCmd)
}

func formatNarratorResponse(response string) string {
	lines := strings.Split(response, "\n")
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
	if !strings.HasPrefix(strings.TrimSpace(result), speakerStyle.Render("")) {
		result = narratorStyle.Render("Narrator: ") + result
	}
	
	return result
}

func (m model) handleCommand(input string) (tea.Model, tea.Cmd) {
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

func (m model) sendChatMessage(message string) tea.Cmd {
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

func (m model) refreshGameState() tea.Cmd {
	return func() tea.Msg {
		gs, err := getGameState(m.client, m.config.APIBaseURL, m.gameState.ID)
		return gameStateMsg{gs, err}
	}
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	chatWidth := int(float64(m.width) * 0.75) - 4
	metaWidth := m.width - chatWidth - 6

	chatPanel := chatPanelStyle.Width(chatWidth).Height(m.height - 6).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.chatViewport.View(),
			m.textarea.View(),
		),
	)

	metaPanel := metaPanelStyle.Width(metaWidth).Height(m.height - 2).Render(
		m.metaViewport.View(),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, metaPanel)
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	cfg := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    30 * time.Second,
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	if !testConnection(client, cfg.APIBaseURL) {
		fmt.Fprintf(os.Stderr, "Could not connect to API. Please ensure the API is running.\nTry: docker-compose up -d\n")
		os.Exit(1)
	}

	orderedNames, scenarioMap, err := listScenarios(client, cfg.APIBaseURL)
	if err != nil || len(orderedNames) == 0 {
		fmt.Fprintf(os.Stderr, "Failed to list scenarios: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Available Scenarios:")
	for i := range orderedNames {
		fmt.Printf("  %d - %s (%s)\n", i+1, orderedNames[i], scenarioMap[orderedNames[i]])
	}
	fmt.Print("\nSelect a scenario by number: ")
	
	var choice int
	if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(orderedNames) {
		fmt.Fprintf(os.Stderr, "Invalid selection\n")
		os.Exit(1)
	}

	scenarioName := orderedNames[choice-1]
	scenarioFile := scenarioMap[scenarioName]

	gs, err := createGameState(client, cfg.APIBaseURL, scenarioFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create game state: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(cfg, client, gs), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func testConnection(client *http.Client, baseURL string) bool {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()
	return resp.StatusCode == http.StatusOK
}

func getGameState(client *http.Client, baseURL string, gameStateID uuid.UUID) (*state.GameState, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v1/gamestate/%s", baseURL, gameStateID))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to get game state: %s", errorResp.Error)
	}

	var gameState state.GameState
	if err := json.Unmarshal(body, &gameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}
	return &gameState, nil
}

func createGameState(client *http.Client, baseURL string, scenarioFile string) (*state.GameState, error) {
	gameState := &state.GameState{
		Scenario: scenarioFile,
	}

	jsonData, err := json.Marshal(gameState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal game state: %w", err)
	}

	resp, err := client.Post(
		baseURL+"/v1/gamestate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to create game state: %s", errorResp.Error)
	}

	var createdGameState state.GameState
	if err := json.Unmarshal(body, &createdGameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}

	return &createdGameState, nil
}

func listScenarios(client *http.Client, baseURL string) ([]string, map[string]string, error) {
	resp, err := client.Get(baseURL + "/v1/scenarios")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()
	
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var scenarioMap map[string]string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	
	if err := json.Unmarshal(body, &scenarioMap); err != nil {
		return nil, nil, err
	}

	var names []string
	for name := range scenarioMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, scenarioMap, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}