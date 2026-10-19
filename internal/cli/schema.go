package cli

// CanonicalSchema returns the Draft 2020-12 JSON Schema for Arkon manifests and state files.
func CanonicalSchema() map[string]any {
	return map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://github.com/adrianfinantyo/arkon/schema.json",
		"title":       "ArkonSchema",
		"description": "Canonical JSON Schema for Arkon manifests and runtime state files",
		"oneOf": []any{
			map[string]any{"$ref": "#/$defs/manifest"},
			map[string]any{"$ref": "#/$defs/state"},
		},
		"$defs": map[string]any{
			"manifest": map[string]any{
				"title":    "ArkonManifest",
				"type":     "object",
				"required": []any{"version", "host", "circadian", "teardown"},
				"properties": map[string]any{
					"version": map[string]any{
						"type": "string",
						"enum": []any{"1"},
					},
					"host": map[string]any{
						"type":     "object",
						"required": []any{"timezone"},
						"properties": map[string]any{
							"timezone": map[string]any{
								"type": "string",
							},
							"rtc_device": map[string]any{
								"type":    "string",
								"default": "/sys/class/rtc/rtc0",
							},
							"power_action": map[string]any{
								"type":    "string",
								"enum":    []any{"poweroff", "suspend", "hibernate"},
								"default": "poweroff",
							},
						},
					},
					"circadian": map[string]any{
						"type":     "object",
						"required": []any{"sleep_at", "wake_at"},
						"properties": map[string]any{
							"sleep_at": map[string]any{
								"type":    "string",
								"pattern": "^[0-2][0-9]:[0-5][0-9]$",
							},
							"wake_at": map[string]any{
								"type":    "string",
								"pattern": "^[0-2][0-9]:[0-5][0-9]$",
							},
							"days": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
							},
						},
					},
					"teardown": map[string]any{
						"type":     "object",
						"required": []any{"profile_order"},
						"properties": map[string]any{
							"provider": map[string]any{
								"type":    "string",
								"enum":    []any{"docker-compose", "podman-compose", "command", "systemd"},
								"default": "docker-compose",
							},
							"timeout_seconds": map[string]any{
								"type":    "integer",
								"default": 30,
							},
							"compose_stacks": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type":     "object",
									"required": []any{"path"},
									"properties": map[string]any{
										"path": map[string]any{"type": "string"},
									},
								},
							},
							"profile_order": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
							},
						},
					},
					"hooks": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"pre_teardown": map[string]any{
								"type":  "array",
								"items": map[string]any{"$ref": "#/$defs/hook"},
							},
							"post_teardown": map[string]any{
								"type":  "array",
								"items": map[string]any{"$ref": "#/$defs/hook"},
							},
							"pre_sleep": map[string]any{
								"type":  "array",
								"items": map[string]any{"$ref": "#/$defs/hook"},
							},
							"post_wake": map[string]any{
								"type":  "array",
								"items": map[string]any{"$ref": "#/$defs/hook"},
							},
						},
					},
					"inhibition": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"default_duration": map[string]any{
								"type":    "string",
								"default": "2h",
							},
							"daytime_protection": map[string]any{
								"type":    "boolean",
								"default": true,
							},
						},
					},
				},
			},
			"hook": map[string]any{
				"type":     "object",
				"required": []any{"name", "command"},
				"properties": map[string]any{
					"name":            map[string]any{"type": "string"},
					"command":         map[string]any{"type": "string"},
					"timeout_seconds": map[string]any{"type": "integer", "default": 10},
					"ignore_errors":   map[string]any{"type": "boolean", "default": false},
				},
			},
			"state": map[string]any{
				"title":    "ArkonState",
				"type":     "object",
				"required": []any{"locks"},
				"properties": map[string]any{
					"locks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"$ref": "#/$defs/inhibition_lock",
						},
					},
				},
			},
			"inhibition_lock": map[string]any{
				"type":     "object",
				"required": []any{"id", "reason", "created_by", "expires_at"},
				"properties": map[string]any{
					"id":         map[string]any{"type": "string"},
					"reason":     map[string]any{"type": "string"},
					"created_by": map[string]any{"type": "string"},
					"expires_at": map[string]any{"type": "string", "format": "date-time"},
					"remaining":  map[string]any{"type": "string"},
				},
			},
		},
	}
}
