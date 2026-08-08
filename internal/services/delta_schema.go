package services

// deltaUpdateSchema is the shared apply_changes JSON Schema used by every vendor
// wrapper (Anthropic tool input_schema and Venice response_format.json_schema).
// An edit here changes both.
func deltaUpdateSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"user_location": map[string]any{
				"type": "string",
			},
			"scene_change": map[string]any{
				"anyOf": []any{
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"to":     map[string]any{"type": "string"},
							"reason": map[string]any{"type": "string"},
						},
						"required": []string{"to", "reason"},
					},
					map[string]any{"type": "null"},
				},
			},
			"item_events": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"item": map[string]any{
							"type": "string",
						},
						"action": map[string]any{
							"type": "string",
							"enum": []string{"acquire", "give", "drop", "move", "use"},
						},
						"from": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"type": map[string]any{
									"type": "string",
									"enum": []string{"player", "npc", "location"},
								},
								"name": map[string]any{
									"type": "string",
								},
							},
							"required": []string{"type"},
						},
						"to": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"type": map[string]any{
									"type": "string",
									"enum": []string{"player", "npc", "location"},
								},
								"name": map[string]any{
									"type": "string",
								},
							},
							"required": []string{"type"},
						},
						"consumed": map[string]any{
							"type": "boolean",
						},
					},
					"required": []string{"item", "action"},
				},
			},
			"npc_events": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"npc_id": map[string]any{
							"type": "string",
						},
						"set_location": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"npc_id"},
				},
			},
			"set_vars": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
			"game_ended": map[string]any{
				"type": "boolean",
			},
		},
		"required": []string{"user_location", "scene_change", "item_events", "npc_events", "set_vars", "game_ended"},
	}
}
