package actor

import "testing"

func TestNewMonster(t *testing.T) {
	t.Run("creates monster from template", func(t *testing.T) {
		base := &Monster{
			Name:        "Giant Rat",
			Description: "A filthy rodent",
			AC:          12,
			MaxHP:       9,
			Attributes: map[string]int{
				"strength": 7,
			},
		}

		m := NewMonster("rat_1", base, "cellar")

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
		base := &Monster{
			Name:  "Wolf",
			MaxHP: 20,
			HP:    0,
		}

		m := NewMonster("wolf_1", base, "forest")

		if m.HP != 20 {
			t.Errorf("expected HP to be 20, got %d", m.HP)
		}
	})

	t.Run("preserves HP when already set", func(t *testing.T) {
		base := &Monster{
			Name:  "Wolf",
			MaxHP: 20,
			HP:    15,
		}

		m := NewMonster("wolf_1", base, "forest")

		if m.HP != 15 {
			t.Errorf("expected HP to be 15, got %d", m.HP)
		}
	})

	t.Run("clamps negative HP to 0", func(t *testing.T) {
		base := &Monster{
			Name:  "Skeleton",
			MaxHP: 10,
			HP:    -5,
		}

		m := NewMonster("skeleton_1", base, "crypt")

		if m.HP != 0 {
			t.Errorf("expected HP to be clamped to 0, got %d", m.HP)
		}
	})

	t.Run("clamps negative AC to 0", func(t *testing.T) {
		base := &Monster{
			Name:  "Blob",
			AC:    -3,
			MaxHP: 5,
		}

		m := NewMonster("blob_1", base, "cave")

		if m.AC != 0 {
			t.Errorf("expected AC to be clamped to 0, got %d", m.AC)
		}
	})

	t.Run("returns nil for nil base", func(t *testing.T) {
		m := NewMonster("test_1", nil, "location")

		if m != nil {
			t.Error("expected nil for nil base template")
		}
	})

	t.Run("copies attributes and items", func(t *testing.T) {
		base := &Monster{
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

		m := NewMonster("goblin_1", base, "cave")

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
