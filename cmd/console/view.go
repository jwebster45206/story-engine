package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/muesli/reflow/wordwrap"
)

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

func writeSidebar(gs *state.GameState, width int, scenarioDisplay string, pollingActive bool, chatLatencies []float64) string {
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
	// Display location name instead of key
	if loc, ok := gs.WorldLocations[gs.Location]; ok && loc.Name != "" {
		content.WriteString(loc.Name + "\n")
	} else {
		content.WriteString(gs.Location + "\n")
	}
	content.WriteString(metaStyle.Render("Turn: "))
	content.WriteString(fmt.Sprintf("%d", gs.TurnCounter) + "\n\n")

	content.WriteString(metaStyle.Render("Inventory: ") + "\n")
	if len(gs.Inventory) == 0 {
		content.WriteString("None\n\n")
	} else {
		for i := range gs.Inventory {
			fmt.Fprintf(&content, "• %s\n", gs.Inventory[i])
		}
	}

	content.WriteString("\n")
	content.WriteString(metaStyle.Render("Commands:") + "\n")
	content.WriteString("• Ctrl+C: Quit\n")
	content.WriteString("• Ctrl+N: New Game\n")
	content.WriteString("• Ctrl+E: Export Chat\n")
	content.WriteString("• Ctrl+S: Save State\n")
	content.WriteString("• Ctrl+R: Re-render\n")

	if gs.IsEnded {
		content.WriteString("\n" + titleStyle.Render("GAME ENDED") + "\n")
	}

	if pollingActive {
		content.WriteString("\n" + loadingStyle.Render("Syncing game state...") + "\n")
	}

	// chat latency
	if len(chatLatencies) > 0 {
		lastChatLatency := chatLatencies[len(chatLatencies)-1]
		content.WriteString("\n" + promptStyle.Render(fmt.Sprintf("Last Chat: %.3fs", lastChatLatency)) + "\n")
		avgLatency := calculateAverageLatency(chatLatencies)
		content.WriteString(promptStyle.Render(fmt.Sprintf("Avg Chat: %.3fs", avgLatency)) + "\n")
	}

	content.WriteString("\n")
	if gs.Provider != "" {
		content.WriteString(promptStyle.Render(gs.Provider) + "\n")
	}
	content.WriteString(promptStyle.Render(gs.ModelName) + "\n\n")
	// width = max(8, width) // min width of 8

	// // Format the UUID to wrap nicely
	// idStr := gs.ID.String()
	// wrappedIDLines := smartWrap(idStr, width)
	// for _, line := range wrappedIDLines {
	// 	content.WriteString(promptStyle.Render(line) + "\n")
	// }
	// content.WriteString("\n")

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
		case "assistant":
			formattedMsg := formatNarratorResponse(msg.Content, chatWidth)
			content.WriteString(formattedMsg + "\n\n")
		case "system":
			// Check if this is an error message (already styled) or regular system message
			if strings.Contains(msg.Content, "Error:") && strings.Contains(msg.Content, "\x1b[") {
				// This is a pre-styled error message, display as-is
				content.WriteString(msg.Content + "\n\n")
			} else {
				// Regular system message, format normally
				formattedMsg := formatNarratorResponse(msg.Content, chatWidth)
				content.WriteString(formattedMsg + "\n\n")
			}
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

func (m ConsoleUI) View() string {
	if m.showScenarioModal {
		return m.renderScenarioModal()
	}

	if m.showPCModal {
		return m.renderPCModal()
	}

	if m.showPlayStyleModal {
		return m.renderPlayStyleModal()
	}

	if m.showProviderModal {
		return m.renderProviderModal()
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
