# Herdr Dog configuration and execution

This reference describes the v0.4.0 implementation. It is bundled with the skill
so agents can read it independently of the Dog source repository.

## Configuration location

Dog reads `.herdr/worktrees.yml` from the newly created checkout first. If the file
is absent, it reads the same path in Git's primary/main checkout, supporting
untracked and gitignored local configuration.

The new checkout's configuration takes precedence, even with an empty setup list.
Invalid, unreadable, or dangling-symlink configuration is an error and does not
trigger fallback. Dog does not merge configurations, copy the fallback file, or
search parent directories. Configuration symlinks must resolve inside the checkout
supplying the file. Commands always execute in the new checkout, regardless of
which checkout supplies configuration; referenced project scripts must exist there.

When neither checkout has the file, setup is a silent no-op. Bare repositories are
unsupported because discovery requires a primary/main checkout.

## Supported fields

```yaml
version: 1
worktrees:
  setup:
    - name: Copy environment
      run: cp "$HERDR_MAIN_WORKTREE/.env" .env
    - name: Install dependencies
      run: npm install
      env:
        NODE_ENV: development
```

| Field | Meaning |
| --- | --- |
| `version` | Required integer `1`. |
| `worktrees.setup` | Required ordered sequence. `[]` is a valid no-op. |
| `run` | Required nonempty shell command string for each step. |
| `name` | Optional label in progress and errors; defaults to the command. |
| `env` | Optional mapping of variable names to string values for this step. |

Quote numbers and booleans in `env` so YAML treats them as strings. Unknown fields,
duplicate mapping keys, incorrect types, multiple YAML documents, aliases, and
merge keys are rejected before any command executes. Error indexes such as
`setup[1]` are zero-based.

There is no conditional, retry, parallel-step, dependency, templating, teardown, or
other lifecycle-hook syntax. Express those needs in project commands, rather than
adding unsupported keys.

## Environment and checkout context

Each command inherits the plugin process environment. Its `env` mapping overrides
inherited values. Dog then supplies these reserved context variables:

| Variable | Value |
| --- | --- |
| `HERDR_WORKTREE` | Absolute, symlink-resolved new checkout path. |
| `HERDR_MAIN_WORKTREE` | Absolute, symlink-resolved primary checkout path. |
| `HERDR_BRANCH` | New checkout branch, or an empty string for detached HEAD. |

The primary checkout is Git's original checkout, regardless of its current branch.
Step configuration cannot override the meaning of these variables.

Every command starts in the new checkout root and runs through `/bin/sh -c` with
stdin connected to `/dev/null`. Changing directory or exporting a variable affects
only the current step. Generated files remain available to subsequent steps.

Dog does not interpret shell templates or source environment files. An application
may load its own `.env`; that is application behavior, not Dog behavior.

## Failure and startup boundaries

- Steps execute in order and stop at the first failing shell command invocation.
- Within a multiline `run`, the shell's exit status determines success. Use
  `set -eu` or appropriate chaining when earlier command failures must propagate.
- stdout and stderr are forwarded to Herdr's plugin command logs.
- Herdr currently retains up to 64 KiB per output stream in those logs.
- The hook runs asynchronously after worktree/workspace creation.
- Failed setup does not remove the checkout or undo completed steps.
- There is no automatic retry or persistent setup-status API.
- Separate worktree events may run concurrently.

The plugin's executable expects `HERDR_PLUGIN_EVENT_JSON` for a
`worktree_created` event. It is not a general-purpose manual task runner.

Projects own long-running service supervision, shared-resource locking, resource
cleanup, port selection, and readiness checks. Merely launching a server in a
terminal does not verify that it is serving requests.
