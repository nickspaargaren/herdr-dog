package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseTagMustMatchManifest(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "scripts", "build-release")
	writeFile(t, script, readFile(t, filepath.Join("..", "..", "scripts", "build-release")))
	writeFile(t, filepath.Join(root, "herdr-plugin.toml"), "version = \"0.1.0\"\n")
	cmd := exec.Command("/bin/sh", script, "v9.9.9")
	cmd.Dir = "/"
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "release tag v9.9.9 does not match manifest version v0.1.0") {
		t.Fatalf("unexpected mismatch result: %v, %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); !os.IsNotExist(err) {
		t.Fatal("mismatched tag must fail before building release artifacts")
	}
}
