package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

func fetchMonitorData(ctx context.Context, cfg config) (summary, []quotaReport, error) {
	auths, err := loadCodexAuths(ctx, cfg)
	if err != nil {
		return summary{}, nil, err
	}
	reports, err := queryAllQuotas(ctx, cfg, auths)
	if err != nil {
		return summary{}, nil, err
	}
	if usageResult, err := fetchTokenUsageByAuth(ctx, cfg, configNow(cfg)); err == nil {
		for i := range reports {
			reports[i].tokenUsage = tokenUsageSummary{
				Available:       true,
				HistoryStart:    formatTokenUsageHistoryTimestamp(usageResult.HistoryStart, time.Local),
				HistoryEnd:      formatTokenUsageHistoryTimestamp(usageResult.HistoryEnd, time.Local),
				Complete7Hours:  usageResult.Complete7Hours,
				Complete24Hours: usageResult.Complete24Hours,
				Complete7Days:   usageResult.Complete7Days,
			}
			if usage, ok := usageResult.ByAuth[reports[i].AuthIndex]; ok {
				usage.Available = true
				usage.HistoryStart = reports[i].tokenUsage.HistoryStart
				usage.HistoryEnd = reports[i].tokenUsage.HistoryEnd
				usage.Complete7Hours = reports[i].tokenUsage.Complete7Hours
				usage.Complete24Hours = reports[i].tokenUsage.Complete24Hours
				usage.Complete7Days = reports[i].tokenUsage.Complete7Days
				reports[i].tokenUsage = usage
			}
		}
	}
	sortReportsDefault(reports)
	return summarize(reports), reports, nil
}

func loadCodexAuths(ctx context.Context, cfg config) ([]authEntry, error) {
	client := &http.Client{Timeout: cfg.Timeout}
	payload, err := fetchJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/auth-files")
	if err != nil {
		return nil, err
	}
	files, ok := payload["files"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected auth-files payload from CPA management API")
	}
	out := make([]authEntry, 0, len(files))
	for _, item := range files {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if normalizePlan(firstValue(entry["provider"], entry["type"])) != "codex" {
			continue
		}
		out = append(out, authEntry{Raw: entry})
	}
	return out, nil
}

func queryAllQuotas(ctx context.Context, cfg config, auths []authEntry) ([]quotaReport, error) {
	if len(auths) == 0 {
		return []quotaReport{}, nil
	}
	client := &http.Client{Timeout: cfg.Timeout}
	reports := make([]quotaReport, len(auths))
	errCh := make(chan error, len(auths))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	for i := range auths {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			report, err := querySingleQuota(ctx, client, cfg, auths[i])
			if err != nil {
				errCh <- err
				return
			}
			reports[i] = report
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(reports, func(i, j int) bool { return strings.ToLower(reports[i].Name) < strings.ToLower(reports[j].Name) })
	return reports, nil
}

func querySingleQuota(ctx context.Context, client *http.Client, cfg config, entry authEntry) (quotaReport, error) {
	report := quotaReport{
		Name:      authIdentifier(entry.Raw),
		AuthIndex: cleanString(firstValue(entry.Raw["auth_index"], entry.Raw["authIndex"])),
		AccountID: parseAccountID(entry.Raw),
		PlanType:  parsePlanType(entry.Raw),
		Disabled:  isAuthDisabled(entry.Raw),
		Status:    "unknown",
	}
	if report.Name == "" {
		report.Name = "unknown"
	}
	if report.Disabled {
		report.Status = deriveStatus(report)
		return report, nil
	}
	if report.AuthIndex == "" || report.AccountID == "" {
		report.Error = "missing auth_index or chatgpt_account_id"
		report.Status = deriveStatus(report)
		return report, nil
	}
	payload := map[string]any{
		"auth_index": report.AuthIndex,
		"method":     http.MethodGet,
		"url":        whamUsageURL,
		"header":     mergeMaps(whamHeaders, map[string]string{"Chatgpt-Account-Id": report.AccountID}),
	}
	response, err := postJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/api-call", payload)
	if err != nil {
		report.Error = err.Error()
		report.Status = deriveStatus(report)
		return report, nil
	}
	statusCode := intFromAny(firstValue(response["status_code"], response["statusCode"]))
	body := response["body"]
	parsedBody, parseErr := parseBody(body)
	if statusCode < 200 || statusCode >= 300 {
		report.Error = bodyString(body)
		if report.Error == "" {
			report.Error = fmt.Sprintf("HTTP %d", statusCode)
		}
		report.Status = deriveStatus(report)
		return report, nil
	}
	if parseErr != nil {
		report.Error = "invalid quota payload"
		report.Status = deriveStatus(report)
		return report, nil
	}
	report.PlanType = firstNonEmpty(normalizePlan(firstValue(parsedBody["plan_type"], parsedBody["planType"])), report.PlanType)
	report.Windows = parseCodexWindows(parsedBody)
	report.AdditionalWindows = parseAdditionalWindows(parsedBody)
	report.Status = deriveStatus(report)
	return report, nil
}

func fetchTokenUsageByAuth(ctx context.Context, cfg config, now time.Time) (tokenUsageResult, error) {
	client := &http.Client{Timeout: cfg.Timeout}
	payload, err := fetchJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/usage")
	if err != nil {
		return tokenUsageResult{}, err
	}
	return parseTokenUsageByAuth(payload, now), nil
}

func fetchJSON(ctx context.Context, client *http.Client, cfg config, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.ManagementKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ManagementKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeResponse(resp)
}

func postJSON(ctx context.Context, client *http.Client, cfg config, url string, payload map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if cfg.ManagementKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ManagementKey)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeResponse(resp)
}

