package main

import (
	"context"
	"net/http"
	"time"
	"uuid"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
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
	// Pending user messages awaiting server echo (assistant responses are applied when streaming completes)
	pendingUserMessages []chat.ChatMessage

	// Streaming state
	isStreaming         bool   // whether we're currently receiving a streaming response
	streamingContent    string // accumulated content from streaming chunks
	streamingMessageIdx int    // index of the message being streamed in ChatHistory

	// SSE event channel for async request updates
	eventChan <-chan SSEEvent    // channel for receiving SSE events from the server
	sseCancel context.CancelFunc // cancels the in-flight SSE request
	sseGameID uuid.UUID          // game ID the current SSE listener is subscribed to

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
	}
}

func (m ConsoleUI) Init() tea.Cmd {
	if m.showScenarioModal {
		return m.loadScenarios()
	}
	// Start polling even before game state; scheduler will requeue until game state exists
	return tea.Batch(textarea.Blink, schedulePoll())
}
