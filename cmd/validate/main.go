package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <scenario.json>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	validator := &ScenarioValidator{}

	if err := validator.validateFile(filename); err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Scenario file is valid!")
}

type ScenarioValidator struct {
	errors []string
}

func (v *ScenarioValidator) validateFile(filename string) error {
	fmt.Printf("Validating %s...\n", filename)

	// Validate filename format
	baseName := filepath.Base(filename)
	if !strings.HasSuffix(baseName, ".json") {
		return fmt.Errorf("scenario file must have .json extension: %s", baseName)
	}

	nameWithoutExt := strings.TrimSuffix(baseName, ".json")
	if !isValidScenarioFilename(nameWithoutExt) {
		return fmt.Errorf("scenario filename '%s' must be lowercase snake_case (e.g., my_scenario.json, not my-scenario.json or MyScenario.json)", baseName)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	v.errors = nil

	if !json.Valid(data) {
		return fmt.Errorf("file %s contains invalid JSON", filename)
	}

	var s scenario.Scenario
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&s); err != nil {
		return fmt.Errorf("file %s failed strict JSON unmarshaling: %w", filename, err)
	}

	v.validateScenario(&s, filename)

	if len(v.errors) > 0 {
		return fmt.Errorf("validation errors in %s:\n%s", filename, strings.Join(v.errors, "\n"))
	}

	return nil
}

func (v *ScenarioValidator) validateScenario(s *scenario.Scenario, filename string) {
	// Validate opening_scene ID
	v.validateIDFormat("opening_scene", s.OpeningScene)

	// Validate location IDs
	for locationID := range s.Locations {
		v.validateIDFormat("location ID", locationID)
	}

	// Validate NPC IDs
	for npcID := range s.NPCs {
		v.validateIDFormat("NPC ID", npcID)
	}

	// Validate scene IDs and their contents
	for sceneID, scene := range s.Scenes {
		v.validateIDFormat("scene ID", sceneID)
		v.validateScene(&scene, sceneID)
	}

	for _, cp := range s.ContingencyPrompts {
		v.validateContingencyPrompt(&cp)
	}
}

func (v *ScenarioValidator) validateScene(scene *scenario.Scene, sceneID string) {
	// Validate location IDs within the scene
	for locationID := range scene.Locations {
		v.validateIDFormat("scene location ID", locationID)
	}

	// Validate NPC IDs within the scene
	for npcID := range scene.NPCs {
		v.validateIDFormat("scene NPC ID", npcID)
	}

	// Validate conditional keys (map keys are the conditional IDs)
	for conditionalKey, conditional := range scene.Conditionals {
		v.validateIDFormat("conditional key", conditionalKey)
		v.validateConditional(&conditional, sceneID, conditionalKey)
	}

	for _, cp := range scene.ContingencyPrompts {
		v.validateContingencyPrompt(&cp)
	}
}

