package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestExampleConfigurations(t *testing.T) {
	for _, name := range []string{"minimal", "node", "postgres"} {
		t.Run(name, func(t *testing.T) {
			text := readFile(t, filepath.Join("..", "..", "examples", name, "worktrees.yml"))
			if _, err := parseConfig(strings.NewReader(text)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPostgresScriptEscapeHatch(t *testing.T) {
	// Exercise the illustrative script with fake clients, never a real database.
	script, err := filepath.Abs(filepath.Join("..", "..", "examples", "postgres", "scripts", "worktree-db-setup"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	tools := filepath.Join(root, "tools")
	for _, name := range []string{"createdb", "psql"} {
		path := filepath.Join(tools, name)
		writeFile(t, path, "#!/bin/sh\nprintf '%s\\n' \""+name+" $*\" >> calls\n")
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	names := map[string]bool{}
	for i, branch := range []string{"feature/a-b", "feature/a+b", "$(touch injected);QUOTED"} {
		worktree := filepath.Join(root, string(rune('a'+i))+" checkout")
		if err := os.MkdirAll(worktree, 0755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("/bin/sh", script)
		cmd.Dir = worktree
		cmd.Env = append(os.Environ(), "HERDR_WORKTREE="+worktree, "HERDR_BRANCH="+branch, "PATH="+tools+":"+os.Getenv("PATH"))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("script failed: %v\n%s", err, output)
		}
		name := strings.TrimSpace(readFile(t, filepath.Join(worktree, ".worktree-database")))
		if !regexp.MustCompile(`^wt_[a-z0-9_]+$`).MatchString(name) || len(name) > 63 || names[name] {
			t.Fatalf("unsafe or duplicate database name: %q", name)
		}
		names[name] = true
		want := "createdb " + name + "\n" +
			"psql --dbname=" + name + " --set=ON_ERROR_STOP=1 --file=./db/migrations.sql\n" +
			"psql --dbname=" + name + " --set=ON_ERROR_STOP=1 --file=./db/fixtures.sql\n"
		if got := readFile(t, filepath.Join(worktree, "calls")); got != want {
			t.Fatalf("unexpected database commands: %q", got)
		}
		if _, err := os.Stat(filepath.Join(worktree, "injected")); !os.IsNotExist(err) {
			t.Fatal("shell-significant branch name was evaluated")
		}
	}
}
