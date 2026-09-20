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

func strikeLine(attacker, target string, hit bool) string {
	result := "misses"
	if hit {
		result = "hits"
	}
	return fmt.Sprintf("%s strikes %s and %s.", attacker, target, result)
}

// resolveAttempts resolves the attempts of the given ruling.
// It may roll dice if the ruling needs it.
// It returns a slice of strings, each representing an attempt and its outcome.
func (p *ChatProcessor) resolveAttempts(gs *state.GameState, ruling *chat.Ruling) []string {
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

func (p *ChatProcessor) resolveCombatAttempts(gs *state.GameState, ruling *chat.Ruling) []string {
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

	var lines []string
	if line := p.attempt(pcName, target); line != "" {
		lines = append(lines, line)
	}
	// TODO: consider a reaction attempt (focused actor strikes the PC).
	return lines
}

func (p *ChatProcessor) attempt(actor, target string) string {
	if p == nil || p.roller == nil {
		return ""
	}
	outcome, err := p.roller.Roll(d20Die)
	if err != nil {
		if p.logger != nil {
			p.logger.Error("attempt roll failed", "error", err, "actor", actor)
		}
		return ""
	}
	hit := outcome.Value >= defaultDC
	line := strikeLine(actor, target, hit)
	// TODO: emit outcome.Detail() as a game SSE event (internal/events.Broadcaster).
	// Do not send dice breakdown or DC to the narrator.
	if p.logger != nil {
		p.logger.Info(line, "detail", outcome.Detail())
	}
	return line
}