func decodeResponse(resp *http.Response) (map[string]any, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("management API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseBody(body any) (map[string]any, error) {
	switch v := body.(type) {
	case map[string]any:
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, errors.New("empty body")
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, errors.New("invalid body")
	}
}

func bodyString(body any) string {
	switch v := body.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case nil:
		return ""
	default:
		raw, _ := json.Marshal(v)
		return strings.TrimSpace(string(raw))
	}
}

func parseCodexWindows(payload map[string]any) []quotaWindow {
	rateLimit, _ := firstValue(payload["rate_limit"], payload["rateLimit"]).(map[string]any)
	fiveHour, weekly := findQuotaWindows(rateLimit)
	limitReached := firstValue(rateLimit["limit_reached"], rateLimit["limitReached"])
	allowed := rateLimit["allowed"]
	var windows []quotaWindow
	if window := buildWindow("code-5h", "Code 5h", fiveHour, limitReached, allowed); window != nil {
		windows = append(windows, *window)
	}
	if window := buildWindow("code-7d", "Code 7d", weekly, limitReached, allowed); window != nil {
		windows = append(windows, *window)
	}
	return windows
}

func parseAdditionalWindows(payload map[string]any) []quotaWindow {
	raw, ok := firstValue(payload["additional_rate_limits"], payload["additionalRateLimits"]).([]any)
	if !ok {
		return nil
	}
	var windows []quotaWindow
	for i, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rateLimit, ok := firstValue(entry["rate_limit"], entry["rateLimit"]).(map[string]any)
		if !ok {
			continue
		}
		name := cleanString(firstValue(entry["limit_name"], entry["limitName"], entry["metered_feature"], entry["meteredFeature"]))
		if name == "" {
			name = fmt.Sprintf("additional-%d", i+1)
		}
		if window := buildWindow(name+"-primary", name+" 5h", firstMap(rateLimit, "primary_window", "primaryWindow"), firstValue(rateLimit["limit_reached"], rateLimit["limitReached"]), rateLimit["allowed"]); window != nil {
			windows = append(windows, *window)
		}
		if window := buildWindow(name+"-secondary", name+" 7d", firstMap(rateLimit, "secondary_window", "secondaryWindow"), firstValue(rateLimit["limit_reached"], rateLimit["limitReached"]), rateLimit["allowed"]); window != nil {
			windows = append(windows, *window)
		}
	}
	return windows
}

func firstMap(m map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if raw, ok := m[key].(map[string]any); ok {
			return raw
		}
	}
	return nil
}

