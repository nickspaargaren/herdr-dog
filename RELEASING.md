# Releasing Herdr Dog

Users install prebuilt binaries. Go is needed only for development and release
builds. GitHub Actions builds and publishes releases; no local cross-compilation
or manual asset upload is required.

## Versioning

- Use stable semantic versions: `MAJOR.MINOR.PATCH`, tagged `vMAJOR.MINOR.PATCH`.
- `version` in `herdr-plugin.toml` is the **single source of truth**. Both the
  installer and build script read it. The tag must match it exactly.
- Plugin release versions are separate from YAML's `version: 1`. Bumping the
  plugin does not change the project configuration format.
- Every released version needs its own binaries and `SHA256SUMS`. The installer
  selects the manifest's exact version, not GitHub's latest release.
- Published versions and tags are immutable. Ship fixes as a new version.

## Version 0.2.0

This release adds main-checkout configuration fallback, supporting untracked and
gitignored `.herdr/worktrees.yml` files. The YAML format remains `version: 1`.
After merging and passing CI, publish `v0.2.0` using the steps below. Until its
assets are published, developers can build and link source, and users can pin
the previously published `v0.1.0` release.

## Release a new version

1. Change `version` in `herdr-plugin.toml` (for example, `0.2.0`). Update relevant
   docs and `min_herdr_version` if the required Herdr APIs changed.
2. Open a PR and let CI pass: tests/vet/build on macOS and Linux, plus the four
   cross-compiled release binaries and their checksums. Merge the PR.
3. From an up-to-date, clean `main` checkout, create and push an annotated tag
   matching the manifest:

   ```sh
   git switch main
   git pull --ff-only
   git tag -a v0.2.0 -m "Herdr Dog v0.2.0"
   git push origin v0.2.0
   ```

4. Watch **Release** in the repository's Actions tab. The workflow validates the
   tag against the manifest, runs tests and vet, and builds with `CGO_ENABLED=0`:

   ```text
   herdr-dog_darwin_amd64
   herdr-dog_darwin_arm64
   herdr-dog_linux_amd64
   herdr-dog_linux_arm64
   SHA256SUMS
   ```

   Assets are raw binaries, with YAML support compiled in. The workflow creates a
   draft release, uploads all five assets, then publishes it with generated notes.
   Its built-in GitHub token needs `contents: write`, declared in the workflow.
   There are no additional secrets to configure.
5. Verify the published assets and test a pinned installation on a machine with
   no Go toolchain:

   ```sh
   gh release view v0.2.0
   herdr plugin install nickspaargaren/herdr-dog --ref v0.2.0
   ```

   For a locally linked development plugin, use a separate verification machine
   or unregister the local link with `herdr plugin unlink herdr-dog` first.
   Herdr refuses a managed installation over an existing local link.

Between merging a version bump and publishing its assets, the unpinned default
branch installation refers to an unavailable release. Tag and publish promptly;
existing installations continue to work. Users needing a predictable installation
can pin the previous published tag until the new release finishes.

## Build release assets locally

For packaging verification without publishing:

```sh
go test ./...
go vet ./...
/bin/sh scripts/build-release v0.2.0
```

Use the version currently in the manifest, or omit the argument to select it.
The script writes the four binaries and `SHA256SUMS` under ignored `dist/`.
Verify checksums from that directory with `sha256sum --check SHA256SUMS` on Linux
or `shasum -a 256 --check SHA256SUMS` on macOS. Building assets does not publish
anything, modify the manifest, or install a plugin.

## Failures and updates

- If the workflow fails, inspect its logs and rerun the failed jobs after fixing
  a transient cause. An existing **draft** release can be safely resumed: assets
  are replaced while still unpublished, then the release is published.
- If source needs fixing, create a new version/commit/tag rather than moving the
  failed tag. A tag/manifest mismatch fails before any binaries are built.
- Once published, a rerun refuses to overwrite the release. Do not replace
  published binaries or checksums; consumers pinning that version expect stable
  bytes. Make a new patch release instead.
- Users update a managed plugin by reinstalling it with `herdr plugin install
  nickspaargaren/herdr-dog` (optionally with `--ref`); Herdr has no separate
  `plugin update` command. Linked dotfiles checkouts are updated with Git followed
  by `/bin/sh scripts/install-binary`.
