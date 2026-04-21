package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func buildSummaryTextMessage(sum summary, baseURL string, now time.Time) string {
	var b strings.Builder
	b.WriteString("状态概览\n")
	if baseURL != "" {
		b.WriteString("来源: ")
		b.WriteString(baseURL)
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("时间: %s\n\n", now.Format("2006-01-02 15:04:05 MST")))
	b.WriteString(buildSummarySnapshotLine(sum))
	if disabled := sum.StatusCounts["disabled"]; disabled > 0 {
		b.WriteString(fmt.Sprintf(" | 禁用 %d", disabled))
	}
	if errs := sum.StatusCounts["error"] + sum.StatusCounts["missing"]; errs > 0 {
		b.WriteString(fmt.Sprintf(" | 异常 %d", errs))
	}
	b.WriteString("\n\n汇总\n")
	b.WriteString(fmt.Sprintf("7日免费等效: %.0f%%\n", sum.FreeEquivalent7D))
	b.WriteString(fmt.Sprintf("7小时 Token 用量: %s\n", formatNumberWithCommas(sum.TokenUsage.Last7Hours)))
	b.WriteString(fmt.Sprintf("24小时 Token 用量: %s\n", formatNumberWithCommas(sum.TokenUsage.Last24Hours)))
	b.WriteString(fmt.Sprintf("7天 Token 用量: %s\n", formatNumberWithCommas(sum.TokenUsage.Last7Days)))
	b.WriteString(fmt.Sprintf("累计 Token 用量: %s", formatNumberWithCommas(sum.TokenUsage.AllTime)))
	return b.String()
}

func buildFeishuTextMessage(sum summary, baseURL string) string {
	return buildSummaryTextMessage(sum, baseURL, time.Now())
}

func buildHeartbeatText(state runtimeState, heartbeatInterval time.Duration) string {
	var b strings.Builder
	b.WriteString("健康心跳\n")
	if state.LastSuccessAt != "" {
		b.WriteString("上次成功: ")
		b.WriteString(state.LastSuccessAt)
		b.WriteString("\n")
	}
	if state.LastSummaryAt != "" {
		b.WriteString("上次汇总: ")
		b.WriteString(state.LastSummaryAt)
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("心跳间隔: %v\n", heartbeatInterval))
	b.WriteString(fmt.Sprintf("连续失败: %d\n", state.ConsecutiveFailures))
	if state.LastError != "" {
		b.WriteString("最近错误: ")
		b.WriteString(state.LastError)
		b.WriteString("\n")
	}
	if state.LastSummary != nil {
		b.WriteString("\n")
		b.WriteString(buildSummarySnapshotLine(*state.LastSummary))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("24小时 Token 用量: %s", formatNumberWithCommas(state.LastSummary.TokenUsage.Last24Hours)))
	}
	return b.String()
}

func buildStartupText(cfg config) string {
	return fmt.Sprintf("服务启动成功\n汇总推送间隔: %v\n心跳间隔: %v\n启动后立即汇总: %t\n心跳开关: %t", cfg.PollInterval, cfg.HeartbeatInterval, cfg.RunSummaryOnStartup, cfg.HeartbeatEnabled)
}

func buildFailureAlertText(state runtimeState, runErr error, cfg config) string {
	return fmt.Sprintf("监控异常提醒\n连续失败: %d\n失败阈值: %d\n最近错误: %v\n上次成功: %s", state.ConsecutiveFailures, cfg.FailureAlertThreshold, runErr, emptyFallback(state.LastSuccessAt, "无"))
}

func buildSummarySnapshotLine(sum summary) string {
	return fmt.Sprintf("账号总数 %d | 充足 %d | 高 %d | 中 %d | 低 %d | 耗尽 %d", sum.Accounts, sum.StatusCounts["full"], sum.StatusCounts["high"], sum.StatusCounts["medium"], sum.StatusCounts["low"], sum.StatusCounts["exhausted"])
}

func buildFeishuSignedPayload(text, secret string, now func() time.Time) ([]byte, error) {
	if now == nil {
		now = time.Now
	}
	body := map[string]any{
		"msg_type": "text",
		"content": map[string]any{
			"text": text,
		},
	}
	if strings.TrimSpace(secret) != "" {
		timestamp := fmt.Sprintf("%d", now().Unix())
		stringToSign := timestamp + "\n" + secret
		mac := hmac.New(sha256.New, []byte(stringToSign))
		body["timestamp"] = timestamp
		body["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return json.Marshal(body)
}

func pushToFeishu(client *http.Client, webhook, secret, text string) error {
	payload, err := buildFeishuSignedPayload(text, secret, time.Now)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu webhook HTTP %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil
	}
	if code := intFromAny(firstValue(out["code"], out["StatusCode"])); code != 0 {
		return fmt.Errorf("feishu webhook error %d: %s", code, cleanString(firstValue(out["msg"], out["message"])))
	}
	return nil
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
