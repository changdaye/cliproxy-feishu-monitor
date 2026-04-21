package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func parseFlags(args []string, workdir string) (string, config, error) {
	mode, remaining := extractMode(args)
	explicitConfigPath := scanFlagValue(remaining, "config")
	baseCfg, err := loadRuntimeConfig(workdir, explicitConfigPath)
	if err != nil {
		return "", config{}, err
	}

	fs := flag.NewFlagSet("cliproxy-feishu-monitor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := baseCfg
	baseURL := cfg.BaseURL
	managementKey := cfg.ManagementKey
	feishuWebhook := cfg.FeishuWebhook
	feishuSecret := cfg.FeishuSecret
	pollHours := int(maxDuration(cfg.PollInterval, time.Hour) / time.Hour)
	heartbeatHours := int(maxDuration(cfg.HeartbeatInterval, time.Hour) / time.Hour)
	timeoutSeconds := int(maxDuration(cfg.Timeout, time.Second) / time.Second)
	concurrency := cfg.Concurrency
	failureAlertThreshold := cfg.FailureAlertThreshold
	statePath := cfg.StatePath
	configPath := cfg.ConfigPath
	startupEnabled := cfg.StartupNotificationEnabled
	heartbeatEnabled := cfg.HeartbeatEnabled
	runSummaryOnStartup := cfg.RunSummaryOnStartup

	fs.StringVar(&baseURL, "cpa-base-url", baseURL, "CPA base URL or management page URL")
	fs.StringVar(&managementKey, "management-key", managementKey, "CPA management key")
	fs.StringVar(&feishuWebhook, "feishu-webhook", feishuWebhook, "Feishu bot webhook URL")
	fs.StringVar(&feishuSecret, "feishu-secret", feishuSecret, "Feishu bot signature secret")
	fs.IntVar(&pollHours, "poll-interval-hours", pollHours, "Summary push interval in hours")
	fs.IntVar(&heartbeatHours, "heartbeat-interval-hours", heartbeatHours, "Heartbeat push interval in hours")
	fs.BoolVar(&heartbeatEnabled, "heartbeat-enabled", heartbeatEnabled, "Enable heartbeat push")
	fs.BoolVar(&startupEnabled, "startup-notification-enabled", startupEnabled, "Enable startup notification")
	fs.BoolVar(&runSummaryOnStartup, "run-summary-on-startup", runSummaryOnStartup, "Run one summary immediately when service starts")
	fs.IntVar(&timeoutSeconds, "timeout", timeoutSeconds, "HTTP timeout in seconds")
	fs.IntVar(&concurrency, "concurrency", concurrency, "Concurrent account quota checks")
	fs.IntVar(&failureAlertThreshold, "failure-alert-threshold", failureAlertThreshold, "Consecutive failures before sending alert")
	fs.StringVar(&statePath, "state-path", statePath, "Runtime state file path")
	fs.StringVar(&configPath, "config", configPath, "Path to runtime JSON config")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Print message only, do not push to Feishu")
	fs.BoolVar(&cfg.PrintJSON, "json", false, "Also print JSON payload on run-once")

	if err := fs.Parse(remaining); err != nil {
		return "", config{}, err
	}

	cfg.BaseURL = normalizeBaseURL(baseURL)
	cfg.ManagementKey = strings.TrimSpace(managementKey)
	cfg.FeishuWebhook = strings.TrimSpace(feishuWebhook)
	cfg.FeishuSecret = strings.TrimSpace(feishuSecret)
	cfg.Timeout = time.Duration(maxInt(timeoutSeconds, 1)) * time.Second
	cfg.Concurrency = maxInt(concurrency, 1)
	cfg.PollInterval = time.Duration(maxInt(pollHours, 1)) * time.Hour
	cfg.HeartbeatInterval = time.Duration(maxInt(heartbeatHours, 1)) * time.Hour
	cfg.HeartbeatEnabled = heartbeatEnabled
	cfg.StartupNotificationEnabled = startupEnabled
	cfg.RunSummaryOnStartup = runSummaryOnStartup
	cfg.FailureAlertThreshold = maxInt(failureAlertThreshold, 1)
	cfg.StatePath = resolveStatePath(workdir, statePath)
	cfg.ConfigPath = resolveConfigPath(workdir, configPath)
	cfg.Now = time.Now

	if cfg.BaseURL == "" {
		return "", config{}, fmt.Errorf("missing CPA base URL")
	}
	if cfg.ManagementKey == "" {
		return "", config{}, fmt.Errorf("missing management key")
	}
	return mode, cfg, nil
}

func loadRuntimeConfig(workdir, explicitPath string) (config, error) {
	cfg := config{
		BaseURL:                    defaultCPABaseURL,
		Timeout:                    defaultTimeoutSeconds * time.Second,
		Concurrency:                defaultConcurrency,
		PollInterval:               defaultPollIntervalHours * time.Hour,
		HeartbeatInterval:          defaultHeartbeatIntervalHours * time.Hour,
		HeartbeatEnabled:           true,
		StartupNotificationEnabled: true,
		RunSummaryOnStartup:        true,
		FailureAlertThreshold:      defaultFailureAlertThreshold,
		StatePath:                  resolveStatePath(workdir, ""),
		ConfigPath:                 resolveConfigPath(workdir, explicitPath),
		Now:                        time.Now,
	}

	configPath := locateConfigFile(workdir, explicitPath)
	if configPath != "" {
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return config{}, fmt.Errorf("read config %s: %w", configPath, err)
		}
		var fileCfg runtimeFileConfig
		if err := json.Unmarshal(raw, &fileCfg); err != nil {
			return config{}, fmt.Errorf("parse config %s: %w", configPath, err)
		}
		cfg.ConfigPath = configPath
		if fileCfg.BaseURL != "" {
			cfg.BaseURL = fileCfg.BaseURL
		}
		if fileCfg.ManagementKey != "" {
			cfg.ManagementKey = fileCfg.ManagementKey
		}
		if fileCfg.FeishuWebhook != "" {
			cfg.FeishuWebhook = fileCfg.FeishuWebhook
		}
		if fileCfg.FeishuSecret != "" {
			cfg.FeishuSecret = fileCfg.FeishuSecret
		}
		if fileCfg.PollIntervalHours > 0 {
			cfg.PollInterval = time.Duration(fileCfg.PollIntervalHours) * time.Hour
		}
		if fileCfg.HeartbeatIntervalHours > 0 {
			cfg.HeartbeatInterval = time.Duration(fileCfg.HeartbeatIntervalHours) * time.Hour
		}
		if fileCfg.RequestTimeoutSeconds > 0 {
			cfg.Timeout = time.Duration(fileCfg.RequestTimeoutSeconds) * time.Second
		}
		if fileCfg.Concurrency > 0 {
			cfg.Concurrency = fileCfg.Concurrency
		}
		if fileCfg.FailureAlertThreshold > 0 {
			cfg.FailureAlertThreshold = fileCfg.FailureAlertThreshold
		}
		if fileCfg.HeartbeatEnabled != nil {
			cfg.HeartbeatEnabled = *fileCfg.HeartbeatEnabled
		}
		if fileCfg.StartupNotificationEnabled != nil {
			cfg.StartupNotificationEnabled = *fileCfg.StartupNotificationEnabled
		}
		if fileCfg.RunSummaryOnStartup != nil {
			cfg.RunSummaryOnStartup = *fileCfg.RunSummaryOnStartup
		}
		cfg.StatePath = resolveStatePath(workdir, fileCfg.StatePath)
		return cfg, nil
	}

	cfg.BaseURL = firstNonEmpty(envFirst("CPA_BASE_URL", "CPA_URL", "CPA_LOGIN_URL"), cfg.BaseURL)
	cfg.ManagementKey = envFirst("CPA_MANAGEMENT_KEY", "CPA_MANAGEMENT_PASSWORD", "MANAGEMENT_PASSWORD")
	cfg.FeishuWebhook = envFirst("FEISHU_WEBHOOK")
	cfg.FeishuSecret = envFirst("FEISHU_SECRET")
	return cfg, nil
}

