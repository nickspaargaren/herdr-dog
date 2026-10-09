package main

import (
	"fmt"
	"os"

	"github.com/nickspaargaren/herdr-dog/internal/setup"
)

func main() {
	err := setup.HandleEvent(os.Getenv("HERDR_PLUGIN_EVENT_JSON"), os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "worktree-setup:", err)
		os.Exit(1)
	}
}
