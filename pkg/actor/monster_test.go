package actor

import "testing"

func TestNewMonster(t *testing.T) {
	t.Run("creates monster from template", func(t *testing.T) {
		template := &Monster{
			Name:        "Giant Rat",
			Description: "A filthy rodent",
			AC:          12,
			MaxHP:       9,
			Attributes: map[string]int{
				"strength": 7,
			},
		}

		overrides := &Monster{
			ID:       "rat_1",
			Location: "cellar",
		}

		m := NewMonster(template, overrides)

		if m.ID != "rat_1" {
			t.Errorf("expected ID 'rat_1', got '%s'", m.ID)
		}
		if m.Location != "cellar" {
			t.Errorf("expected location 'cellar', got '%s'", m.Location)
		}
		if m.Name != "Giant Rat" {
			t.Errorf("expected name 'Giant Rat', got '%s'", m.Name)
		}
	})

	t.Run("sets HP from MaxHP when HP is 0", func(t *testing.T) {
		template := &Monster{
			Name:  "Wolf",
			MaxHP: 20,
			HP:    0,
		}

		overrides := &Monster{
			ID:       "wolf_1",
			Location: "forest",
		}

		m := NewMonster(template, overrides)

		if m.HP != 20 {
			t.Errorf("expected HP to be 20, got %d", m.HP)
		}
	})

	t.Run("preserves HP when already set in template", func(t *testing.T) {
		template := &Monster{
			Name:  "Wolf",
			MaxHP: 20,
			HP:    15,
		}

		overrides := &Monster{
			ID:       "wolf_1",
			Location: "forest",
		}

		m := NewMonster(template, overrides)

		if m.HP != 15 {
			t.Errorf("expected HP to be 15, got %d", m.HP)
		}
	})

	t.Run("overrides HP when specified", func(t *testing.T) {
		template := &Monster{
			Name:  "Wolf",
			MaxHP: 20,
			HP:    15,
		}

		overrides := &Monster{
			ID:       "wolf_1",
			Location: "forest",
			HP:       10,
		}

		m := NewMonster(template, overrides)

		if m.HP != 10 {
			t.Errorf("expected HP to be 10 (overridden), got %d", m.HP)
		}
	})

	t.Run("returns nil for nil template", func(t *testing.T) {
		overrides := &Monster{
			ID:       "test_1",
			Location: "location",
		}

		m := NewMonster(nil, overrides)

		if m != nil {
			t.Error("expected nil for nil template")
		}
	})

	t.Run("returns nil for nil overrides", func(t *testing.T) {
		template := &Monster{
			Name:  "Test",
			MaxHP: 10,
		}

		m := NewMonster(template, nil)

		if m != nil {
			t.Error("expected nil for nil overrides")
		}
	})

	t.Run("copies attributes and items", func(t *testing.T) {
		template := &Monster{
			Name:  "Goblin",
			MaxHP: 7,
			Attributes: map[string]int{
				"strength":  8,
				"dexterity": 14,
			},
			CombatMods: map[string]int{
				"dagger": 4,
			},
			Items: []string{"dagger", "gold_coin"},
		}

		overrides := &Monster{
			ID:       "goblin_1",
			Location: "cave",
		}

		m := NewMonster(template, overrides)

		if len(m.Attributes) != 2 {
			t.Errorf("expected 2 attributes, got %d", len(m.Attributes))
		}
		if m.Attributes["strength"] != 8 {
			t.Errorf("expected strength 8, got %d", m.Attributes["strength"])
		}
		if len(m.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(m.Items))
		}
	})

	t.Run("overrides template fields", func(t *testing.T) {
		template := &Monster{
			Name:        "Giant Rat",
			Description: "A filthy rodent",
			AC:          12,
			MaxHP:       9,
		}

		overrides := &Monster{
			ID:          "rat_boss",
			Location:    "cellar",
			Name:        "Rat King",
			Description: "An enormous rat with a crown",
			AC:          15,
			MaxHP:       20,
		}

		m := NewMonster(template, overrides)

		if m.Name != "Rat King" {
			t.Errorf("expected name 'Rat King', got '%s'", m.Name)
		}
		if m.Description != "An enormous rat with a crown" {
			t.Errorf("expected overridden description, got '%s'", m.Description)
		}
		if m.AC != 15 {
			t.Errorf("expected AC 15, got %d", m.AC)
		}
		if m.MaxHP != 20 {
			t.Errorf("expected MaxHP 20, got %d", m.MaxHP)
		}
	})
}

