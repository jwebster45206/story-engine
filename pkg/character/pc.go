package character

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// Stats5e holds the six named ability scores.
type Stats5e struct {
	Strength     int `json:"strength"`
	Dexterity    int `json:"dexterity"`
	Constitution int `json:"constitution"`
	Intelligence int `json:"intelligence"`
	Wisdom       int `json:"wisdom"`
	Charisma     int `json:"charisma"`
}

// ToAttributes converts Stats5e to a map keyed by ability name.
func (s *Stats5e) ToAttributes() map[string]int {
	return map[string]int{
		"strength":     s.Strength,
		"dexterity":    s.Dexterity,
		"constitution": s.Constitution,
		"intelligence": s.Intelligence,
		"wisdom":       s.Wisdom,
		"charisma":     s.Charisma,
	}
}

// PC is a player character. Narrative fields (prompts, class, race) live here.
// TODO: project a d20.Actor at the roll site when combat exists.
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
	Stats              Stats5e                          `json:"stats,omitzero"`
	HP                 int                              `json:"hp,omitempty"`
	MaxHP              int                              `json:"max_hp,omitempty"`
	AC                 int                              `json:"ac,omitempty"`
	CombatModifiers    map[string]int                   `json:"combat_modifiers,omitempty"`
	Attributes         map[string]int                   `json:"attributes,omitempty"`
	Inventory          []string                         `json:"inventory,omitempty"`
}

// BuildPrompt constructs the player character section for the system prompt.
// Returns an empty string if pc is nil.
//
// Example output:
// The user is controlling: Sir Galahad (he/him), Level 5 Paladin.A brave knight of the Round Table, clad in shining armor and wielding a mighty sword.
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
