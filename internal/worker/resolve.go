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
	actionHeal      = "heal"
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
	if ruling == nil || gs == nil || !ruling.Allowed || ruling.Scope != chat.RulingScopeCombat {
		return nil
	}
	if !ruling.IsPCSubject(gs.PCName()) {
		return nil
	}
	obj := strings.TrimSpace(ruling.Object)
	if obj == "" || !actionHasEffect(gs, ruling) {
		return nil
	}
	return new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeCombat,
		Subject: obj,
		Object:  gs.PCName(),
	})
}

func actionHasEffect(gs *state.GameState, ruling *chat.Ruling) bool {
	if gs == nil || ruling == nil {
		return false
	}
	name := ruling.ActorName(gs.PCName())
	actor, ok := gs.FindCombatant(name)
	if !ok {
		return false
	}
	_, action, picked, err := chosenAction(actor, ruling, name)
	if err != nil {
		return false
	}
	return picked && action.Effect.Count > 0
}

// reactionAction is the attack a counterattack will roll. The queued prompt
// and the later attempt both use it, so the narrator sees the same action.
func reactionAction(gs *state.GameState, actorName string) (id, name string) {
	if gs == nil {
		return "", ""
	}
	c, ok := gs.FindCombatant(actorName)
	if !ok {
		return "", ""
	}
	actor, err := c.Stats.ToActor(actorName, actorName)
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
	targetName := ruling.TargetName(pcName)
	if targetName == "" {
		return resolvedAttempt{}
	}
	actorName := ruling.ActorName(pcName)
	var actor, target state.Combatant
	dc := defaultDC
	if gs != nil {
		actor, _ = gs.FindCombatant(actorName)
		var targetOK bool
		target, targetOK = gs.FindCombatant(targetName)
		if targetOK && target.Stats != nil && target.Stats.AC > 0 {
			dc = target.Stats.AC
		}
	}

	d20Actor, action, picked, err := chosenAction(actor, ruling, actorName)
	if err != nil && p != nil && p.logger != nil {
		p.logger.Error("actor projection failed", "error", err, "actor", actorName)
	}
	actionName := ""
	auto := false
	die := d20Die
	if err == nil && picked {
		actionName = action.Name
		if action.Attempt.Count == 0 {
			auto = true
		} else if d20Actor != nil {
			die = d20Actor.Dice(action.Attempt, vocab.Striking)
		}
	}

	if auto {
		return p.finishAttempt(gs, actor, target, d20Actor, action, actorName, targetName, "", ruling.Scope)
	}
	if p == nil || p.roller == nil {
		return resolvedAttempt{}
	}
	outcome, err := p.roller.Roll(die)
	if err != nil {
		if p.logger != nil {
			p.logger.Error("attempt roll failed", "error", err, "actor", actorName)
		}
		return resolvedAttempt{}
	}
	hit := outcome.Value >= dc
	content := rollContent(actorName, actionName, outcome.Detail(), dc, hit)
	if !hit {
		text := outcomeLine(actorName, targetName, actionName, false)
		if p.logger != nil {
			p.logger.Info(text, "content", content)
		}
		return resolvedAttempt{NarratorText: text, Content: content, Success: false, Scope: ruling.Scope}
	}
	return p.finishAttempt(gs, actor, target, d20Actor, action, actorName, targetName, content, ruling.Scope)
}

// attempt rolls a bare 1d20 against defaultDC. Used when the actor has no action.
func (p *ChatOrchestrator) attempt(actor, target string) resolvedAttempt {
	return p.rollAttempt(actor, target, "", d20Die, defaultDC)
}

// chosenAction projects the combatant and picks the ruling's action.
// ok is false when the actor has no usable action, which is a bare 1d20.
func chosenAction(c state.Combatant, ruling *chat.Ruling, actorName string) (*d20.Actor, d20.Action, bool, error) {
	if ruling == nil || c.Stats == nil {
		return nil, d20.Action{}, false, nil
	}
	actor, err := c.Stats.ToActor(actorName, actorName)
	if err != nil {
		return nil, d20.Action{}, false, err
	}
	action, ok := pickAction(actor, ruling.ActionID)
	if !ok {
		return actor, d20.Action{}, false, nil
	}
	return actor, action, true, nil
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

// finishAttempt writes the hit line, and on a damaging effect updates HP.
// content is the attempt dice line, empty when the attempt was automatic.
func (p *ChatOrchestrator) finishAttempt(gs *state.GameState, actor, target state.Combatant, d20Actor *d20.Actor, action d20.Action, actorName, targetName, content string, scope chat.RulingScope) resolvedAttempt {
	text := outcomeLine(actorName, targetName, action.Name, true)
	wound, clause := p.applyActionEffect(gs, actor, target, d20Actor, action)
	text += wound
	if clause != "" {
		switch {
		case content != "":
			content += "; " + clause
		case action.Name != "":
			content = action.Name + ": " + clause
		default:
			content = clause
		}
	}
	if p != nil && p.logger != nil {
		p.logger.Info(text, "content", content)
	}
	return resolvedAttempt{NarratorText: text, Content: content, Success: true, Scope: scope}
}

// applyActionEffect rolls Effect plus the damage modifier and writes HP.
// Healing lands on the actor. Damage lands on the target. An empty effect,
// a miss, or a missing recipient changes nothing.
func (p *ChatOrchestrator) applyActionEffect(gs *state.GameState, actor, target state.Combatant, d20Actor *d20.Actor, action d20.Action) (wound, clause string) {
	if action.Effect.Count == 0 || d20Actor == nil || p == nil || p.roller == nil {
		return "", ""
	}
	recipient := target
	if action.Type == actionHeal {
		recipient = actor
	}
	if recipient.Stats == nil {
		return "", ""
	}
	outcome, err := p.roller.Roll(d20Actor.Dice(action.Effect, vocab.Damage))
	if err != nil {
		if p.logger != nil {
			p.logger.Error("effect roll failed", "error", err, "actor", actor.Display)
		}
		return "", ""
	}
	if action.Type == actionHeal {
		hp := recipient.Stats.ApplyHPEffect(-outcome.Value)
		return "", hpClause("healing", recipient.Display, outcome.Value, hp, recipient.Stats.MaxHP)
	}
	before := recipient.Stats.HP
	hp := recipient.Stats.ApplyHPEffect(outcome.Value)
	knocked := before > 0 && hp <= 0
	if knocked && gs != nil {
		gs.KnockOut(recipient)
	}
	if outcome.Value > 0 && hp < before {
		wound = woundSentence(recipient.Display, hp, recipient.Stats.MaxHP, knocked)
	}
	return wound, hpClause("damage", recipient.Display, outcome.Value, hp, recipient.Stats.MaxHP)
}

func woundSentence(name string, hp, maxHP int, knocked bool) string {
	if knocked {
		return fmt.Sprintf(" The %s is knocked out.", name)
	}
	if maxHP <= 0 || hp <= 0 {
		return ""
	}
	band := "wounded"
	if hp*2 <= maxHP {
		band = "badly wounded"
	}
	return fmt.Sprintf(" The %s is %s.", name, band)
}

func hpClause(kind, target string, amount, hp, maxHP int) string {
	switch {
	case maxHP > 0:
		return fmt.Sprintf("%d %s (%s %d/%d)", amount, kind, target, hp, maxHP)
	case hp <= 0:
		return fmt.Sprintf("%d %s (%s knocked out)", amount, kind, target)
	default:
		return fmt.Sprintf("%d %s (%s %d)", amount, kind, target, hp)
	}
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
