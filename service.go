package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type monitorService struct {
	cfg       config
	state     runtimeState
	client    *http.Client
	push      func(string) error
	fetch     func(context.Context, config) (summary, []quotaReport, error)
	stdoutNow func() time.Time
}

func newMonitorService(cfg config) (*monitorService, error) {
	state, err := loadRuntimeState(cfg.StatePath)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: cfg.Timeout}
	svc := &monitorService{
		cfg:       cfg,
		state:     state,
		client:    client,
		fetch:     fetchMonitorData,
		stdoutNow: configNowFunc(cfg),
	}
	svc.push = func(text string) error {
		fmt.Println(text)
		if cfg.DryRun || strings.TrimSpace(cfg.FeishuWebhook) == "" {
			return nil
		}
		return pushToFeishu(client, cfg.FeishuWebhook, cfg.FeishuSecret, text)
	}
	return svc, nil
}

func (s *monitorService) runOnce(ctx context.Context) (summary, []quotaReport, error) {
	sum, reports, err := s.fetch(ctx, s.cfg)
	if err != nil {
		return summary{}, nil, err
	}
	if err := s.push(buildSummaryTextMessage(sum, s.cfg.BaseURL, s.stdoutNow())); err != nil {
		return summary{}, nil, err
	}
	if s.cfg.PrintJSON {
		printJSONPayload(s.cfg.BaseURL, sum, reports)
	}
	return sum, reports, nil
}

func (s *monitorService) serve(ctx context.Context) error {
	if s.cfg.StartupNotificationEnabled {
		if err := s.push(buildStartupText(s.cfg)); err != nil {
			fmt.Fprintf(os.Stderr, "startup_notification_failed=%v\n", err)
		}
	}
	if s.cfg.RunSummaryOnStartup {
		if err := s.runSummaryCycle(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "initial_summary_failed=%v\n", err)
		}
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			now := s.stdoutNow()
			if shouldSendByInterval(s.state.LastSummaryAt, s.cfg.PollInterval, now) {
				if err := s.runSummaryCycle(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "summary_cycle_failed=%v\n", err)
				}
			}
			if s.cfg.HeartbeatEnabled && shouldSendByInterval(s.state.LastHeartbeatAt, s.cfg.HeartbeatInterval, now) {
				if err := s.runHeartbeatCycle(); err != nil {
					fmt.Fprintf(os.Stderr, "heartbeat_cycle_failed=%v\n", err)
				}
			}
		}
	}
}

func (s *monitorService) runSummaryCycle(ctx context.Context) error {
	now := s.stdoutNow()
	sum, reports, err := s.fetch(ctx, s.cfg)
	if err != nil {
		s.state.ConsecutiveFailures++
		s.state.LastError = err.Error()
		s.state.LastErrorAt = now.UTC().Format(time.RFC3339Nano)
		if saveErr := saveRuntimeState(s.cfg.StatePath, s.state); saveErr != nil {
			return fmt.Errorf("summary failed: %v; save state failed: %w", err, saveErr)
		}
		if s.state.ConsecutiveFailures >= s.cfg.FailureAlertThreshold {
			alert := buildFailureAlertText(s.state, err, s.cfg)
			if pushErr := s.push(alert); pushErr != nil {
				return fmt.Errorf("summary failed: %v; alert push failed: %w", err, pushErr)
			}
		}
		return err
	}
	message := buildSummaryTextMessage(sum, s.cfg.BaseURL, now)
	if err := s.push(message); err != nil {
		return err
	}
	if s.cfg.PrintJSON {
		printJSONPayload(s.cfg.BaseURL, sum, reports)
	}
	s.state.LastSummaryAt = now.UTC().Format(time.RFC3339Nano)
	s.state.LastSuccessAt = s.state.LastSummaryAt
	s.state.LastError = ""
	s.state.ConsecutiveFailures = 0
	s.state.LastSummary = &sum
	return saveRuntimeState(s.cfg.StatePath, s.state)
}

func (s *monitorService) runHeartbeatCycle() error {
	now := s.stdoutNow()
	message := buildHeartbeatText(s.state, s.cfg.HeartbeatInterval)
	if err := s.push(message); err != nil {
		return err
	}
	s.state.LastHeartbeatAt = now.UTC().Format(time.RFC3339Nano)
	return saveRuntimeState(s.cfg.StatePath, s.state)
}

func shouldSendByInterval(last string, interval time.Duration, now time.Time) bool {
	if strings.TrimSpace(last) == "" {
		return true
	}
	if interval <= 0 {
		return true
	}
	previous, err := time.Parse(time.RFC3339Nano, last)
	if err != nil {
		return true
	}
	return !now.Before(previous.Add(interval))
}
