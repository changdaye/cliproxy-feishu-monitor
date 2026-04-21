package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func loadRuntimeState(path string) (runtimeState, error) {
	if path == "" {
		return runtimeState{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runtimeState{}, nil
		}
		return runtimeState{}, fmt.Errorf("read runtime state %s: %w", path, err)
	}
	var state runtimeState
	if err := json.Unmarshal(raw, &state); err != nil {
		return runtimeState{}, fmt.Errorf("parse runtime state %s: %w", path, err)
	}
	return state, nil
}

func saveRuntimeState(path string, state runtimeState) error {
	if path == "" {
		return nil
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime state: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write runtime state %s: %w", path, err)
	}
	return nil
}
