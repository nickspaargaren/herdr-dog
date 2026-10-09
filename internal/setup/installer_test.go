package setup

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryInstaller(t *testing.T) {
	installer := readFile(t, filepath.Join("..", "..", "scripts", "install-binary"))
	tests := []struct {
		name, os, arch, failure, want string
	}{
		{"macOS ARM64", "Darwin", "arm64", "", ""},
		{"macOS x86-64", "Darwin", "x86_64", "", ""},
		{"Linux ARM64", "Linux", "aarch64", "", ""},
		{"Linux x86-64", "Linux", "x86_64", "", ""},
		{"unsupported OS", "Windows", "x86_64", "", "unsupported operating system"},
		{"unsupported arch", "Linux", "riscv64", "", "unsupported architecture"},
		{"failed binary download", "Linux", "x86_64", "download", "cannot download herdr-dog_linux_amd64"},
		{"failed checksum download", "Linux", "x86_64", "checksums", "cannot download checksums"},
		{"corrupt binary", "Linux", "x86_64", "corrupt", "checksum verification failed"},
		{"missing checksum", "Linux", "x86_64", "missing", "exactly one entry"},
		{"duplicate checksum", "Linux", "x86_64", "duplicate", "exactly one entry"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "plugin checkout with spaces")
			tools := filepath.Join(root, "tools")
			script := filepath.Join(root, "scripts", "install-binary")
			writeFile(t, script, installer)
			writeFile(t, filepath.Join(root, "herdr-plugin.toml"), "version = \"0.1.0\"\n")
			writeFile(t, filepath.Join(root, "bin", "herdr-dog"), "previous binary")
			writeFile(t, filepath.Join(tools, "uname"), "#!/bin/sh\ncase $1 in\n-s) printf '%s\\n' \"$TEST_OS\";;\n-m) printf '%s\\n' \"$TEST_ARCH\";;\nesac\n")
			// Fake only the HTTP transport and platform detection. The real SHA-256
			// utility must verify the bytes before installation; no network is used.
			writeFile(t, filepath.Join(tools, "curl"), `#!/bin/sh
set -eu
url= output=
while [ "$#" -gt 0 ]; do
    case $1 in
        https://*) url=$1 ;;
        --output) shift; output=$1 ;;
    esac
    shift
done
printf '%s\n' "$url" >> "$TEST_ROOT/urls"
case $url in
    https://github.com/nickspaargaren/herdr-dog/releases/download/v0.1.0/*) ;;
    *) exit 9 ;;
esac
case $url in
    */SHA256SUMS)
        [ "$TEST_FAILURE" != checksums ] || exit 22
        cp "$TEST_ROOT/SHA256SUMS" "$output" ;;
    *)
        [ "$TEST_FAILURE" != download ] || exit 22
        if [ "$TEST_FAILURE" = corrupt ]; then
            printf 'corrupt' > "$output"
        else
            cp "$TEST_ROOT/asset" "$output"
        fi ;;
esac
`)
			for _, name := range []string{"uname", "curl"} {
				if err := os.Chmod(filepath.Join(tools, name), 0755); err != nil {
					t.Fatal(err)
				}
			}
			binary := "verified prebuilt binary\n"
			writeFile(t, filepath.Join(root, "asset"), binary)
			osName := strings.ToLower(test.os)
			arch := "amd64"
			if test.arch == "aarch64" || test.arch == "arm64" {
				arch = "arm64"
			}
			asset := "herdr-dog_" + osName + "_" + arch
			line := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(binary)), asset)
			checksums := line
			if test.failure == "missing" {
				checksums = strings.Replace(line, asset, "unrelated", 1)
			} else if test.failure == "duplicate" {
				checksums += line
			}
			writeFile(t, filepath.Join(root, "SHA256SUMS"), checksums)
			cmd := exec.Command("/bin/sh", script)
			cmd.Dir = "/" // Installation must not depend on the caller's directory.
			cmd.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"),
				"TEST_ROOT="+root, "TEST_OS="+test.os, "TEST_ARCH="+test.arch, "TEST_FAILURE="+test.failure)
			output, err := cmd.CombinedOutput()
			if test.want != "" {
				if err == nil || !strings.Contains(string(output), test.want) {
					t.Fatalf("got %v, %s; want %q", err, output, test.want)
				}
				if got := readFile(t, filepath.Join(root, "bin", "herdr-dog")); got != "previous binary" {
					t.Fatal("failed installation replaced the existing binary")
				}
			} else {
				if err != nil {
					t.Fatalf("installer failed: %v\n%s", err, output)
				}
				if got := readFile(t, filepath.Join(root, "bin", "herdr-dog")); got != binary {
					t.Fatalf("unexpected installed bytes: %q", got)
				}
				info, err := os.Stat(filepath.Join(root, "bin", "herdr-dog"))
				if err != nil || info.Mode().Perm() != 0755 {
					t.Fatalf("installed binary permissions: %v, %v", info, err)
				}
				urls := readFile(t, filepath.Join(root, "urls"))
				if !strings.Contains(urls, "/v0.1.0/"+asset+"\n") || !strings.Contains(urls, "/v0.1.0/SHA256SUMS\n") {
					t.Fatalf("unexpected release URLs: %s", urls)
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(root, "bin", ".download.*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary downloads not cleaned up: %v, %v", leftovers, err)
			}
		})
	}
}
