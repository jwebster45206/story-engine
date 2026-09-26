package worker

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/d20/vocab"
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

func outcomeLine(actor, target, actionName string, hit bool) string {
	result := "misses"
	if hit {
		result = "hits"
	}
	if actionName == "" {
		return fmt.Sprintf("%s strikes %s and %s.", actor, target, result)
	}
	return fmt.Sprintf("%s strikes %s with %s and %s.", actor, target, actionName, result)
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
		if a := p.resolveActionAttempt(gs, ruling); a.NarratorText != "" {
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

// reactionAction is the attack a counterattack will roll. The queued prompt
// and the later attempt both use it, so the narrator sees the same action.
func reactionAction(gs *state.GameState, actorName string) (id, name string) {
	if gs == nil {
		return "", ""
	}
	stats, _, ok := gs.FindActor(actorName)
	if !ok {
		return "", ""
	}
	actor, err := stats.ToActor(actorName, actorName)
	if err != nil {
		return "", ""
	}
	action, ok := pickAction(actor, "")
	if !ok {
		return "", ""
	}
	name = action.Name
	if name == "" {
		name = action.ID
	}
	return action.ID, name
}

func reactionPrompt(actor, target, actionName string) string {
	if actionName == "" {
		return fmt.Sprintf("%s strikes %s.", actor, target)
	}
	return fmt.Sprintf("%s strikes %s with %s.", actor, target, actionName)
}

func (p *ChatOrchestrator) resolveActionAttempt(gs *state.GameState, ruling *chat.Ruling) resolvedAttempt {
	pcName := ""
	if gs != nil {
		pcName = gs.PCName()
	}
	target := ruling.TargetName(pcName)
	if target == "" {
		return resolvedAttempt{}
	}
	actorName := ruling.ActorName(pcName)
	dc := attemptDC(gs, target)
	die, actionName, auto := p.attemptDice(gs, ruling, actorName)
	if auto {
		text := outcomeLine(actorName, target, actionName, true)
		if p != nil && p.logger != nil {
			p.logger.Info(text)
		}
		return resolvedAttempt{NarratorText: text, Success: true, Scope: ruling.Scope}
	}
	a := p.rollAttempt(actorName, target, actionName, die, dc)
	a.Scope = ruling.Scope
	return a
}

// attempt rolls a bare 1d20 against defaultDC. Used when the actor has no action.
func (p *ChatOrchestrator) attempt(actor, target string) resolvedAttempt {
	return p.rollAttempt(actor, target, "", d20Die, defaultDC)
}

func (p *ChatOrchestrator) attemptDice(gs *state.GameState, ruling *chat.Ruling, actorName string) (die d20.Dice, actionName string, auto bool) {
	die = d20Die
	if gs == nil || ruling == nil {
		return die, "", false
	}
	stats, _, ok := gs.FindActor(actorName)
	if !ok {
		return die, "", false
	}
	d20Actor, err := stats.ToActor(actorName, actorName)
	if err != nil {
		if p != nil && p.logger != nil {
			p.logger.Error("actor projection failed", "error", err, "actor", actorName)
		}
		return die, "", false
	}
	action, ok := pickAction(d20Actor, ruling.ActionID)
	if !ok {
		return die, "", false
	}
	if action.Attempt.Count == 0 {
		return d20.Dice{}, action.Name, true
	}
	return d20Actor.Dice(action.Attempt, vocab.Striking), action.Name, false
}

func attemptDC(gs *state.GameState, target string) int {
	if gs == nil {
		return defaultDC
	}
	stats, _, ok := gs.FindActor(target)
	if !ok || stats.AC <= 0 {
		return defaultDC
	}
	return stats.AC
}

// pickAction uses actionID when the actor has it. Otherwise it uses the
// lowest sorted attack. A missing menu returns false so the caller rolls 1d20.
func pickAction(actor *d20.Actor, actionID string) (d20.Action, bool) {
	if actor == nil {
		return d20.Action{}, false
	}
	if id := strings.TrimSpace(actionID); id != "" {
		if act, ok := actor.Action(id); ok {
			return act, true
		}
	}
	ids := slices.Sorted(maps.Keys(actor.Actions))
	for _, id := range ids {
		if actor.Actions[id].Type == vocab.Attack {
			return actor.Actions[id], true
		}
	}
	return d20.Action{}, false
}

func (p *ChatOrchestrator) rollAttempt(actor, target, actionName string, die d20.Dice, dc int) resolvedAttempt {
	if p == nil || p.roller == nil {
		return resolvedAttempt{}
	}
	outcome, err := p.roller.Roll(die)
	if err != nil {
		if p.logger != nil {
			p.logger.Error("attempt roll failed", "error", err, "actor", actor)
		}
		return resolvedAttempt{}
	}
	success := outcome.Value >= dc
	text := outcomeLine(actor, target, actionName, success)
	content := rollContent(actor, actionName, outcome.Detail(), dc, success)
	if p.logger != nil {
		p.logger.Info(text, "content", content)
	}
	return resolvedAttempt{NarratorText: text, Content: content, Success: success}
}

// rollContent is the player-facing dice line. Detail's numeric *Result* is
// replaced with hit or miss, and the difficulty is written out as AC.
func rollContent(actor, actionName, detail string, dc int, hit bool) string {
	if strings.HasPrefix(detail, "R") {
		detail = "r" + detail[1:]
	}
	if i := strings.LastIndex(detail, "; *Result:"); i >= 0 {
		detail = detail[:i]
	}
	result := "Miss"
	if hit {
		result = "Hit"
	}
	roll := fmt.Sprintf("%s %s; AC %d; *Result %s*", actor, detail, dc, result)
	if actionName == "" {
		return roll
	}
	return actionName + ": " + roll
}
