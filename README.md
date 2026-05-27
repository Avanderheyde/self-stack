# SelfStack

> **One command deploys your AI-generated personal apps to a VPS you own, behind Tailscale.**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

You vibe-code a personal expense tracker with Claude. It works on `localhost`. Then what?

```bash
selfstack deploy
```

A minute later (design target: <60 s on a warm-cache redeploy), the app is live on a $4/mo Hetzner VPS, reachable only by your own devices over Tailscale. No public URL. No DNS. No nginx. No `docker compose` muscle memory. You own the box, the data, the binary.

That's the whole pitch. The rest of this README is how to run it.

> Timing numbers in this README are **design targets** from `docs/plans/2026-03-28-deploy-command-design.md` — they haven't been measured end-to-end yet. The first three live runs will land in `docs/mvp-demo.md`.

---

## Why this exists

AI made building personal apps free. Hosting them didn't get easier. The options are:

- **Vercel / Lovable / Bolt** — built for public apps on their cloud. You don't own the data; you pay per app.
- **DIY VPS** — SSH, Docker, nginx, Caddy, certbot, systemd. 1–4 hours per app the first time. Doesn't scale to ten.
- **Localhost forever** — the app dies when your laptop sleeps.

SelfStack closes the gap for the **technical-but-lazy** developer: someone who *can* configure Docker but would rather not, for the tenth personal app this month.

Full design context: [`docs/plans/2026-02-18-selfstack-design.md`](docs/plans/2026-02-18-selfstack-design.md) and [`docs/plans/2026-03-28-deploy-command-design.md`](docs/plans/2026-03-28-deploy-command-design.md).

---

## 30-second demo

```bash
# 1. Provision a VPS (one-time; design target ~2 minutes)
selfstack cloud setup
  # Prompts: Hetzner API token, optional Tailscale auth key
  # Creates a cax11 VPS in Falkenstein, installs Docker + Tailscale + selfstack
  # ~$4/mo

# 2. Deploy whatever you just built with Claude (in any project dir)
cd ~/code/my-expense-tracker
selfstack deploy
  # Detects project (Dockerfile / compose / Node / Go / Python)
  # Auto-generates Docker config if missing
  # rsyncs to the VPS, builds, starts, health-checks
  # Configures `tailscale serve` for the chosen port
  # → https://your-vps.tail-scale.ts.net:10001
```

Open the URL from your phone. Done.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Avanderheyde/self-stack/main/scripts/install.sh | sh
```

For forks or pinned releases:

```bash
SELFSTACK_REPO=not0xjarvis/self-stack SELFSTACK_VERSION=v0.1.0 \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/Avanderheyde/self-stack/main/scripts/install.sh)"
```

For unreleased review branches, point cloud setup/init at the branch installer:

```bash
selfstack cloud setup --install-url https://raw.githubusercontent.com/<owner>/self-stack/<branch>/scripts/install.sh
selfstack init --install-url https://raw.githubusercontent.com/<owner>/self-stack/<branch>/scripts/install.sh
```

Or build from source:

```bash
git clone https://github.com/Avanderheyde/self-stack
cd self-stack
make build
./selfstack --help
```

> Until the first GitHub release is published, the curl-pipe installer above prints a 3-option remediation message (build from source, set `SELFSTACK_REPO`, or pin `SELFSTACK_VERSION`). Pre-launch checklist lives in [`docs/runbooks/oss-launch.md`](docs/runbooks/oss-launch.md); the measured-timing runbook lives in [`docs/runbooks/live-demo.md`](docs/runbooks/live-demo.md).

---

## Commands

### Cloud (managed VPS)

| Command | What it does |
|---|---|
| `selfstack cloud setup` | Provisions a Hetzner cax11 VPS, installs Docker + Tailscale + selfstack as a systemd service. |
| `selfstack cloud status` | Lists your selfstack-managed Hetzner servers. |
| `selfstack cloud destroy` | Tears the VPS down. Requires typing `destroy` to confirm. |

### Deploy (the moment of creation)

| Command | What it does |
|---|---|
| `selfstack deploy [--name X] [--env-file .env]` | Detects project type, generates Dockerfile/compose if needed, rsyncs to VPS, builds, starts, configures Tailscale Serve. Streams progress as SSE. |
| `selfstack undeploy <app>` | Stops the app and removes the Tailscale Serve route. |

Project auto-detection (priority order): `docker-compose.yml` → `Dockerfile` → `package.json` → `go.mod` → `requirements.txt` / `pyproject.toml`.

### Bring-your-own VPS

| Command | What it does |
|---|---|
| `selfstack init` | Points the CLI at an existing VPS via SSH; verifies Docker is installed. |

### App lifecycle (works on local or remote control plane)

| Command | What it does |
|---|---|
| `selfstack list` | Lists installed apps + status. |
| `selfstack start <app>` / `stop <app>` | Stop/start a container. |
| `selfstack update <app>` | Pulls latest, rebuilds, preserves volumes. |
| `selfstack remove <app>` | Removes an app. |
| `selfstack install <app> [--local path] [--seed-data dir]` | Install from the curated registry, or from a local source tree (optionally seeding the data volume). |
| `selfstack search <query>` | Search the registry. |
| `selfstack status` | Server health + app count. |

### Control plane

| Command | What it does |
|---|---|
| `selfstack serve` | The HTTP API + dashboard (port 8080) + reverse proxy (port 8081). Runs as a systemd unit on cloud-provisioned VPSes. |

---

## How it works

```
┌─────────────────────┐         ┌────────────────────────────────────┐
│  Your laptop        │         │  Hetzner cax11 VPS (~$4/mo)        │
│                     │         │                                    │
│  $ selfstack deploy │  rsync  │  ~/.selfstack/apps/<name>/         │
│  ────────────────▶  │ ──────▶ │                                    │
│                     │         │  ┌──────────────────────────────┐  │
│                     │  POST   │  │  selfstack serve (systemd)   │  │
│                     │ ──────▶ │  │   ├─ /api/apps/deploy (SSE)  │  │
│                     │         │  │   ├─ Docker SDK (build/run)  │  │
│                     │         │  │   ├─ port allocator          │  │
│                     │         │  │   └─ reverse proxy           │  │
│                     │         │  └────────────┬─────────────────┘  │
│                     │         │               │                    │
│                     │         │  ┌────────────▼─────────────────┐  │
│                     │         │  │  tailscale serve --bg <port> │  │
│                     │         │  └────────────┬─────────────────┘  │
└─────────────────────┘         └───────────────┼────────────────────┘
                                                │
                                  ┌─────────────▼──────────────┐
                                  │  Your phone (Tailscale)    │
                                  │  https://<vps>.ts.net:PORT │
                                  └────────────────────────────┘
