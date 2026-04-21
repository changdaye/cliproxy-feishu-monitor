package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldSendByInterval(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	if !shouldSendByInterval("", 3*time.Hour, now) {
		t.Fatal("empty timestamp should trigger send")
	}
	if shouldSendByInterval("2026-04-21T10:00:01Z", 2*time.Hour, now) {
		t.Fatal("interval not reached should not trigger send")
	}
	if !shouldSendByInterval("2026-04-21T10:00:00Z", 2*time.Hour, now) {
		t.Fatal("exact interval should trigger send")
	}
}

func TestBuildHeartbeatText(t *testing.T) {
	state := runtimeState{
		LastSummaryAt:       "2026-04-21T06:00:00Z",
		LastSuccessAt:       "2026-04-21T06:00:00Z",
		ConsecutiveFailures: 1,
		LastSummary: &summary{
			Accounts: 166,
			StatusCounts: map[string]int{
				"full":      40,
				"high":      77,
				"medium":    26,
				"low":       11,
				"exhausted": 7,
			},
			TokenUsage: tokenUsageSummary{Last24Hours: 97312161},
		},
	}
	text := buildHeartbeatText(state, 3*time.Hour)
	checks := []string{"健康心跳", "上次汇总", "账号总数 166", "充足 40", "高 77", "24小时 Token 用量: 97,312,161", "连续失败: 1", "心跳间隔: 3h0m0s"}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("heartbeat missing %q\n%s", want, text)
		}
	}
}

func TestLoadRuntimeConfigFromJSON(t *testing.T) {
	t.Setenv("CPA_BASE_URL", "")
	t.Setenv("CPA_MANAGEMENT_KEY", "")
	t.Setenv("FEISHU_WEBHOOK", "")
	t.Setenv("FEISHU_SECRET", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "local.runtime.json")
	raw := `{
		"cpa_base_url": "http://127.0.0.1:8317/management.html#/login",
		"management_key": "secret-key",
		"feishu_webhook": "https://open.feishu.cn/open-apis/bot/v2/hook/test",
		"feishu_secret": "sig",
		"poll_interval_hours": 6,
		"heartbeat_interval_hours": 3,
		"startup_notification_enabled": true,
		"heartbeat_enabled": true,
		"request_timeout_seconds": 21,
		"concurrency": 4,
		"state_path": "data/runtime-state.json"
	}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir, "")
	if err != nil {
		t.Fatalf("loadRuntimeConfig error: %v", err)
	}
	if cfg.ManagementKey != "secret-key" || cfg.FeishuSecret != "sig" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.PollInterval != 6*time.Hour || cfg.HeartbeatInterval != 3*time.Hour {
		t.Fatalf("unexpected intervals: poll=%v heartbeat=%v", cfg.PollInterval, cfg.HeartbeatInterval)
	}
	if cfg.Timeout != 21*time.Second || cfg.Concurrency != 4 {
		t.Fatalf("unexpected runtime knobs: timeout=%v concurrency=%d", cfg.Timeout, cfg.Concurrency)
	}
	if !strings.HasSuffix(cfg.StatePath, filepath.Join("data", "runtime-state.json")) {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
}
