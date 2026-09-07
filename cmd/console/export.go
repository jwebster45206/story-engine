package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
				fmt.Fprintf(&varsText, "• %s = %v\n", k, v)
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

func (m ConsoleUI) handleExport() (ConsoleUI, tea.Cmd) {
	if m.gameState == nil {
		// Show error in chat if no game state exists
		errMsg := "Cannot export: no active game session"
		currentContent := m.chatViewport.View()
		m.chatViewport.SetContent(currentContent + "\n" + errorStyle.Render("Error: "+errMsg))
		m.chatViewport.GotoBottom()
		return m, nil
	}

	// Generate filename: {scene_name}_{pc_name}_{timestamp}.md
	sceneName := sanitizeFilename(m.gameState.SceneName)
	if sceneName == "" {
		sceneName = "unknown_scene"
	}

	pcName := "unknown_pc"
	if m.gameState.PC != nil && m.gameState.PC.Spec.Name != "" {
		pcName = sanitizeFilename(m.gameState.PC.Spec.Name)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.md", sceneName, pcName, timestamp)

	// Resolve export directory: prefer ./data/exports/, fall back to working dir
	exportDir := "./data/exports"
	if info, err := os.Stat(exportDir); err != nil || !info.IsDir() {
		exportDir = "."
	}
	filepath := exportDir + "/" + filename

	// Generate markdown content
	var content strings.Builder

	// Get scenario title from display name
	scenarioTitle := m.scenarioDisplayName()
	if scenarioTitle == "" {
		scenarioTitle = strings.TrimSuffix(m.gameState.Scenario, ".json")
	}

	// Get character name for bolding
	charName := ""
	if m.gameState.PC != nil && m.gameState.PC.Spec.Name != "" {
		charName = m.gameState.PC.Spec.Name
	}

	// Header with metadata
	fmt.Fprintf(&content, "# %s\n\n", scenarioTitle)
	if charName != "" {
		fmt.Fprintf(&content, "**Character:** %s\n\n", charName)
	}
	if m.gameState.Provider != "" {
		fmt.Fprintf(&content, "**Provider:** %s\n\n", m.gameState.Provider)
	}
	fmt.Fprintf(&content, "**Model:** %s\n\n", m.gameState.ModelName)
	fmt.Fprintf(&content, "**Turn Count:** %d\n\n", m.gameState.TurnCounter)
	fmt.Fprintf(&content, "**Exported:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	content.WriteString("---\n\n")

	// Chat history
	for _, msg := range m.gameState.ChatHistory {
		switch msg.Role {
		case "user":
			// Bold character name: prefix takes priority; ReplaceAll only runs if prefix didn't match
			// (avoids double-bolding when charName: appears at the start)
			msgContent := msg.Content
			if charName != "" && strings.HasPrefix(msgContent, charName+":") {
				msgContent = "**" + charName + ":**" + msgContent[len(charName)+1:]
			} else if charName != "" {
				msgContent = strings.ReplaceAll(msgContent, charName+":", "**"+charName+":**")
			}
			fmt.Fprintf(&content, "%s\n\n", msgContent)
		case "assistant":
			// Bold narrator prefix
			fmt.Fprintf(&content, "**Narrator:** %s\n\n", msg.Content)
		case "system":
			fmt.Fprintf(&content, "_System: %s_\n\n", msg.Content)
		}
	}

	// Write file
	err := os.WriteFile(filepath, []byte(content.String()), 0644)
	if err != nil {
		// Show error in chat
		errMsg := fmt.Sprintf("Failed to export: %v", err)
		currentContent := m.chatViewport.View()
		m.chatViewport.SetContent(currentContent + "\n" + errorStyle.Render("Error: "+errMsg))
		m.chatViewport.GotoBottom()
		return m, nil
	}

	// Show success message in chat
	successMsg := fmt.Sprintf("Chat history exported to: %s", filepath)
	currentContent := m.chatViewport.View()
	m.chatViewport.SetContent(currentContent + "\n" + narratorStyle.Render("✓ "+successMsg))
	m.chatViewport.GotoBottom()

	return m, nil
}

func (m ConsoleUI) handleSave() (ConsoleUI, tea.Cmd) {
	if m.gameState == nil {
		return m, nil
	}

	// Generate filename: {scenario_slug}_{pc_name}_{timestamp}.json
	scenarioSlug := sanitizeFilename(strings.TrimSuffix(m.gameState.Scenario, ".json"))
	if scenarioSlug == "" {
		scenarioSlug = "unknown_scenario"
	}

	pcName := "unknown_pc"
	if m.gameState.PC != nil && m.gameState.PC.Spec != nil && m.gameState.PC.Spec.Name != "" {
		pcName = sanitizeFilename(m.gameState.PC.Spec.Name)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.json", scenarioSlug, pcName, timestamp)

	// Resolve save directory: prefer ./data/saves/, fall back to working dir
	saveDir := "./data/saves"
	if info, err := os.Stat(saveDir); err != nil || !info.IsDir() {
		saveDir = "."
	}
	filepath := saveDir + "/" + filename

	// Marshal game state as indented JSON
	data, err := json.MarshalIndent(m.gameState, "", "  ")
	if err != nil {
		errMsg := fmt.Sprintf("Failed to serialize game state: %v", err)
		currentContent := m.chatViewport.View()
		m.chatViewport.SetContent(currentContent + "\n" + errorStyle.Render("Error: "+errMsg))
		m.chatViewport.GotoBottom()
		return m, nil
	}

	// Write file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		errMsg := fmt.Sprintf("Failed to save: %v", err)
		currentContent := m.chatViewport.View()
		m.chatViewport.SetContent(currentContent + "\n" + errorStyle.Render("Error: "+errMsg))
		m.chatViewport.GotoBottom()
		return m, nil
	}

	// Show success message in chat
	successMsg := fmt.Sprintf("Game state saved to: %s", filepath)
	currentContent := m.chatViewport.View()
	m.chatViewport.SetContent(currentContent + "\n" + narratorStyle.Render("✓ "+successMsg))
	m.chatViewport.GotoBottom()

	return m, nil
}

// sanitizeFilename removes or replaces characters that are invalid in filenames
func sanitizeFilename(s string) string {
	// Replace spaces with underscores
	s = strings.ReplaceAll(s, " ", "_")
	// Remove common problematic characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		s = strings.ReplaceAll(s, char, "")
	}
	return strings.ToLower(s)
}
