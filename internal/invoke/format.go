package invoke

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatAssistantText extracts human-readable content from Claude Code's
// streaming JSON output. Each line of raw model stdout may be a JSON
// envelope (assistant, result, system) or plain text; this function
// normalises both into something a human would want to read.
func FormatAssistantText(raw string) string {
	var envelope struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Message struct {
			Content []struct {
				Type  string `json:"type"`
				Text  string `json:"text"`
				Name  string `json:"name"`
				Input any    `json:"input"`
			} `json:"content"`
		} `json:"message"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		if len(raw) > 200 {
			return raw[:200] + "..."
		}
		return raw
	}

	switch envelope.Type {
	case "assistant":
		var parts []string
		for _, c := range envelope.Message.Content {
			switch c.Type {
			case "text":
				if c.Text != "" {
					parts = append(parts, c.Text)
				}
			case "tool_use":
				if c.Name != "" {
					if inputMap, ok := c.Input.(map[string]any); ok {
						if desc, ok := inputMap["description"].(string); ok {
							parts = append(parts, fmt.Sprintf("  → %s: %s", c.Name, desc))
						} else if cmd, ok := inputMap["command"].(string); ok {
							if len(cmd) > 80 {
								cmd = cmd[:80] + "..."
							}
							parts = append(parts, fmt.Sprintf("  → %s: %s", c.Name, cmd))
						} else {
							parts = append(parts, fmt.Sprintf("  → %s", c.Name))
						}
					} else {
						parts = append(parts, fmt.Sprintf("  → %s", c.Name))
					}
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	case "result":
		if envelope.Result != "" {
			return "[result] " + envelope.Result
		}
	case "system":
		if envelope.Subtype == "init" {
			return "[session started]"
		}
	}

	return ""
}
