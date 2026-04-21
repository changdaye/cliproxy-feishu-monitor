package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseTokenUsageByAuth(t *testing.T) {
	now := time.Date(2026, 4, 21, 11, 0, 0, 0, time.UTC)
	payload := map[string]any{
		"usage": map[string]any{
			"apis": map[string]any{
				"key": map[string]any{
					"models": map[string]any{
						"gpt-5.4": map[string]any{
							"details": []any{
								map[string]any{
									"auth_index": "a1",
									"timestamp":  "2026-04-21T10:00:00Z",
									"tokens": map[string]any{
										"total_tokens": 100,
									},
								},
								map[string]any{
									"auth_index": "a1",
									"timestamp":  "2026-04-20T12:00:00Z",
									"tokens": map[string]any{
										"input_tokens":  30,
										"output_tokens": 20,
									},
								},
								map[string]any{
									"auth_index": "a2",
									"timestamp":  "2026-04-12T12:00:00Z",
									"tokens": map[string]any{
										"total_tokens": 999,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := parseTokenUsageByAuth(payload, now)
	if result.ByAuth["a1"].Last7Hours != 100 {
		t.Fatalf("Last7Hours = %d, want 100", result.ByAuth["a1"].Last7Hours)
	}
	if result.ByAuth["a1"].Last24Hours != 150 {
		t.Fatalf("Last24Hours = %d, want 150", result.ByAuth["a1"].Last24Hours)
	}
	if result.ByAuth["a1"].Last7Days != 150 {
		t.Fatalf("Last7Days = %d, want 150", result.ByAuth["a1"].Last7Days)
	}
	if result.ByAuth["a1"].AllTime != 150 {
		t.Fatalf("AllTime = %d, want 150", result.ByAuth["a1"].AllTime)
	}
	if result.ByAuth["a2"].AllTime != 999 {
		t.Fatalf("a2 AllTime = %d, want 999", result.ByAuth["a2"].AllTime)
	}
	if !result.Complete7Days {
		t.Fatalf("Complete7Days = false, want true")
	}
}

func TestSummarizeReports(t *testing.T) {
	reports := []quotaReport{
		{
			Name:       "full@example.com",
			PlanType:   "free",
			AuthIndex:  "a1",
			AccountID:  "acc1",
			Status:     "full",
			Windows:    []quotaWindow{{ID: "code-7d", RemainingPercent: floatPtr(100)}},
			tokenUsage: tokenUsageSummary{Available: true, Last7Hours: 10, Last24Hours: 20, Last7Days: 30, AllTime: 40},
		},
		{
			Name:       "med@example.com",
			PlanType:   "free",
			AuthIndex:  "a2",
			AccountID:  "acc2",
			Status:     "medium",
			Windows:    []quotaWindow{{ID: "code-7d", RemainingPercent: floatPtr(50)}},
			tokenUsage: tokenUsageSummary{Available: true, Last7Hours: 1, Last24Hours: 2, Last7Days: 3, AllTime: 4},
		},
		{
			Name:     "disabled@example.com",
			PlanType: "free",
			Disabled: true,
			Status:   "disabled",
		},
		{
			Name:      "exhausted@example.com",
			PlanType:  "free",
			AuthIndex: "a4",
			AccountID: "acc4",
			Status:    "exhausted",
			Windows:   []quotaWindow{{ID: "code-7d", RemainingPercent: floatPtr(0)}},
		},
	}

	sum := summarize(reports)
	if sum.Accounts != 4 {
		t.Fatalf("Accounts = %d, want 4", sum.Accounts)
	}
	if sum.StatusCounts["full"] != 1 || sum.StatusCounts["medium"] != 1 || sum.StatusCounts["disabled"] != 1 || sum.StatusCounts["exhausted"] != 1 {
		t.Fatalf("unexpected StatusCounts: %+v", sum.StatusCounts)
	}
	if sum.FreeEquivalent7D != 150 {
		t.Fatalf("FreeEquivalent7D = %.0f, want 150", sum.FreeEquivalent7D)
	}
	if sum.TokenUsage.Last7Hours != 11 || sum.TokenUsage.Last24Hours != 22 || sum.TokenUsage.Last7Days != 33 || sum.TokenUsage.AllTime != 44 {
		t.Fatalf("unexpected token usage summary: %+v", sum.TokenUsage)
	}
}

func TestBuildFeishuTextMessage(t *testing.T) {
	sum := summary{
		Accounts: 166,
		StatusCounts: map[string]int{
			"full":      40,
			"high":      77,
			"medium":    26,
			"low":       11,
			"exhausted": 7,
			"disabled":  5,
		},
		FreeEquivalent7D: 12283,
		TokenUsage:       tokenUsageSummary{Available: true, Last7Hours: 40845322, Last24Hours: 97312161, Last7Days: 226996537, AllTime: 297376388},
	}

	text := buildSummaryTextMessage(sum, "http://example-cpa-host:8317", time.Date(2026, 4, 21, 19, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	checks := []string{"状态概览", "账号总数 166", "充足 40", "高 77", "中 26", "低 11", "耗尽 7", "7日免费等效: 12283%", "7小时 Token 用量: 40,845,322", "累计 Token 用量: 297,376,388"}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("message missing %q\n%s", want, text)
		}
	}
}

func TestBuildFeishuSignedPayload(t *testing.T) {
	payload, err := buildFeishuSignedPayload("hello", "vHbqkjsOlWLPXlPeZV7Lif", func() time.Time {
		return time.Unix(1713700000, 0)
	})
	if err != nil {
		t.Fatalf("buildFeishuSignedPayload error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if body["timestamp"] != "1713700000" {
		t.Fatalf("timestamp = %v, want 1713700000", body["timestamp"])
	}
	if body["sign"] == "" {
		t.Fatal("sign is empty")
	}
	if body["msg_type"] != "text" {
		t.Fatalf("msg_type = %v, want text", body["msg_type"])
	}
}
