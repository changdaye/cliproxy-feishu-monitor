package main

import "time"

const (
	defaultCPABaseURL             = "http://127.0.0.1:8317"
	defaultTimeoutSeconds         = 30
	defaultConcurrency            = 8
	defaultPollIntervalHours      = 6
	defaultHeartbeatIntervalHours = 3
	defaultFailureAlertThreshold  = 3
	window5HSeconds               = 5 * 60 * 60
	window7DSeconds               = 7 * 24 * 60 * 60
	whamUsageURL                  = "https://chatgpt.com/backend-api/wham/usage"
)

var whamHeaders = map[string]string{
	"Authorization": "Bearer $TOKEN$",
	"Content-Type":  "application/json",
	"User-Agent":    "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal",
}

type config struct {
	BaseURL                    string
	ManagementKey              string
	FeishuWebhook              string
	FeishuSecret               string
	Timeout                    time.Duration
	Concurrency                int
	PollInterval               time.Duration
	HeartbeatInterval          time.Duration
	HeartbeatEnabled           bool
	StartupNotificationEnabled bool
	RunSummaryOnStartup        bool
	FailureAlertThreshold      int
	StatePath                  string
	ConfigPath                 string
	DryRun                     bool
	PrintJSON                  bool
	Now                        func() time.Time
}

type runtimeFileConfig struct {
	BaseURL                    string `json:"cpa_base_url"`
	ManagementKey              string `json:"management_key"`
	FeishuWebhook              string `json:"feishu_webhook"`
	FeishuSecret               string `json:"feishu_secret"`
	PollIntervalHours          int    `json:"poll_interval_hours"`
	HeartbeatIntervalHours     int    `json:"heartbeat_interval_hours"`
	HeartbeatEnabled           *bool  `json:"heartbeat_enabled"`
	StartupNotificationEnabled *bool  `json:"startup_notification_enabled"`
	RunSummaryOnStartup        *bool  `json:"run_summary_on_startup"`
	RequestTimeoutSeconds      int    `json:"request_timeout_seconds"`
	Concurrency                int    `json:"concurrency"`
	FailureAlertThreshold      int    `json:"failure_alert_threshold"`
	StatePath                  string `json:"state_path"`
}

type runtimeState struct {
	LastSummaryAt       string   `json:"last_summary_at,omitempty"`
	LastHeartbeatAt     string   `json:"last_heartbeat_at,omitempty"`
	LastSuccessAt       string   `json:"last_success_at,omitempty"`
	LastErrorAt         string   `json:"last_error_at,omitempty"`
	LastError           string   `json:"last_error,omitempty"`
	ConsecutiveFailures int      `json:"consecutive_failures,omitempty"`
	LastSummary         *summary `json:"last_summary,omitempty"`
}

type authEntry struct {
	Raw map[string]any
}

type quotaWindow struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	UsedPercent      *float64 `json:"used_percent,omitempty"`
	RemainingPercent *float64 `json:"remaining_percent,omitempty"`
	ResetLabel       string   `json:"reset_label,omitempty"`
	Exhausted        bool     `json:"exhausted"`
}

type quotaReport struct {
	Name              string        `json:"name"`
	AuthIndex         string        `json:"auth_index,omitempty"`
	AccountID         string        `json:"account_id,omitempty"`
	PlanType          string        `json:"plan_type,omitempty"`
	Disabled          bool          `json:"disabled"`
	Status            string        `json:"status"`
	Windows           []quotaWindow `json:"windows,omitempty"`
	AdditionalWindows []quotaWindow `json:"additional_windows,omitempty"`
	Error             string        `json:"error,omitempty"`
	tokenUsage        tokenUsageSummary
}

type tokenUsageSummary struct {
	Available       bool   `json:"available"`
	AllTime         int64  `json:"all_time"`
	Last7Hours      int64  `json:"last_7_hours"`
	Last24Hours     int64  `json:"last_24_hours"`
	Last7Days       int64  `json:"last_7_days"`
	HistoryStart    string `json:"history_start,omitempty"`
	HistoryEnd      string `json:"history_end,omitempty"`
	Complete7Hours  bool   `json:"complete_7_hours"`
	Complete24Hours bool   `json:"complete_24_hours"`
	Complete7Days   bool   `json:"complete_7_days"`
}

type summary struct {
	Accounts         int               `json:"accounts"`
	StatusCounts     map[string]int    `json:"status_counts"`
	PlanCounts       map[string]int    `json:"plan_counts"`
	FreeEquivalent7D float64           `json:"free_equivalent_7d"`
	PlusEquivalent7D float64           `json:"plus_equivalent_7d"`
	TokenUsage       tokenUsageSummary `json:"token_usage"`
}

type tokenUsageResult struct {
	ByAuth          map[string]tokenUsageSummary
	HistoryStart    time.Time
	HistoryEnd      time.Time
	Complete7Hours  bool
	Complete24Hours bool
	Complete7Days   bool
}
