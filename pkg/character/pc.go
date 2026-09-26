package character

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// PC is a player character. Narrative fields (prompts, class, race) live here.
type PC struct {
	ID                 string                           `json:"id"`
	Name               string                           `json:"name,omitempty"`
	Class              string                           `json:"class,omitempty"`
	Level              int                              `json:"level,omitempty"`
	Race               string                           `json:"race,omitempty"`
	Pronouns           string                           `json:"pronouns,omitempty"`
	Description        string                           `json:"description,omitempty"`
	Background         string                           `json:"background,omitempty"`
	OpeningPrompt      string                           `json:"opening_prompt,omitempty"`
	ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts,omitempty"`
	Inventory          []string                         `json:"inventory,omitempty"`
	Stats
}

// BuildPrompt constructs the player character section for the system prompt.
// Returns an empty string if pc is nil.
//
// Example output:
// The user is controlling: Sir Galahad (he/him), Level 5 Paladin. A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.
func BuildPrompt(pc *PC) string {
	if pc == nil {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("REMEMBER: In this game, the user is controlling: ")
	sb.WriteString(pc.Name)
	if pc.Pronouns != "" {
		fmt.Fprintf(&sb, " (%s)", pc.Pronouns)
	}
	if pc.Level > 0 || pc.Class != "" || pc.Race != "" {
		summaryParts := []string{}
		if pc.Level > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("Level %d", pc.Level))
		}
		if pc.Race != "" {
			summaryParts = append(summaryParts, pc.Race)
		}
		if pc.Class != "" {
			summaryParts = append(summaryParts, pc.Class)
		}
		sb.WriteString(", " + strings.Join(summaryParts, " "))
	}
	if pc.Description != "" {
		sb.WriteString(". " + pc.Description)
	}
	return sb.String()
}
