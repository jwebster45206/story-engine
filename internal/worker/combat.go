package worker

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const combatDC = 10

var d20Die = mustDie(1, 20)

func mustDie(count, faces uint) d20.Dice {
	d, err := d20.NewDice(count, faces)
	if err != nil {
		panic(err)
	}
	return d
}

func strikeLine(attacker, target string, hit bool) string {
	result := "misses"
	if hit {
		result = "hits"
	}
	if target == "" {
		return fmt.Sprintf("%s strikes and %s.", attacker, result)
	}
	return fmt.Sprintf("%s strikes %s and %s.", attacker, target, result)
}

func (p *ChatProcessor) combatStrikeLines(gs *state.GameState, ruling *chat.Ruling) string {
	if ruling == nil || !ruling.Allowed || ruling.Scope != chat.RulingScopeCombat {
		return ""
	}
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
	if line := p.rollStrike(pcName, target); line != "" {
		lines = append(lines, line)
	}
	if target != "" {
		if line := p.rollStrike(target, pcName); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func (p *ChatProcessor) rollStrike(attacker, target string) string {
	if p == nil || p.roller == nil {
		return ""
	}
	outcome, err := p.roller.Roll(d20Die)
	if err != nil {
		if p.logger != nil {
			p.logger.Error("combat roll failed", "error", err, "attacker", attacker)
		}
		return ""
	}
	hit := outcome.Value >= combatDC
	line := strikeLine(attacker, target, hit)
	// TODO: emit outcome.Detail() as a game SSE event (internal/events.Broadcaster).
	// Do not send dice breakdown or DC to the narrator.
	if p.logger != nil {
		p.logger.Info(line, "detail", outcome.Detail())
	}
	return line
}
