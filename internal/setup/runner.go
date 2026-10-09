package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func runSetup(config config, checkout checkout, stdout, stderr io.Writer) error {
	for _, step := range config.Steps {
		label := step.Name
		if label == "" {
			label = step.Run
		}
		fmt.Fprintf(stderr, "worktree-setup: running %q\n", label)
		cmd := exec.Command("/bin/sh", "-c", step.Run)
		cmd.Dir = checkout.Worktree
		cmd.Env = append([]string{}, os.Environ()...)
		for key, value := range step.Env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
		// These describe the checkout; a step cannot override their meaning.
		cmd.Env = append(cmd.Env,
			"HERDR_WORKTREE="+checkout.Worktree,
			"HERDR_MAIN_WORKTREE="+checkout.Main,
			"HERDR_BRANCH="+checkout.Branch,
		)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		// Setup is noninteractive. Unset Stdin makes Go attach /dev/null.
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				if exit.ExitCode() < 0 {
					return fmt.Errorf("step %q terminated by a signal", label)
				}
				return fmt.Errorf("step %q failed with exit code %d", label, exit.ExitCode())
			}
			return fmt.Errorf("cannot execute step %q: %w", label, err)
		}
	}
	return nil
}
