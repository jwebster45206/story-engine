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
		if a := p.resolveCombatAttempt(gs, ruling); a.NarratorText != "" {
			return []resolvedAttempt{a}
		}
		return nil
	default:
		return nil
	}
}

func pendingCombatReaction(gs *state.GameState, ruling *chat.Ruling) *chat.Ruling {
	if ruling == nil || !ruling.Allowed || ruling.Scope != chat.RulingScopeCombat {
		return nil
	}
	if !ruling.IsPCSubject(gs.PCName()) {
		return nil
	}
	obj := strings.TrimSpace(ruling.Object)
	if obj == "" {
		return nil
	}
	return new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeCombat,
		Subject: obj,
		Object:  gs.PCName(),
	})
}

func actorPresentAtLocation(gs *state.GameState, name string) bool {
	name = strings.TrimSpace(name)
	if gs == nil || name == "" {
		return false
	}
	for key, npc := range gs.NPCs {
		if npc == nil || npc.Location != gs.Location {
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
		if m == nil {
			continue
		}
		if strings.EqualFold(m.Name, name) || strings.EqualFold(id, name) || strings.EqualFold(m.ID, name) {
			return true
		}
	}
	return false
}

func (p *ChatOrchestrator) resolveCombatAttempt(gs *state.GameState, ruling *chat.Ruling) resolvedAttempt {
	pcName := gs.PCName()
	target := ruling.TargetName(pcName)
	if target == "" {
		return resolvedAttempt{}
	}
	a := p.attempt(ruling.ActorName(pcName), target)
	a.Scope = ruling.Scope
	return a
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