func TestMonster_TakeDamage(t *testing.T) {
	t.Run("reduces HP by damage amount", func(t *testing.T) {
		m := &Monster{HP: 20, MaxHP: 20}
		m.TakeDamage(5)

		if m.HP != 15 {
			t.Errorf("expected HP 15, got %d", m.HP)
		}
	})

	t.Run("clamps HP at 0", func(t *testing.T) {
		m := &Monster{HP: 5, MaxHP: 20}
		m.TakeDamage(10)

		if m.HP != 0 {
			t.Errorf("expected HP to be clamped at 0, got %d", m.HP)
		}
	})

	t.Run("ignores 0 damage", func(t *testing.T) {
		m := &Monster{HP: 20, MaxHP: 20}
		m.TakeDamage(0)

		if m.HP != 20 {
			t.Errorf("expected HP to remain 20, got %d", m.HP)
		}
	})

	t.Run("ignores negative damage", func(t *testing.T) {
		m := &Monster{HP: 20, MaxHP: 20}
		m.TakeDamage(-5)

		if m.HP != 20 {
			t.Errorf("expected HP to remain 20, got %d", m.HP)
		}
	})
}

func TestMonster_Heal(t *testing.T) {
	t.Run("increases HP by heal amount", func(t *testing.T) {
		m := &Monster{HP: 10, MaxHP: 20}
		m.Heal(5)

		if m.HP != 15 {
			t.Errorf("expected HP 15, got %d", m.HP)
		}
	})

	t.Run("clamps HP at MaxHP", func(t *testing.T) {
		m := &Monster{HP: 18, MaxHP: 20}
		m.Heal(5)

		if m.HP != 20 {
			t.Errorf("expected HP to be clamped at MaxHP (20), got %d", m.HP)
		}
	})

	t.Run("ignores 0 healing", func(t *testing.T) {
		m := &Monster{HP: 10, MaxHP: 20}
		m.Heal(0)

		if m.HP != 10 {
			t.Errorf("expected HP to remain 10, got %d", m.HP)
		}
	})

	t.Run("ignores negative healing", func(t *testing.T) {
		m := &Monster{HP: 10, MaxHP: 20}
		m.Heal(-5)

		if m.HP != 10 {
			t.Errorf("expected HP to remain 10, got %d", m.HP)
		}
	})
}

func TestMonster_IsDefeated(t *testing.T) {
	t.Run("returns true when HP is 0", func(t *testing.T) {
		m := &Monster{HP: 0, MaxHP: 20}

		if !m.IsDefeated() {
			t.Error("expected IsDefeated to be true when HP is 0")
		}
	})

	t.Run("returns true when HP is negative", func(t *testing.T) {
		m := &Monster{HP: -5, MaxHP: 20}

		if !m.IsDefeated() {
			t.Error("expected IsDefeated to be true when HP is negative")
		}
	})

	t.Run("returns false when HP is positive", func(t *testing.T) {
		m := &Monster{HP: 1, MaxHP: 20}

		if m.IsDefeated() {
			t.Error("expected IsDefeated to be false when HP is positive")
		}
	})
}

func TestMonster_MoveTo(t *testing.T) {
	t.Run("updates location", func(t *testing.T) {
		m := &Monster{Location: "cellar"}
		m.MoveTo("tavern")

		if m.Location != "tavern" {
			t.Errorf("expected location 'tavern', got '%s'", m.Location)
		}
	})

	t.Run("can set empty location", func(t *testing.T) {
		m := &Monster{Location: "cellar"}
		m.MoveTo("")

		if m.Location != "" {
			t.Errorf("expected empty location, got '%s'", m.Location)
		}
	})
}
