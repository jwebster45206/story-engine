package worker

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const (
	defaultDC       = 10
	attemptScopeAll = "all"
)

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
	if ruling == nil {
		return nil
	}
	if !ruling.Allowed {
		return []resolvedAttempt{{
			Content: strings.TrimSpace(ruling.Reasoning),
			Success: false,
			Scope:   attemptScopeAll,
		}}
	}
	switch ruling.Scope {
	case chat.RulingScopeCombat:
		return p.resolveCombatAttempts(gs, ruling)
	default:
		return nil
	}
}

func pcDisplayName(gs *state.GameState) string {
	if gs != nil && gs.PC != nil {
		if name := strings.TrimSpace(gs.PC.Name); name != "" {
			return name
		}
	}
	return "PC"
}

// pendingCombatReaction builds a strike-back ruling after a PC combat turn.
// Non-PC subjects do not chain. Empty object does not enqueue.
func pendingCombatReaction(gs *state.GameState, ruling *chat.Ruling) *chat.Ruling {
	if ruling == nil || !ruling.Allowed || ruling.Scope != chat.RulingScopeCombat {
		return nil
	}
	if !ruling.IsPCSubject() {
		return nil
	}
	obj := strings.TrimSpace(ruling.Object)
	if obj == "" {
		return nil
	}
	return &chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeCombat,
		Subject: obj,
		Object:  pcDisplayName(gs),
	}
}

func actorPresentAtLocation(gs *state.GameState, name string) bool {
	name = strings.TrimSpace(name)
	if gs == nil || name == "" {
		return false
	}
	for key, npc := range gs.NPCs {
		if npc.Location != gs.Location {
			continue
		}
		if strings.EqualFold(npc.Name, name) || strings.EqualFold(key, name) {
			return true
		}
	}
	loc, ok := gs.WorldLocations[gs.Location]
	if !ok {
		return false
	}
	for id, m := range loc.Monsters {
		if m == nil || m.HP <= 0 {
			continue
		}
		if strings.EqualFold(m.Name, name) || strings.EqualFold(id, name) || strings.EqualFold(m.ID, name) {
			return true
		}
	}
	return false
}

func (p *ChatOrchestrator) resolveCombatAttempts(gs *state.GameState, ruling *chat.Ruling) []resolvedAttempt {
	actor := ruling.ActorName(pcDisplayName(gs))
	target := ruling.TargetName(pcDisplayName(gs))

	var out []resolvedAttempt
	if a := p.attempt(actor, target); a.NarratorText != "" {
		a.Scope = ruling.Scope
		out = append(out, a)
	}
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
