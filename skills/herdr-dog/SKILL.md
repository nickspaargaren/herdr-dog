---
name: herdr-dog
description: Set up Herdr Dog for a project and troubleshoot worktree setup defined in .herdr/worktrees.yml. Use when asked to setup Herdr Dog, write setup steps, inspect setup failures, or recover a local development environment prepared by Herdr Dog.
license: MIT
compatibility: Requires Git 2.36+, macOS or Linux, and shell command execution. Automatic setup requires Herdr 0.9.3+.
metadata:
  author: nickspaargaren
---

# Herdr Dog

Herdr Dog runs project setup commands after Herdr creates a Git worktree.
The project chooses dependency installation, services, databases, migrations,
seeding, terminal layout, and application readiness checks. Dog executes the steps.

## Set up a project

When asked to "setup Herdr Dog":

1. Inspect project instructions, manifests, lockfiles, and development docs.
2. Check existing Dog configuration in the current and main checkouts.
3. Discover the project's install, service, migration, seed, and startup commands.
4. Walk through the findings with the user and confirm which setup steps they want.
5. Create or update `.herdr/worktrees.yml` with small, named steps reflecting those choices.
6. Verify supported commands and report what was actually tested.

## Inspect the current checkout

1. Read the project's `.herdr/worktrees.yml` and the scripts or task
   definitions referenced by its commands.
2. Read configuration from the new checkout first. If the file is absent, inspect
   `.herdr/worktrees.yml` in Git's primary/main checkout instead. Dog supports
   untracked or gitignored configuration there. If neither checkout has the file,
   setup is a silent no-op.
3. Inspect existing setup output and development processes before rerunning
   anything. Worktree creation does not mean setup or the application is ready.

The installed plugin version determines supported behavior. This skill describes
the versioned configuration and execution model. Check the installed version with
`herdr plugin list --plugin herdr-dog --json` when diagnosing a mismatch.

## Write small, ordered setup steps

Keep project logic in `.herdr/worktrees.yml`, using named `run` steps. Use the
project's actual commands rather than assuming a package manager or database.

```yaml
version: 1
worktrees:
  setup:
    - name: Copy environment variables
      run: cp "$HERDR_MAIN_WORKTREE/.env" .env
    - name: Install dependencies
      run: npm install
```

- Steps run sequentially in the new checkout's root. The first nonzero exit stops
  the remaining steps. The whole configuration is validated before execution.
- Each step runs in a fresh, noninteractive `/bin/sh` with stdin set to `/dev/null`.
  Shell aliases, prompts, and Bash/Zsh-only syntax are not available by default.
- `export` and `cd` changes do not carry into later steps. Filesystem changes do.
  Persist generated project settings in a project file and load them
  explicitly in subsequent commands; Dog does not automatically load `.env`.
- A multiline command is one shell invocation. Use `set -eu` or explicit command
  chaining when an intermediate failure must stop that step.
- Per-step `env` values override inherited environment variables. Dog's checkout
  context overrides both; quote path variables when using them in shell commands.
- Keep reusable details in project scripts if a step becomes difficult to read.

See [the configuration reference](references/configuration.md) for supported fields,
context variables, and execution boundaries.

## Inspect failures and recover

When running inside Herdr, inspect the plugin's command logs:

```sh
herdr plugin log list --plugin herdr-dog
```

Identify the invocation for the affected checkout and read its output. If output
refers to an application pane, inspect that pane as well. Use the separate Herdr
skill for pane discovery and control, with explicit pane IDs and preserved focus.

1. Identify the failed step and whether earlier steps changed files or resources.
2. Check the command's prerequisites, working directory, environment, and output.
3. Repair the cause, then run only the required project command in the affected
   checkout, with the context and environment it actually needs.
4. Continue later steps in order only after their prerequisites succeed.

The versioned executable is an event handler: there are no manual `setup`, `status`,
selected-step, or dry-run CLI commands. Do not invent those commands or fabricate
event JSON to replay the whole workflow. If running a command manually, Dog's
context variables are not automatically supplied by an ordinary shell.

Prefer idempotent setup that reuses existing resources. Do not create duplicate
application servers, reset databases, or replay fixture seeding merely to restart
an application. Inspect what a seed or reset command changes before choosing it
for recovery.

## Verify readiness

Herdr launches the event hook asynchronously. Setup does not block workspace
creation, roll back a worktree on failure, or retry automatically. Different
worktrees can set up concurrently; project shared resources need their own
coordination.

Keep finite preparation steps ordered before starting the application. A
long-running server directly in a `run` step prevents later steps from executing;
projects can launch it in a Herdr pane instead. Sending a command to a pane proves
submission, not successful startup.

Use the project's readiness check, such as a bounded HTTP probe, before reporting
the environment ready. Report the verified URL and relevant pane or failed step.
Keep connection credentials out of status summaries.
