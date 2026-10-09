package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const configPath = ".herdr/worktrees.yml"

type config struct {
	Steps []step
}

type step struct {
	Name string
	Run  string
	Env  map[string]string
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func loadConfig(worktree string) (*config, error) {
	path, err := filepath.EvalSymlinks(filepath.Join(worktree, configPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", configPath, err)
	}
	rel, err := filepath.Rel(worktree, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s must resolve inside the new worktree", configPath)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", configPath, err)
	}
	defer file.Close()
	return parseConfig(file)
}

// Inspect YAML nodes so scalars are not silently coerced into strings or ints.
// This is schema validation, not a YAML parser: yaml/v3 handles YAML syntax.
func parseConfig(reader io.Reader) (*config, error) {
	invalid := func(err error) (*config, error) {
		return nil, fmt.Errorf("invalid %s: %w", configPath, err)
	}
	decoder := yaml.NewDecoder(reader)
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return invalid(err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("only one YAML document is allowed")
		}
		return invalid(err)
	}
	root, err := mapping(document.Content[0], "config", "version", "worktrees")
	if err != nil {
		return invalid(err)
	}
	version := root["version"]
	if version == nil || version.Kind != yaml.ScalarNode || version.Tag != "!!int" {
		return invalid(fmt.Errorf("version is required and must be an integer"))
	}
	var number int
	if err := version.Decode(&number); err != nil {
		return invalid(fmt.Errorf("version must be an integer"))
	}
	if number != 1 {
		return nil, fmt.Errorf("unsupported config version: %d", number)
	}
	worktrees, err := mapping(root["worktrees"], "worktrees", "setup")
	if err != nil {
		return invalid(err)
	}
	setup := worktrees["setup"]
	if setup == nil || setup.Kind != yaml.SequenceNode {
		return invalid(fmt.Errorf("worktrees.setup is required and must be a sequence"))
	}
	result := &config{}
	for i, node := range setup.Content {
		label := fmt.Sprintf("setup[%d]", i)
		fields, err := mapping(node, label, "name", "run", "env")
		if err != nil {
			return invalid(err)
		}
		run := fields["run"]
		if run == nil || (run.Kind == yaml.ScalarNode && run.Tag == "!!str" && strings.TrimSpace(run.Value) == "") {
			return invalid(fmt.Errorf("%s.run is required", label))
		}
		command, err := stringValue(run, label+".run")
		if err != nil {
			return invalid(err)
		}
		s := step{Run: command, Env: map[string]string{}}
		if name := fields["name"]; name != nil {
			s.Name, err = stringValue(name, label+".name")
			if err != nil {
				return invalid(err)
			}
		}
		if env := fields["env"]; env != nil {
			values, err := mapping(env, label+".env")
			if err != nil {
				return invalid(err)
			}
			for key, value := range values {
				if !envName.MatchString(key) {
					return invalid(fmt.Errorf("%s.env: invalid variable name %q", label, key))
				}
				s.Env[key], err = stringValue(value, label+".env."+key)
				if err != nil {
					return invalid(err)
				}
			}
		}
		result.Steps = append(result.Steps, s)
	}
	return result, nil
}

func mapping(node *yaml.Node, label string, allowed ...string) (map[string]*yaml.Node, error) {
	if node == nil || node.Kind != yaml.MappingNode || node.Tag != "!!map" {
		return nil, fmt.Errorf("%s is required and must be a mapping", label)
	}
	result := map[string]*yaml.Node{}
	for i := 0; i < len(node.Content); i += 2 {
		key, err := stringValue(node.Content[i], label+" key")
		if err != nil {
			return nil, err
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("%s: duplicate key %q", label, key)
		}
		if len(allowed) > 0 {
			known := false
			for _, field := range allowed {
				known = known || key == field
			}
			if !known {
				return nil, fmt.Errorf("%s: unknown field %q", label, key)
			}
		}
		result[key] = node.Content[i+1]
	}
	return result, nil
}

func stringValue(node *yaml.Node, label string) (string, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" || strings.ContainsRune(node.Value, '\x00') {
		return "", fmt.Errorf("%s must be a string without NUL characters", label)
	}
	return node.Value, nil
}