func findQuotaWindows(rateLimit map[string]any) (map[string]any, map[string]any) {
	if rateLimit == nil {
		return nil, nil
	}
	primary := firstMap(rateLimit, "primary_window", "primaryWindow")
	secondary := firstMap(rateLimit, "secondary_window", "secondaryWindow")
	candidates := []map[string]any{primary, secondary}
	var fiveHour, weekly map[string]any
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		duration := numberFromAny(firstValue(candidate["limit_window_seconds"], candidate["limitWindowSeconds"]))
		if duration == window5HSeconds && fiveHour == nil {
			fiveHour = candidate
		}
		if duration == window7DSeconds && weekly == nil {
			weekly = candidate
		}
	}
	if fiveHour == nil {
		fiveHour = primary
	}
	if weekly == nil {
		weekly = secondary
	}
	return fiveHour, weekly
}

func buildWindow(id, label string, window map[string]any, limitReached, allowed any) *quotaWindow {
	if window == nil {
		return nil
	}
	usedPercent := deduceUsedPercent(window, limitReached, allowed)
	var remaining *float64
	if usedPercent != nil {
		v := clampFloat(100-*usedPercent, 0, 100)
		remaining = &v
	}
	return &quotaWindow{ID: id, Label: label, UsedPercent: usedPercent, RemainingPercent: remaining, ResetLabel: formatResetLabel(window), Exhausted: usedPercent != nil && *usedPercent >= 100}
}

func deduceUsedPercent(window map[string]any, limitReached, allowed any) *float64 {
	if used := numberPtr(firstValue(window["used_percent"], window["usedPercent"])); used != nil {
		v := clampFloat(*used, 0, 100)
		return &v
	}
	if (boolFromAny(limitReached) || isFalse(allowed)) && formatResetLabel(window) != "-" {
		v := 100.0
		return &v
	}
	return nil
}

func formatResetLabel(window map[string]any) string {
	if ts := numberFromAny(firstValue(window["reset_at"], window["resetAt"])); ts > 0 {
		return time.Unix(int64(ts), 0).Local().Format("01-02 15:04")
	}
	if secs := numberFromAny(firstValue(window["reset_after_seconds"], window["resetAfterSeconds"])); secs > 0 {
		return time.Now().Add(time.Duration(secs) * time.Second).Local().Format("01-02 15:04")
	}
	return "-"
}

func deriveStatus(report quotaReport) string {
	if report.Disabled {
		return "disabled"
	}
	if report.Error != "" {
		return "error"
	}
	if report.AuthIndex == "" || report.AccountID == "" {
		return "missing"
	}
	window7d := findWindow(report.Windows, "code-7d")
	if window7d == nil || window7d.RemainingPercent == nil {
		return "unknown"
	}
	remaining := *window7d.RemainingPercent
	switch {
	case remaining <= 0:
		return "exhausted"
	case remaining <= 30:
		return "low"
	case remaining <= 70:
		return "medium"
	case remaining < 100:
		return "high"
	default:
		return "full"
	}
}

