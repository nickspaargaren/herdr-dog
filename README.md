# Herdr Dog 🐕🐑

[Herdr](https://herdr.dev) manages the herd. Dog gets each worktree ready to work.

**Run project-defined setup commands after Herdr creates a worktree.**
Install Dog once, globally. Each project opts in by committing
`.herdr/worktrees.yml`. The plugin contains no project-specific configuration.

```text
Herdr creates a Git worktree
  → worktree.created
  → Herdr Dog reads .herdr/worktrees.yml
  → worktrees.setup runs inside the new checkout
```

## Installation

Requires **Herdr 0.9.3+**, **Go 1.22+** to build, **Git 2.31+**, and **macOS or
Linux** with `/bin/sh`. Project commands also require their own tools, such as
`pnpm`. Go is not required at runtime after the binary has been built.

```sh
herdr plugin install nickspaargaren/herdr-dog
```

Herdr clones the repository and runs the manifest's Go build command before
registering the plugin. Installation is global to your user. Add `--yes` for a
noninteractive installation, or `--ref <tag-or-commit>` to pin a revision.

### Local development and dotfiles

Clone anywhere, build in that checkout, then link its absolute path:

```sh
git clone https://github.com/nickspaargaren/herdr-dog.git "$HOME/.local/src/herdr-dog"
cd "$HOME/.local/src/herdr-dog"
go build -o bin/herdr-dog ./cmd/herdr-dog
herdr plugin link "$HOME/.local/src/herdr-dog"
```

Herdr's [`plugin link`](https://herdr.dev/docs/plugins/#install-and-link) uses the
local directory and **does not run build commands**. Rebuild after source changes.
The plugin works independently of where it is cloned or where Herdr is launched.

To unregister a local link: `herdr plugin unlink herdr-dog`.
To remove a managed installation: `herdr plugin uninstall herdr-dog`.

## Quick start

Commit this as `.herdr/worktrees.yml` in your project:

```yaml
version: 1

worktrees:
  setup:
    - run: cp "$HERDR_MAIN_WORKTREE/.env" .env
```

Create a worktree through Herdr. Dog copies `.env` from the main checkout into
the new checkout. The main checkout must already have that file.

For more complex projects, delegate to a project-owned script:

```yaml
version: 1

worktrees:
  setup:
    - run: ./scripts/worktree-setup
```

Commit the script too, with executable permission and an appropriate shebang.

## Configuration reference

```yaml
version: 1

worktrees:
  setup:
    - run: cp "$HERDR_MAIN_WORKTREE/.env" .env

    - name: Install dependencies
      run: pnpm install

    - name: Seed database
      run: pnpm db:seed
      env:
        NODE_ENV: development
```

| Field | Required | Meaning |
| --- | --- | --- |
| `version` | Yes | Integer `1`; other versions are rejected. |
| `worktrees.setup` | Yes | Ordered list of steps; `[]` is a valid no-op. |
| `run` | Each step | Nonempty shell command string. |
| `name` | No | Step label used in progress and failure messages. Defaults to `run`. |
| `env` | No | Environment variable names mapped to string values, for this step only. Quote numbers and booleans. |

The entire file is validated before any command runs. Unknown fields, duplicate
keys, incorrect types, multiple YAML documents, aliases, and merge keys are
rejected. Error step indexes are zero-based (`setup[1]` is the second step).

Configuration is read **from the new checkout**, so it follows the checked-out
branch. Dog does not search parent directories or fall back to another checkout.
Configuration symlinks must resolve inside the new checkout.

This intentionally small v1 format has no conditions, retries, parallel steps,
dependencies, templating, teardown, or other lifecycle hooks. Put project logic
in project scripts.

### Shell and execution

Each `run` executes as `/bin/sh -c <run>`, sequentially, with the new worktree as
its working directory. Commands are passed to the shell unchanged; quote path
variables as shown above. This is a **noninteractive POSIX shell**, not your login
shell: stdin is `/dev/null`, and shell aliases and Bash/Zsh-specific syntax should
not be assumed. A project script may select another shell with its shebang.

Every step starts with a fresh environment inherited from the plugin process,
plus its `env` and the variables below. Changes to environment or working
directory inside a step do not carry over to later steps. Filesystem changes do.

## Available environment variables

Dog provides these to each setup command; they are **not assumed Herdr variables**:

| Variable | Value |
| --- | --- |
| `HERDR_WORKTREE` | Absolute, symlink-resolved path to the newly created checkout. |
| `HERDR_MAIN_WORKTREE` | Absolute, symlink-resolved path to Git's primary/main checkout. |
| `HERDR_BRANCH` | New worktree's branch name; empty for detached HEAD. |

These values override inherited or step-defined values of the same names.
The main checkout means Git's original checkout, regardless of its branch name.
Repositories backed by a bare repository have no primary checkout and are not
supported.

## Examples

Copy an example's `worktrees.yml` to your project's `.herdr/worktrees.yml`:

- [Minimal](examples/minimal/worktrees.yml): copy `.env`.
- [Node](examples/node/worktrees.yml): copy `.env`, then install with `pnpm`.
- [PostgreSQL](examples/postgres/worktrees.yml): copy `.env`, install dependencies,
  then call a [project-owned database script](examples/postgres/scripts/worktree-db-setup).

For PostgreSQL, also copy the script to `scripts/worktree-db-setup` and adapt it.
It illustrates deriving a safe database name from the branch/check-out path,
adding a path checksum, creating the database, then loading migrations and
fixtures. It requires your local PostgreSQL connection settings and your own SQL
files. Adapt application configuration to use that database; the example records
its name in `.worktree-database`. Ignore that generated file in your project.

**Dog executes setup steps. Your project knows how to configure its database.**

## Errors and failure behavior

A checkout without `.herdr/worktrees.yml` exits successfully without output.
Otherwise Dog writes one short progress line per step and forwards each command's
stdout/stderr. A nonzero shell exit stops execution immediately. Dog itself exits
nonzero on configuration, discovery, or command failure.

```text
worktree-setup: unsupported config version: 2
worktree-setup: invalid .herdr/worktrees.yml: setup[1].run is required
worktree-setup: step "Seed database" failed with exit code 1
```

Herdr runs event hooks asynchronously after creating the worktree and opening its
workspace. Setup is **not a readiness barrier**: it does not delay the workspace,
roll back creation on failure, or automatically retry. Separate worktree events
can run concurrently; steps within one event always run sequentially.

Herdr captures plugin output in its command logs, currently retaining up to
64 KiB per stream. Inspect logs with:

```sh
herdr plugin log list --plugin herdr-dog
```

## Security and trust

**Creating a worktree for a repository containing `.herdr/worktrees.yml` can
execute commands defined by that repository.** Treat the file and scripts it
calls as executable project configuration. They run as your user, inherit your
environment, and have normal access to your files and tools. Review repositories
and the branches you create worktrees from before using them.

Dog does not sandbox commands or manage project trust. It does not load executable
configuration from outside the new checkout.

## Development and testing

Go is a good fit here: a standalone binary, standard-library Git/subprocess
handling, and one runtime dependency,
[`go.yaml.in/yaml/v3`](https://github.com/yaml/go-yaml), for YAML parsing.

```sh
go test ./...
go vet ./...
go build -o bin/herdr-dog ./cmd/herdr-dog
herdr plugin link /absolute/path/to/herdr-dog
```

Tests use temporary Git repositories and real `/bin/sh` subprocesses; a running
Herdr instance is not required. They cover configuration validation, event parsing,
checkout discovery, command order and failure, environment isolation, output,
unusual paths/branch names, detached HEAD, and configuration path containment.

The integration was verified against [Herdr's plugin docs](https://herdr.dev/docs/plugins/)
and the v0.9.3 sources for [event serialization](https://github.com/herdrdev/herdr/blob/v0.9.3/src/api/schema/events.rs),
[worktree fields](https://github.com/herdrdev/herdr/blob/v0.9.3/src/api/schema/worktrees.rs),
and [hook execution](https://github.com/herdrdev/herdr/blob/v0.9.3/src/app/api/plugins/runtime.rs).
Herdr supplies `HERDR_PLUGIN_EVENT_JSON` with `event: "worktree_created"` and
`data.type: "worktree_created"`, even though the manifest hook uses
`worktree.created`. Dog reads `data.worktree.path` and its optional branch, then
uses `git worktree list --porcelain -z` to discover the main checkout. When the
branch is absent, Git's `branch --show-current` supplies it.

Licensed under [MIT](LICENSE).
