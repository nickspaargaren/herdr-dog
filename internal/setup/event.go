package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

// Herdr v0.9.3 serializes EventEnvelope and EventData using snake_case,
// although the manifest hook name is worktree.created. Only fields used here
// are decoded; additional Herdr event fields remain forward-compatible.
type createdEvent struct {
	Event string `json:"event"`
	Data  struct {
		Type     string `json:"type"`
		Worktree struct {
			Path       string  `json:"path"`
			Branch     *string `json:"branch"`
			IsDetached bool    `json:"is_detached"`
		} `json:"worktree"`
	} `json:"data"`
}

func parseEvent(raw string) (createdEvent, error) {
	var event createdEvent
	if raw == "" {
		return event, fmt.Errorf("HERDR_PLUGIN_EVENT_JSON is required")
	}
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return event, fmt.Errorf("invalid Herdr event JSON: %w", err)
	}
	if event.Event != "worktree_created" || event.Data.Type != "worktree_created" {
		return event, fmt.Errorf("expected worktree_created event")
	}
	if !filepath.IsAbs(event.Data.Worktree.Path) {
		return event, fmt.Errorf("event data.worktree.path must be an absolute path")
	}
	return event, nil
}

// HandleEvent keeps Herdr integration separate from configuration and execution.
func HandleEvent(raw string, stdout, stderr io.Writer) error {
	event, err := parseEvent(raw)
	if err != nil {
		return err
	}
	checkout, err := discover(event)
	if err != nil {
		return err
	}
	config, err := loadConfig(checkout.Worktree)
	if err == nil && config == nil && checkout.Main != checkout.Worktree {
		config, err = loadConfig(checkout.Main)
	}
	if err != nil || config == nil {
		return err
	}
	return runSetup(*config, checkout, stdout, stderr)
}
