package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	mode, cfg, err := parseFlags(os.Args[1:], workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	service, err := newMonitorService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	switch mode {
	case "serve":
		if err := service.serve(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		if _, _, err := service.runOnce(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func printJSONPayload(baseURL string, sum summary, reports []quotaReport) {
	payload := map[string]any{"base_url": baseURL, "summary": sum, "reports": reports}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