func parseTokenUsageByAuth(payload map[string]any, now time.Time) tokenUsageResult {
	usage, _ := payload["usage"].(map[string]any)
	apis, _ := usage["apis"].(map[string]any)
	if len(apis) == 0 {
		return tokenUsageResult{ByAuth: map[string]tokenUsageSummary{}}
	}
	if now.IsZero() {
		now = time.Now()
	}
	last7Hours := now.Add(-7 * time.Hour)
	last24Hours := now.Add(-24 * time.Hour)
	last7Days := now.Add(-7 * 24 * time.Hour)
	out := make(map[string]tokenUsageSummary)
	var historyStart, historyEnd time.Time
	for _, apiValue := range apis {
		apiEntry, ok := apiValue.(map[string]any)
		if !ok {
			continue
		}
		models, _ := apiEntry["models"].(map[string]any)
		for _, modelValue := range models {
			modelEntry, ok := modelValue.(map[string]any)
			if !ok {
				continue
			}
			details, _ := modelEntry["details"].([]any)
			for _, detailValue := range details {
				detail, ok := detailValue.(map[string]any)
				if !ok {
					continue
				}
				authIndex := cleanString(firstValue(detail["auth_index"], detail["authIndex"]))
				timestampText := cleanString(detail["timestamp"])
				if authIndex == "" || timestampText == "" {
					continue
				}
				timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
				if err != nil {
					continue
				}
				if historyStart.IsZero() || timestamp.Before(historyStart) {
					historyStart = timestamp
				}
				if historyEnd.IsZero() || timestamp.After(historyEnd) {
					historyEnd = timestamp
				}
				totalTokens := tokenTotalFromDetail(detail)
				current := out[authIndex]
				current.Available = true
				current.AllTime += totalTokens
				if !timestamp.Before(last7Hours) {
					current.Last7Hours += totalTokens
				}
				if !timestamp.Before(last24Hours) {
					current.Last24Hours += totalTokens
				}
				if !timestamp.Before(last7Days) {
					current.Last7Days += totalTokens
				}
				out[authIndex] = current
			}
		}
	}
	result := tokenUsageResult{ByAuth: out, HistoryStart: historyStart, HistoryEnd: historyEnd}
	if historyStart.IsZero() {
		return result
	}
	result.Complete7Hours = !historyStart.After(last7Hours)
	result.Complete24Hours = !historyStart.After(last24Hours)
	result.Complete7Days = !historyStart.After(last7Days)
	return result
}

func summarize(reports []quotaReport) summary {
	sum := summary{Accounts: len(reports), StatusCounts: map[string]int{}, PlanCounts: map[string]int{}}
	for _, report := range reports {
		sum.StatusCounts[report.Status]++
		plan := report.PlanType
		if plan == "" {
			plan = "unknown"
		}
		sum.PlanCounts[plan]++
		window7d := findWindow(report.Windows, "code-7d")
		if window7d != nil && window7d.RemainingPercent != nil {
			switch strings.ToLower(strings.TrimSpace(report.PlanType)) {
			case "free":
				sum.FreeEquivalent7D += *window7d.RemainingPercent
			case "plus":
				sum.PlusEquivalent7D += *window7d.RemainingPercent
			}
		}
		if report.tokenUsage.Available {
			if !sum.TokenUsage.Available {
				sum.TokenUsage.Available = true
				sum.TokenUsage.HistoryStart = report.tokenUsage.HistoryStart
				sum.TokenUsage.HistoryEnd = report.tokenUsage.HistoryEnd
				sum.TokenUsage.Complete7Hours = report.tokenUsage.Complete7Hours
				sum.TokenUsage.Complete24Hours = report.tokenUsage.Complete24Hours
				sum.TokenUsage.Complete7Days = report.tokenUsage.Complete7Days
			}
			sum.TokenUsage.Last7Hours += report.tokenUsage.Last7Hours
			sum.TokenUsage.Last24Hours += report.tokenUsage.Last24Hours
			sum.TokenUsage.Last7Days += report.tokenUsage.Last7Days
			sum.TokenUsage.AllTime += report.tokenUsage.AllTime
		}
	}
	return sum
}

func sortReportsDefault(reports []quotaReport) {
	planRank := func(plan string) int {
		switch strings.ToLower(strings.TrimSpace(plan)) {
		case "free":
			return 0
		case "team":
			return 1
		case "plus":
			return 2
		default:
			return 3
		}
	}
	remaining7d := func(report quotaReport) float64 {
		window := findWindow(report.Windows, "code-7d")
		if window == nil || window.RemainingPercent == nil {
			return 101
		}
		return *window.RemainingPercent
	}
	sort.SliceStable(reports, func(i, j int) bool {
		li, lj := planRank(reports[i].PlanType), planRank(reports[j].PlanType)
		if li != lj {
			return li < lj
		}
		ri, rj := remaining7d(reports[i]), remaining7d(reports[j])
		if ri != rj {
			return ri < rj
		}
		return strings.ToLower(reports[i].Name) < strings.ToLower(reports[j].Name)
	})
}

func mergeMaps(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func configNow(cfg config) time.Time {
	if cfg.Now != nil {
		return cfg.Now()
	}
	return time.Now()
}
