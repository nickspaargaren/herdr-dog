package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	main, worktree, branch string
}

func newFixture(t *testing.T, yaml, branch string) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{
		main:     filepath.Join(root, "main checkout 'quoted';$"),
		worktree: filepath.Join(root, "new checkout 'quoted';$\nline"),
		branch:   branch,
	}
	if err := os.MkdirAll(f.main, 0755); err != nil {
		t.Fatal(err)
	}
	f.main, _ = filepath.EvalSymlinks(f.main)
	// macOS temp paths can themselves be symlinks.
	root, _ = filepath.EvalSymlinks(root)
	f.worktree = filepath.Join(root, filepath.Base(f.worktree))
	git(t, f.main, "init", "--quiet")
	writeFile(t, filepath.Join(f.main, "README.md"), "fixture\n")
	if yaml != "" {
		writeFile(t, filepath.Join(f.main, configPath), yaml)
	}
	git(t, f.main, "add", ".")
	git(t, f.main, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "fixture")
	if branch == "" {
		git(t, f.main, "worktree", "add", "--quiet", "--detach", f.worktree, "HEAD")
	} else {
		git(t, f.main, "worktree", "add", "--quiet", "-b", branch, f.worktree, "HEAD")
	}
	return f
}

func git(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (f fixture) event(t *testing.T, includeBranch bool) string {
	t.Helper()
	event := createdEvent{Event: "worktree_created"}
	event.Data.Type = "worktree_created"
	event.Data.Worktree.Path = f.worktree
	event.Data.Worktree.IsDetached = f.branch == ""
	if includeBranch && f.branch != "" {
		event.Data.Worktree.Branch = &f.branch
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMissingConfigIsSilent(t *testing.T) {
	f := newFixture(t, "", "feature")
	var stdout, stderr bytes.Buffer
	if err := HandleEvent(f.event(t, true), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("unexpected output: %q / %q", stdout.String(), stderr.String())
	}
}

func TestMainCheckoutConfigFallback(t *testing.T) {
	for _, ignored := range []bool{false, true} {
		t.Run(map[bool]string{false: "untracked", true: "gitignored"}[ignored], func(t *testing.T) {
			f := newFixture(t, "", "local-config")
			if ignored {
				writeFile(t, filepath.Join(f.main, ".gitignore"), ".herdr/\n")
			}
			writeFile(t, filepath.Join(f.main, configPath), "version: 1\nworktrees:\n  setup:\n    - run: cp \"$HERDR_MAIN_WORKTREE/.env\" .env\n")
			writeFile(t, filepath.Join(f.main, ".env"), "LOCAL=fixture\n")
			var stdout, stderr bytes.Buffer
			if err := HandleEvent(f.event(t, true), &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, filepath.Join(f.worktree, ".env")); got != "LOCAL=fixture\n" {
				t.Fatalf("unexpected copy: %q", got)
			}
			if _, err := os.Lstat(filepath.Join(f.worktree, configPath)); !os.IsNotExist(err) {
				t.Fatal("fallback must not copy configuration into the new checkout")
			}
		})
	}
}

func TestNewCheckoutConfigPrecedence(t *testing.T) {
	f := newFixture(t, "version: 1\nworktrees: {setup: []}\n", "precedence")
	writeFile(t, filepath.Join(f.main, configPath), "version: 99")
	var stdout, stderr bytes.Buffer
	if err := HandleEvent(f.event(t, true), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidConfigDoesNotFallBack(t *testing.T) {
	f := newFixture(t, "version: 1\nworktrees: {setup: []}\n", "invalid-local")
	writeFile(t, filepath.Join(f.worktree, configPath), "version: 99")
	var stdout, stderr bytes.Buffer
	err := HandleEvent(f.event(t, true), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), filepath.Join(f.worktree, configPath)) || !strings.Contains(err.Error(), "unsupported config version") {
		t.Fatalf("expected new checkout validation error, got %v", err)
	}
}

func TestInvalidFallbackConfig(t *testing.T) {
	f := newFixture(t, "", "invalid-fallback")
	writeFile(t, filepath.Join(f.main, configPath), "version: 99")
	var stdout, stderr bytes.Buffer
	err := HandleEvent(f.event(t, true), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), filepath.Join(f.main, configPath)) {
		t.Fatalf("expected main checkout validation error, got %v", err)
	}
}

func TestDanglingConfigSymlinkDoesNotFallBack(t *testing.T) {
	f := newFixture(t, "version: 1\nworktrees: {setup: []}\n", "dangling")
	path := filepath.Join(f.worktree, configPath)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.worktree, "missing.yml"), path); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := HandleEvent(f.event(t, true), &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "cannot resolve") {
		t.Fatalf("expected dangling symlink error, got %v", err)
	}
}

func TestMinimalConfig(t *testing.T) {
	f := newFixture(t, `version: 1
worktrees:
  setup:
    - run: cp "$HERDR_MAIN_WORKTREE/.env" .env
`, "feature")
	writeFile(t, filepath.Join(f.main, ".env"), "SECRET=development\n")
	var stdout, stderr bytes.Buffer
	if err := HandleEvent(f.event(t, true), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(f.worktree, ".env")); got != "SECRET=development\n" {
		t.Fatalf("unexpected copy: %q", got)
	}
}

func TestStepsWorkingDirectoryAndEnvironment(t *testing.T) {
	t.Setenv("DOG_VALUE", "inherited")
	t.Setenv("HERDR_WORKTREE", "stale")
	t.Setenv("HERDR_MAIN_WORKTREE", "stale")
	t.Setenv("HERDR_BRANCH", "stale")
	f := newFixture(t, `version: 1
worktrees:
  setup:
    - name: First
      run: |
        printf '%s\n' "$HERDR_WORKTREE" "$HERDR_MAIN_WORKTREE" "$HERDR_BRANCH" "$DOG_VALUE" > result
        pwd -P > cwd
        printf '%s\n' one > order
        printf 'forwarded stdout\n'
        printf 'forwarded stderr\n' >&2
        export DOG_VALUE=changed-by-shell
        cd /
      env:
        DOG_VALUE: 'step value; $(false)'
        HERDR_WORKTREE: wrong
        HERDR_MAIN_WORKTREE: wrong
        HERDR_BRANCH: wrong
    - name: Second
      run: |
        printf '%s\n' "$DOG_VALUE" >> result
        printf '%s\n' two >> order
`, "feature/$(false);literal")
	var stdout, stderr bytes.Buffer
	if err := HandleEvent(f.event(t, true), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{f.worktree, f.main, f.branch, "step value; $(false)", "inherited", ""}, "\n")
	if got := readFile(t, filepath.Join(f.worktree, "result")); got != want {
		t.Fatalf("environment got %q, want %q", got, want)
	}
	if got := readFile(t, filepath.Join(f.worktree, "cwd")); got != f.worktree+"\n" {
		t.Fatalf("wrong working directory: %q", got)
	}
	if got := readFile(t, filepath.Join(f.worktree, "order")); got != "one\ntwo\n" {
		t.Fatalf("wrong execution order: %q", got)
	}
	if stdout.String() != "forwarded stdout\n" || !strings.Contains(stderr.String(), "forwarded stderr\n") {
		t.Fatalf("output not forwarded: %q / %q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `running "First"`) || !strings.Contains(stderr.String(), `running "Second"`) {
		t.Fatalf("missing named progress: %q", stderr.String())
	}
}

func TestFailureStopsRemainingSteps(t *testing.T) {
	for _, named := range []bool{false, true} {
		t.Run(map[bool]string{false: "command label", true: "name label"}[named], func(t *testing.T) {
			name := ""
			label := "exit 7"
			if named {
				name = "      name: Seed database\n"
				label = "Seed database"
			}
			f := newFixture(t, "version: 1\nworktrees:\n  setup:\n    - run: echo first > order\n    - run: exit 7\n"+name+"    - run: echo third >> order\n", "failure")
			var stdout, stderr bytes.Buffer
			err := HandleEvent(f.event(t, true), &stdout, &stderr)
			want := `step "` + label + `" failed with exit code 7`
			if err == nil || err.Error() != want {
				t.Fatalf("got %v, want %s", err, want)
			}
			if got := readFile(t, filepath.Join(f.worktree, "order")); got != "first\n" {
				t.Fatalf("continued after failure: %q", got)
			}
		})
	}
}

func TestWholeConfigValidatedBeforeExecution(t *testing.T) {
	f := newFixture(t, "version: 1\nworktrees:\n  setup:\n    - run: touch should-not-exist\n    - name: Missing run\n", "invalid")
	var stdout, stderr bytes.Buffer
	if err := HandleEvent(f.event(t, true), &stdout, &stderr); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := os.Stat(filepath.Join(f.worktree, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("executed before validating all steps")
	}
}

func TestBranchFallbackAndDetachedHEAD(t *testing.T) {
	for _, branch := range []string{"feature/fallback", ""} {
		t.Run(map[bool]string{true: "branch fallback", false: "detached"}[branch != ""], func(t *testing.T) {
			t.Setenv("HERDR_BRANCH", "stale-inherited-branch")
			f := newFixture(t, `version: 1
worktrees:
  setup:
    - run: printf '%s' "$HERDR_BRANCH" > branch
`, branch)
			var stdout, stderr bytes.Buffer
			if err := HandleEvent(f.event(t, false), &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, filepath.Join(f.worktree, "branch")); got != branch {
				t.Fatalf("branch got %q, want %q", got, branch)
			}
		})
	}
}

func TestConfigSymlinkContainment(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			f := newFixture(t, "", "symlink")
			outside := t.TempDir()
			writeFile(t, filepath.Join(outside, "worktrees.yml"), "version: 1\nworktrees: {setup: []}")
			target, link := filepath.Join(outside, "worktrees.yml"), filepath.Join(f.worktree, configPath)
			if directory {
				target, link = outside, filepath.Join(f.worktree, ".herdr")
			} else if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if err := HandleEvent(f.event(t, true), &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "must resolve inside") {
				t.Fatalf("got %v, want containment error", err)
			}
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			mainLink := filepath.Join(f.main, configPath)
			if directory {
				mainLink = filepath.Join(f.main, ".herdr")
			} else if err := os.MkdirAll(filepath.Dir(mainLink), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, mainLink); err != nil {
				t.Fatal(err)
			}
			if err := HandleEvent(f.event(t, true), &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "must resolve inside") {
				t.Fatalf("got %v, want fallback containment error", err)
			}
		})
	}
}

func TestDiscoveryIgnoresInheritedGitRouting(t *testing.T) {
	f := newFixture(t, "", "routing")
	t.Setenv("GIT_DIR", "/nonexistent/.git")
	t.Setenv("GIT_WORK_TREE", "/nonexistent")
	t.Setenv("GIT_COMMON_DIR", "/nonexistent/.git")
	event, err := parseEvent(f.event(t, true))
	if err != nil {
		t.Fatal(err)
	}
	got, err := discover(event)
	if err != nil || got.Main != f.main || got.Worktree != f.worktree {
		t.Fatalf("discovery got %+v, %v", got, err)
	}
}
