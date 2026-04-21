package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func firstValue(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
		}
		return value
	}
	return nil
}

func nested(value any, keys ...string) any {
	current := value
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

func cleanString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func numberFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}

func intFromAny(value any) int {
	return int(math.Round(numberFromAny(value)))
}

func numberPtr(value any) *float64 {
	if value == nil {
		return nil
	}
	v := numberFromAny(value)
	return &v
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return false
}

func isFalse(value any) bool {
	switch v := value.(type) {
	case bool:
		return !v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "0", "false", "no", "n", "off":
			return true
		}
	case float64:
		return v == 0
	case int:
		return v == 0
	}
	return false
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func decodeBase64URL(v string) ([]byte, error) {
	switch len(v) % 4 {
	case 2:
		v += "=="
	case 3:
		v += "="
	}
	return base64.URLEncoding.DecodeString(v)
}

func normalizePlan(value any) string {
	s := strings.ToLower(cleanString(value))
	switch s {
	case "free", "plus", "team", "pro", "enterprise", "codex":
		return s
	default:
		return s
	}
}

func parseJWTLike(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case string:
		raw := strings.TrimSpace(v)
		if raw == "" {
			return nil
		}
		var out map[string]any
		if json.Unmarshal([]byte(raw), &out) == nil {
			return out
		}
		parts := strings.Split(raw, ".")
		if len(parts) < 2 {
			return nil
		}
		payload, err := decodeBase64URL(parts[1])
		if err != nil {
			return nil
		}
		if json.Unmarshal(payload, &out) != nil {
			return nil
		}
		return out
	default:
		return nil
	}
}

func parseAccountID(entry map[string]any) string {
	for _, candidate := range []any{nested(entry, "id_token", "chatgpt_account_id"), nested(entry, "id_token", "https://api.openai.com/auth"), entry["id_token"], nested(entry, "metadata", "id_token")} {
		switch v := candidate.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case map[string]any:
			if id := cleanString(v["chatgpt_account_id"]); id != "" {
				return id
			}
		}
		if payload := parseJWTLike(candidate); payload != nil {
			if id := cleanString(payload["chatgpt_account_id"]); id != "" {
				return id
			}
			if authInfo, ok := payload["https://api.openai.com/auth"].(map[string]any); ok {
				if id := cleanString(authInfo["chatgpt_account_id"]); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

func parsePlanType(entry map[string]any) string {
	for _, candidate := range []any{nested(entry, "id_token", "plan_type"), entry["plan_type"], entry["planType"], nested(entry, "metadata", "plan_type")} {
		if plan := normalizePlan(candidate); plan != "" {
			return plan
		}
	}
	return ""
}

func isAuthDisabled(entry map[string]any) bool {
	if boolFromAny(entry["disabled"]) {
		return true
	}
	return strings.EqualFold(cleanString(entry["status"]), "disabled")
}

func authIdentifier(entry map[string]any) string {
	return cleanString(firstValue(entry["name"], entry["id"], entry["label"]))
}

func findWindow(windows []quotaWindow, id string) *quotaWindow {
	for i := range windows {
		if windows[i].ID == id {
			return &windows[i]
		}
	}
	return nil
}

func tokenTotalFromDetail(detail map[string]any) int64 {
	tokens, _ := detail["tokens"].(map[string]any)
	if tokens == nil {
		return 0
	}
	if raw := firstValue(tokens["total_tokens"], tokens["totalTokens"]); raw != nil {
		return int64(numberFromAny(raw))
	}
	var total int64
	for _, raw := range []any{firstValue(tokens["input_tokens"], tokens["inputTokens"]), firstValue(tokens["output_tokens"], tokens["outputTokens"]), firstValue(tokens["reasoning_tokens"], tokens["reasoningTokens"])} {
		if raw == nil {
			continue
		}
		total += int64(numberFromAny(raw))
	}
	return total
}

func formatTokenUsageHistoryTimestamp(value time.Time, loc *time.Location) string {
	if value.IsZero() {
		return ""
	}
	if loc == nil {
		loc = time.Local
	}
	return value.In(loc).Format(time.RFC3339Nano)
}

func formatNumberWithCommas(v int64) string {
	raw := strconv.FormatInt(v, 10)
	if len(raw) <= 3 {
		return raw
	}
	neg := ""
	if raw[0] == '-' {
		neg = "-"
		raw = raw[1:]
	}
	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return neg + strings.Join(parts, ",")
}

func formatTokenUsageMillions(v int64) string {
	return fmt.Sprintf("%.2f 百万", float64(v)/1_000_000)
}

func configNowFunc(cfg config) func() time.Time {
	if cfg.Now != nil {
		return cfg.Now
	}
	return time.Now
}

func floatPtr(v float64) *float64 { return &v }