```

Single binary, Go + embedded React dashboard, SQLite for state. Docker for runtime. Tailscale for access. No public ports, no domains, no auth pages.

---

## What's actually shipped (V1)

- ✅ `selfstack cloud setup` — Hetzner provisioning (cax11, fsn1, ubuntu-24.04), auto SSH key upload, cloud-init installs Docker + Tailscale + selfstack
- ✅ `selfstack deploy` — auto-detect, generate Dockerfile/compose, rsync, build, run, health-check, Tailscale Serve
- ✅ `selfstack undeploy` — Tailscale Serve cleanup
- ✅ App lifecycle: install / list / start / stop / update / remove (via embedded REST API)
- ✅ Local install from source: `selfstack install <name> --local <path> [--seed-data <dir>]`
- ✅ Registry client + curated app catalog
- ✅ Embedded React dashboard with mobile layout
- ✅ Self-update (`selfstack update`) + GitHub Actions release pipeline via goreleaser
- ✅ Docker integration test for sample app (`test/e2e_test.go`, build tag `integration`)

### Not yet shipped (TODO)

- ⏳ `--tunnel cloudflare` flag (referenced in design, not wired into `cmd/deploy.go`)
- ⏳ `selfstack logs <app>` CLI command (API endpoint exists; CLI wrapper does not)
- ⏳ MCP server / Claude Code slash-command integration
- ⏳ Device-trust auth middleware (initialized in `serve.go` but not wired into the request path)
- ⏳ Multi-user / Tailscale ACL tier
- ⏳ Automated backups

---

## Project layout

```
cmd/                      cobra commands (deploy, cloud, init, install, …)
internal/
  api/                    REST API + SSE deploy/install streams
  app/                    install/deploy service layer
  cloud/                  Hetzner client + cloud-init setup script
  container/              Docker SDK, port allocator, health checks
  dashboard/              embedded React build
  detect/                 project-type auto-detection + Dockerfile/compose generation
  proxy/                  reverse proxy
  tailscale/              `tailscale serve` wrapper (local + remote via SSH)
  …
dashboard/                React/TS source for the embedded dashboard
docs/plans/               design docs (read these for the "why")
test/                     integration tests + sample app
scripts/install.sh        curl-pipe installer
```

---

## Contributing And Security

SelfStack is Apache-2.0 licensed. See [CONTRIBUTING.md](CONTRIBUTING.md) for
development workflow and [SECURITY.md](SECURITY.md) for private security
reporting.

Before publishing or cutting release branches, run:

```bash
scripts/secret-audit.sh
```

---

## Status

Pre-launch. Founder dogfoods it for personal apps; no external users yet. The Paradigm MVP demo (`docs/mvp-demo.md`) is the next forcing function.