func extractMode(args []string) (string, []string) {
	if len(args) == 0 {
		return "run-once", args
	}
	first := strings.TrimSpace(args[0])
	if first == "run-once" || first == "serve" {
		return first, args[1:]
	}
	return "run-once", args
}

func scanFlagValue(args []string, name string) string {
	long := "--" + name
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == long && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, long+"=") {
			return strings.TrimPrefix(arg, long+"=")
		}
	}
	return ""
}

func locateConfigFile(workdir, explicitPath string) string {
	if strings.TrimSpace(explicitPath) != "" {
		path := resolveConfigPath(workdir, explicitPath)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		return path
	}
	for _, name := range []string{"local.runtime.json", "runtime.local.json", "config.local.json"} {
		candidate := filepath.Join(workdir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func resolveConfigPath(workdir, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filepath.Join(workdir, "local.runtime.json")
	}
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(workdir, value)
}

func resolveStatePath(workdir, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filepath.Join(workdir, "data", "runtime-state.json")
	}
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(workdir, value)
}

func normalizeBaseURL(v string) string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		raw = strings.TrimSuffix(raw, "/")
		raw = strings.TrimSuffix(raw, "/management.html#/login")
		raw = strings.TrimSuffix(raw, "/management.html")
		raw = strings.TrimSuffix(raw, "/login")
		raw = strings.TrimSuffix(raw, "/v0/management")
		return strings.TrimSuffix(raw, "/")
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	for _, suffix := range []string{"/v0/management/auth-files", "/v0/management/api-call", "/v0/management", "/management.html", "/login"} {
		path = strings.TrimSuffix(path, suffix)
	}
	if path == "/" {
		path = ""
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/")
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func ensureParentDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("empty path")
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
