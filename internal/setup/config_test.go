package setup

import (
	"strings"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	valid := "version: 1\nworktrees:\n  setup:\n    - run: echo ready\n"
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"minimal", valid, ""},
		{"empty steps", "version: 1\nworktrees: {setup: []}", ""},
		{"block command", "version: 1\nworktrees:\n  setup:\n    - run: |\n        echo one\n        echo two", ""},
		{"unsupported version", strings.Replace(valid, "version: 1", "version: 2", 1), "unsupported config version: 2"},
		{"malformed YAML", "version: [", "invalid .herdr/worktrees.yml"},
		{"empty document", "", "invalid .herdr/worktrees.yml"},
		{"null document", "null", "config is required"},
		{"missing version", "worktrees: {setup: []}", "version is required"},
		{"string version", "version: '1'\nworktrees: {setup: []}", "version is required"},
		{"float version", "version: 1.0\nworktrees: {setup: []}", "version is required"},
		{"missing worktrees", "version: 1", "worktrees is required"},
		{"missing setup", "version: 1\nworktrees: {}", "worktrees.setup is required"},
		{"null setup", "version: 1\nworktrees: {setup: null}", "worktrees.setup is required"},
		{"missing run", "version: 1\nworktrees: {setup: [{name: Install}]}", "setup[0].run is required"},
		{"second missing run", "version: 1\nworktrees: {setup: [{run: echo hi}, {}]}", "setup[1].run is required"},
		{"empty run", "version: 1\nworktrees: {setup: [{run: '  '}]}", "setup[0].run is required"},
		{"number run", "version: 1\nworktrees: {setup: [{run: 123}]}", "setup[0].run must be a string"},
		{"boolean name", "version: 1\nworktrees: {setup: [{run: echo hi, name: true}]}", "setup[0].name must be a string"},
		{"null env", "version: 1\nworktrees: {setup: [{run: echo hi, env: null}]}", "setup[0].env is required"},
		{"number env", "version: 1\nworktrees: {setup: [{run: echo hi, env: {PORT: 3000}}]}", "env.PORT must be a string"},
		{"invalid env name", "version: 1\nworktrees: {setup: [{run: echo hi, env: {'BAD=KEY': value}}]}", "invalid variable name"},
		{"NUL env", `version: 1
worktrees: {setup: [{run: echo hi, env: {VALUE: "\0"}}]}`, "without NUL"},
		{"unknown root", valid + "conditions: true\n", "unknown field"},
		{"unknown worktrees", "version: 1\nworktrees: {setup: [], teardown: []}", "unknown field"},
		{"unknown step", "version: 1\nworktrees: {setup: [{run: echo hi, retries: 3}]}", "unknown field"},
		{"duplicate root", valid + "version: 1\n", "duplicate key"},
		{"duplicate step", "version: 1\nworktrees: {setup: [{run: echo hi, run: echo bye}]}", "duplicate key"},
		{"duplicate env", "version: 1\nworktrees: {setup: [{run: echo hi, env: {PORT: '3', PORT: '4'}}]}", "duplicate key"},
		{"multiple documents", valid + "---\n" + valid, "only one YAML document"},
		{"merge keys", "version: 1\nworktrees: {setup: [{<<: {run: echo hi}}]}", "must be a string"},
		{"alias step", "version: 1\nworktrees:\n  setup:\n    - &step {run: echo hi}\n    - *step", "must be a mapping"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseConfig(strings.NewReader(test.yaml))
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
}
