package main

import (
	"net/http"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/textfilter"
)

const (
	AgentName       = "Narrator"
	PlaceHolderText = "Type your message here...\nExamples: Look around. Get the key. Talk to the guard."
)

type playStyle struct {
	label       string
	rules       state.RulesMode
	temperature float64
}

var playStyles = []playStyle{
	{"High creativity, relaxed rules", state.RulesRelaxed, 0.8},
	{"Medium creativity, relaxed rules", state.RulesRelaxed, 0.6},
	{"Medium creativity, strict rules", state.RulesStrict, 0.6},
	{"Low creativity, strict rules", state.RulesStrict, 0.4},
}

const defaultPlayStyleIndex = 2

// calculateAverageLatency computes the average latency from a slice of latencies
func calculateAverageLatency(latencies []float64) float64 {
	if len(latencies) == 0 {
		return 0
	}
	sum := 0.0
	for _, latency := range latencies {
		sum += latency
	}
	return sum / float64(len(latencies))
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

	// PC selection state
	showPCModal          bool
	pcs                  []string
	pcMap                map[string]string
	selectedPC           int
	loadingPCs           bool
	selectedScenarioFile string
	defaultPCID          string // Default PC ID from scenario
	selectedPCID         string // PC ID chosen before play-style modal

	// Play style selection state
	showPlayStyleModal bool
	selectedPlayStyle  int

	// Provider selection state
	showProviderModal  bool
	providers          []providerOption
	selectedProvider   int
	loadingProviders   bool
	defaultProvider    string
	selectedProviderID string
	selectedRules      string
	selectedTemp       float64

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

	// Chat latency tracking
	chatRequestStartTime time.Time // timestamp when the last chat request was sent
	lastChatLatency      float64   // latency of the last chat request in seconds
	chatLatencies        []float64 // all chat latencies for the session

	// Pending user messages not yet confirmed in server game state
	// Pending user messages awaiting server echo (assistant responses are applied when streaming completes)
	pendingUserMessages []chat.ChatMessage

	// Streaming state
	isStreaming         bool   // whether we're currently receiving a streaming response
	streamingContent    string // accumulated content from streaming chunks
	streamingMessageIdx int    // index of the message being streamed in ChatHistory

	// SSE event channel for async request updates
	eventChan <-chan SSEEvent // channel for receiving SSE events from the server

	// Force a full chat re-render on next gameStateMsg (used by Ctrl+R)
	forceRerender bool
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

type pcsLoadedMsg struct {
	pcs   []string
	pcMap map[string]string
	err   error
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

type sseEventMsg struct {
	event SSEEvent
}

type chatErrorMsg struct {
	err error
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
			Foreground(lipgloss.Color("203")) // lighter red/pink for better visibility on black

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

type providersLoadedMsg struct {
	defaultProvider string
	providers       []providerOption
	err             error
}

// findDefaultPCIndex finds the index of the default PC in the PC list
// Returns the index if found, or the index of "classic" as fallback, or 0 if neither found
func (m *ConsoleUI) findDefaultPCIndex() int {
	// First, try to find the scenario's default PC
	if m.defaultPCID != "" {
		for i, pcName := range m.pcs {
			if pcID := m.pcMap[pcName]; pcID == m.defaultPCID {
				return i
			}
		}
	}

	// Fallback to "classic" PC
	for i, pcName := range m.pcs {
		if pcID := m.pcMap[pcName]; pcID == "classic" {
			return i
		}
	}

	// If neither found, return 0 (first PC)
	return 0
}

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
	chatVp.MouseWheelEnabled = false // mouse scroll handled manually below

	metaVp := viewport.New(20, 20)
	metaVp.MouseWheelEnabled = false // sidebar is locked at top

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
		selectedPlayStyle: defaultPlayStyleIndex,
		profanityFilter:   textfilter.NewProfanityFilter(),
	}
}

func (m ConsoleUI) Init() tea.Cmd {
	if m.showScenarioModal {
		return m.loadScenarios()
	}
	// Start polling even before game state; scheduler will requeue until game state exists
	return tea.Batch(textarea.Blink, schedulePoll())
}
