package worker

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const defaultDC = 10

var d20Die = d20.MustNewDice(1, 20)

type resolvedAttempt struct {
	NarratorText string
	Content      string
	Success      bool
	Scope        chat.RulingScope
}

func strikeLine(attacker, target string, hit bool) string {
	result := "misses"
	if hit {
		result = "hits"
	}
	return fmt.Sprintf("%s strikes %s and %s.", attacker, target, result)
}

func narratorTexts(attempts []resolvedAttempt) []string {
	if len(attempts) == 0 {
		return nil
	}
	lines := make([]string, 0, len(attempts))
	for _, a := range attempts {
		if a.NarratorText != "" {
			lines = append(lines, a.NarratorText)
		}
	}
	return lines
}

// resolveAttempts resolves the attempts of the given ruling.
// It may roll dice if the ruling needs it.
func (p *ChatOrchestrator) resolveAttempts(gs *state.GameState, ruling *chat.Ruling) []resolvedAttempt {
	if ruling == nil || !ruling.Allowed {
		return nil
	}
	switch ruling.Scope {
	case chat.RulingScopeCombat:
		return p.resolveCombatAttempts(gs, ruling)
	default:
		return nil
	}
}

func (p *ChatOrchestrator) resolveCombatAttempts(gs *state.GameState, ruling *chat.Ruling) []resolvedAttempt {
	pcName := "PC"
	if gs != nil && gs.PC != nil {
		if name := strings.TrimSpace(gs.PC.Name); name != "" {
			pcName = name
		}
	}
	target := ""
	if len(ruling.Focus.Actors) > 0 {
		target = ruling.Focus.Actors[0]
	}

	var out []resolvedAttempt
	if a := p.attempt(pcName, target); a.NarratorText != "" {
		a.Scope = ruling.Scope
		out = append(out, a)
	}
	// TODO: consider a reaction attempt (focused actor strikes the PC).
	return out
}

func (p *ChatOrchestrator) attempt(actor, target string) resolvedAttempt {
	if p == nil || p.roller == nil {
		return resolvedAttempt{}
	}
	outcome, err := p.roller.Roll(d20Die)
	if err != nil {
		if p.logger != nil {
			p.logger.Error("attempt roll failed", "error", err, "actor", actor)
		}
		return resolvedAttempt{}
	}
	success := outcome.Value >= defaultDC
	text := strikeLine(actor, target, success)
	content := rollContent(actor, outcome.Detail())
	if p.logger != nil {
		p.logger.Info(text, "content", content)
	}
	return resolvedAttempt{NarratorText: text, Content: content, Success: success}
}

func rollContent(actor, detail string) string {
	if strings.HasPrefix(detail, "R") {
		detail = "r" + detail[1:]
	}
	return actor + " " + detail
}
