package setup

import (
	"strings"
	"testing"
)

func TestParseHerdrEvent(t *testing.T) {
	// Fields and enum spelling verified against Herdr v0.9.3's EventEnvelope,
	// EventData, WorktreeInfo, and WorkspaceInfo, not the create CLI response.
	raw := `{
  "event": "worktree_created",
  "data": {
    "type": "worktree_created",
    "workspace": {
      "workspace_id": "w2", "number": 2, "label": "feature", "focused": false,
      "pane_count": 1, "tab_count": 1, "active_tab_id": "w2:t1", "agent_status": "unknown",
      "worktree": {
        "repo_key": "/repo/.git", "repo_name": "repo", "repo_root": "/repo",
        "checkout_path": "/new checkout", "is_linked_worktree": true
      }
    },
    "worktree": {
      "path": "/new checkout", "branch": "feature/$(touch-injection);test",
      "is_bare": false, "is_detached": false, "is_prunable": false,
      "is_linked_worktree": true, "open_workspace_id": "w2", "label": "repo"
    }
  }
}`
	event, err := parseEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if event.Data.Worktree.Path != "/new checkout" || *event.Data.Worktree.Branch != "feature/$(touch-injection);test" {
		t.Fatalf("unexpected event: %+v", event)
	}
	for _, bad := range []string{
		"", "{", "null",
		strings.Replace(raw, `"event": "worktree_created"`, `"event": "worktree.created"`, 1),
		strings.Replace(raw, `"type": "worktree_created"`, `"type": "worktree_opened"`, 1),
		strings.ReplaceAll(raw, `"/new checkout"`, `"relative"`),
		`{"id":"cli","result":{"type":"worktree_created","worktree":{"path":"/new"}}}`,
	} {
		if _, err := parseEvent(bad); err == nil {
			t.Fatalf("accepted invalid event: %s", bad)
		}
	}
}
