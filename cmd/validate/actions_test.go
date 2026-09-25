package main

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/character"
)

func TestValidateActorActions_RejectsGarbage(t *testing.T) {
	v := &ScenarioValidator{}
	v.validateLocationMonsters(map[string]*character.Monster{
		"rat_1": {
			TemplateID: "rat",
			Actions: map[string]character.Action{
				"bite": {Attempt: "not-dice"},
			},
		},
	}, "cellar", "scenario")

	if len(v.errors) == 0 {
		t.Fatal("expected a notation error")
	}
	joined := strings.Join(v.errors, "\n")
	if !strings.Contains(joined, "not-dice") {
		t.Fatalf("errors = %s", joined)
	}
}
