package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type checkout struct {
	Worktree string
	Main     string
	Branch   string
}

// Git documents that worktree list returns the main checkout first. -z
// preserves paths verbatim, including spaces, quotes, and embedded newlines.
func discover(event createdEvent) (checkout, error) {
	var result checkout
	worktree, err := filepath.EvalSymlinks(event.Data.Worktree.Path)
	if err != nil {
		return result, fmt.Errorf("cannot access new worktree: %w", err)
	}
	output, err := gitOutput(worktree, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return result, err
	}
	records := strings.Split(output, "\x00\x00")
	first := strings.Split(records[0], "\x00")
	if !strings.HasPrefix(first[0], "worktree ") {
		return result, fmt.Errorf("cannot determine main checkout from git worktree list")
	}
	for _, field := range first {
		if field == "bare" {
			return result, fmt.Errorf("repository is bare; a primary/main checkout is required")
		}
	}
	main, err := filepath.EvalSymlinks(strings.TrimPrefix(first[0], "worktree "))
	if err != nil {
		return result, fmt.Errorf("cannot access main checkout: %w", err)
	}
	// Ensure the event identifies an actual checkout of this repository.
	found := false
	for _, record := range records {
		field := strings.Split(record, "\x00")[0]
		if !strings.HasPrefix(field, "worktree ") {
			continue
		}
		path, err := filepath.EvalSymlinks(strings.TrimPrefix(field, "worktree "))
		if err == nil && path == worktree {
			found = true
		}
	}
	if !found {
		return result, fmt.Errorf("event path is not a registered Git worktree")
	}
	branch := ""
	if !event.Data.Worktree.IsDetached {
		if event.Data.Worktree.Branch != nil {
			branch = *event.Data.Worktree.Branch
		} else {
			output, err := gitOutput(worktree, "branch", "--show-current")
			if err != nil {
				return result, err
			}
			branch = strings.TrimSuffix(output, "\n")
		}
	}
	return checkout{Worktree: worktree, Main: main, Branch: branch}, nil
}

func gitOutput(worktree string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", worktree}, args...)...)
	// Inherited Git routing variables must not redirect discovery to a different
	// checkout. Setup commands themselves still inherit the user's environment.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	output, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("Git discovery failed: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("cannot run Git discovery: %w", err)
	}
	return string(output), nil
}
