# SelfStack Self-Update Design

## Problem

Users have no way to update the SelfStack binary. The version is hardcoded and there's no release infrastructure. Target audience includes non-technical users and AI agents, so the solution must require zero developer tooling.

## Approach

GoReleaser + GitHub Actions for releases. A `selfstack update` CLI command for updates. A curl installer script for first-time installs.

## Version Management

- Add `var Version = "dev"` in `cmd/root.go`
- GoReleaser sets it at build time via `-ldflags "-X github.com/selfstack/selfstack/cmd.Version=X.Y.Z"`
- `/api/status` endpoint and `selfstack status` CLI read this variable
- Local dev builds show `"dev"`, release builds show the semver tag

## Release Pipeline

**Trigger**: Push a git tag (`v0.2.0`) to trigger the release.

**GoReleaser** (`.goreleaser.yml`):
- Builds 4 binaries: `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`
- Creates tar.gz archives with checksums
- Publishes to GitHub Releases

**GitHub Actions** (`.github/workflows/release.yml`):
- Triggers on `v*` tags
- Uses `goreleaser-action` to build and publish

**Release process**: `git tag v0.2.0 && git push --tags`

## `selfstack update` Command

1. Hits `https://api.github.com/repos/Avanderheyde/self-stack/releases/latest`
2. Compares release tag against compiled-in `cmd.Version`
3. If newer: downloads the correct binary for `runtime.GOOS`/`runtime.GOARCH`
4. Writes to temp file, renames over the current executable
5. Prints "Updated selfstack from vX to vY"
6. If current: prints "Already up to date (vX)"

**Startup nudge on `selfstack serve`**:
- Non-blocking goroutine check (3 second timeout)
- If newer version exists, prints: `A new version of SelfStack is available (vX.Y.Z). Run "selfstack update" to upgrade.`
- Never blocks server startup

## Install Script (`scripts/install.sh`)

1. Detects OS (`uname -s`) and architecture (`uname -m`)
2. Fetches latest release tag from GitHub API
3. Downloads correct archive from GitHub Releases
4. Extracts `selfstack` binary to `/usr/local/bin` (or `~/.local/bin` without sudo)
5. Prints version confirmation

Usage:
```
curl -fsSL https://raw.githubusercontent.com/Avanderheyde/self-stack/master/scripts/install.sh | sh
```

## Platforms

- macOS arm64 (Apple Silicon)
- macOS amd64 (Intel)
- Linux arm64
- Linux amd64

## Files to Create/Modify

- `cmd/root.go` — add `Version` variable
- `cmd/update.go` — new `selfstack update` command
- `cmd/serve.go` — add startup version check nudge
- `cmd/status.go` — use `cmd.Version` instead of API call for version
- `internal/api/apps.go` — use `cmd.Version` in status endpoint
- `.goreleaser.yml` — release config
- `.github/workflows/release.yml` — CI pipeline
- `scripts/install.sh` — installer script