func (v *ScenarioValidator) validateConditional(conditional *scenario.Conditional, sceneID string, conditionalKey string) {
	v.validateConditionalWhen(&conditional.When, fmt.Sprintf("conditional %s in scene %s", conditionalKey, sceneID), conditionalKey)

	// Validate Then clause has at least one action
	actionCount := 0
	if conditional.Then.SceneChange != nil && conditional.Then.SceneChange.To != "" {
		v.validateIDFormat("conditional then scene", conditional.Then.SceneChange.To)
		actionCount++
	}
	if conditional.Then.GameEnded != nil {
		actionCount++
	}
	if conditional.Then.Prompt != nil {
		if strings.TrimSpace(*conditional.Then.Prompt) == "" {
			v.addError(fmt.Sprintf("conditional %s in scene %s has empty prompt", conditionalKey, sceneID))
		}
		actionCount++
	}
	if len(conditional.Then.SetVars) > 0 {
		for varName := range conditional.Then.SetVars {
			if !isValidVariableName(varName) {
				v.addError(fmt.Sprintf("conditional %s in scene %s has invalid variable name '%s' in then.set_vars - should be lowercase snake_case", conditionalKey, sceneID, varName))
			}
		}
		actionCount++
	}
	if len(conditional.Then.ItemEvents) > 0 {
		actionCount++
	}
	if len(conditional.Then.NPCEvents) > 0 {
		actionCount++
		// Validate NPC event structure
		for _, npcEvent := range conditional.Then.NPCEvents {
			if npcEvent.NPCID == "" {
				v.addError(fmt.Sprintf("conditional %s in scene %s has NPC event with empty npc_id", conditionalKey, sceneID))
			}
			if npcEvent.LocationChange == nil {
				v.addError(fmt.Sprintf("conditional %s in scene %s has NPC event with no location_change", conditionalKey, sceneID))
			} else {
				if npcEvent.LocationChange.To == "" {
					v.addError(fmt.Sprintf("conditional %s in scene %s has NPC event with empty location_change.to", conditionalKey, sceneID))
				} else {
					v.validateIDFormat(fmt.Sprintf("conditional %s npc_event location_change.to", conditionalKey), npcEvent.LocationChange.To)
				}
			}
		}
	}
	if len(conditional.Then.LocationEvents) > 0 {
		actionCount++
		// Validate location event structure
		for _, locEvent := range conditional.Then.LocationEvents {
			if locEvent.LocationID == "" {
				v.addError(fmt.Sprintf("conditional %s in scene %s has location event with empty location_id", conditionalKey, sceneID))
			} else {
				v.validateIDFormat(fmt.Sprintf("conditional %s location_event location_id", conditionalKey), locEvent.LocationID)
			}
			if len(locEvent.ExitChanges) == 0 {
				v.addError(fmt.Sprintf("conditional %s in scene %s has location event with no exit_changes", conditionalKey, sceneID))
			}
			for _, exitChange := range locEvent.ExitChanges {
				if exitChange.ExitID == "" {
					v.addError(fmt.Sprintf("conditional %s in scene %s has exit change with empty exit_id", conditionalKey, sceneID))
				}
				if exitChange.Status != "blocked" && exitChange.Status != "unblocked" {
					v.addError(fmt.Sprintf("conditional %s in scene %s has exit change with invalid status '%s' (must be 'blocked' or 'unblocked')", conditionalKey, sceneID, exitChange.Status))
				}
			}
		}
	}
	if conditional.Then.UserLocation != "" {
		v.validateIDFormat("conditional then user_location", conditional.Then.UserLocation)
		actionCount++
	}

	if actionCount == 0 {
		v.addError(fmt.Sprintf("conditional %s in scene %s has no action in 'then' clause", conditionalKey, sceneID))
	}
}

func (v *ScenarioValidator) validateContingencyPrompt(cp *conditionals.ContingencyPrompt) {
	if cp.When != nil {
		v.validateConditionalWhen(cp.When, "contingency prompt", cp.Prompt)
	}
}

func (v *ScenarioValidator) validateConditionalWhen(when *conditionals.ConditionalWhen, context string, prompt string) {
	if len(when.Vars) == 0 && when.SceneTurnCounter == nil && when.TurnCounter == nil &&
		when.Location == "" && when.MinSceneTurns == nil && when.MinTurns == nil {
		v.addError(fmt.Sprintf("%s has empty 'when' clause - no conditions specified (%s)", context, prompt))
		return
	}

	if len(when.Vars) > 0 {
		for varName := range when.Vars {
			if !isValidVariableName(varName) {
				v.addError(fmt.Sprintf("%s has invalid variable name '%s' - should be lowercase snake_case", context, varName))
			}
		}
	}

	if when.Location != "" {
		v.validateIDFormat("when location", when.Location)
	}
}

func (v *ScenarioValidator) validateIDFormat(fieldName, id string) {
	if id == "" {
		return
	}

	if !isValidID(id) {
		v.addError(fmt.Sprintf("%s '%s' should be lowercase snake_case", fieldName, id))
	}
}

func (v *ScenarioValidator) addError(msg string) {
	v.errors = append(v.errors, "  - "+msg)
}

var (
	validIDRegex       = regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$|^[a-z]$`)
	validVarRegex      = regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$|^[a-z]$`)
	validFilenameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$|^[a-z]$`)
)

func isValidID(id string) bool {
	return validIDRegex.MatchString(id)
}

func isValidVariableName(name string) bool {
	return validVarRegex.MatchString(name)
}

func isValidScenarioFilename(name string) bool {
	// Allow 'x.' prefix for experimental scenarios
	name = strings.TrimPrefix(name, "x.")
	return validFilenameRegex.MatchString(name)
}
