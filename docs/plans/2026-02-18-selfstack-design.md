# SelfStack Design Document

**Date:** 2026-02-18 | **Status:** Approved | **Version:** 1.0

## Overview

SelfStack is an open-source personal app server that lets users install, run, and securely access self-hosted web apps from anywhere. Think Homebrew meets Docker meets a personal App Store.

**Core problem:** People want to use apps (finance trackers, note-taking, recipe managers) without giving their data to third-party companies. The cost of development is now near zero thanks to AI-assisted coding, but self-hosting remains too complex for most people.

**Solution:** A single binary that provides a click-to-install experience for self-hosted web apps, with automatic Docker orchestration, networking, and secure remote access.

## Key Principles

1. **Your data, your machine** - nothing leaves the user's hardware unless they want it to
2. **One-click simple** - no Docker knowledge, no terminal, no port forwarding required
3. **Agent-first API** - every GUI action is an API call; AI agents are first-class citizens
4. **Convention over configuration** - minimal manifest spec, sensible defaults everywhere

## Architecture

### Single Binary (Go + Embedded React Dashboard)

SelfStack compiles to a single Go binary (~25MB) with a React/TypeScript dashboard embedded at build time. Users download one file and run it.

**Why Go:** Static compilation (zero dependencies), first-class Docker SDK, Caddy (reverse proxy) is Go and can be embedded as a library, excellent cross-compilation, dominant language for infrastructure tools (Docker, Kubernetes, Terraform, Tailscale all use Go).

**Why React dashboard embedded:** Professional, polished UI that any web developer can contribute to. Compiled to static files and embedded in the Go binary via `go:embed`. Users still download and run one file.

### Components

```
+----------------------------------------------------------+
|                    SelfStack Binary                       |
|                                                           |
|  +----------------------------------------------------+  |
|  |                    REST API                         |  |
|  |  POST /apps/install    GET /apps                    |  |
|  |  POST /apps/:id/start  DELETE /apps/:id             |  |
|  |  GET /apps/:id/logs    POST /apps/:id/export        |  |
|  |  GET /status           POST /devices/trust          |  |
|  +----------+-----------------------------+------------+  |
|             |                             |               |
|  +----------v--------+     +-------------v-----------+   |
|  |  Web Dashboard     |     |  Container Manager      |   |
|  |  (React SPA,       |     |  - Docker SDK           |   |
|  |   embedded)        |     |  - Port allocator       |   |
|  +--------------------+     |  - Volume manager       |   |
|                              |  - Health checker       |   |
|  +--------------------+     +-------------+-----------+   |
|  |  Reverse Proxy     |<-----------------+               |
|  |  (Caddy-based)     |                                   |
|  +--------+-----------+                                   |
|           |                                               |
|  +--------v-----------+     +------------------------+    |
|  |  Tunnel Manager    |     |  Auth / Device Trust    |    |
|  |  (Cloudflare)      |     |  - Device registry      |    |
|  +--------------------+     |  - Key exchange         |    |
|                              |  - Session tokens       |    |
|  +--------------------+     +------------------------+    |
|  |  Registry Client   |     +------------------------+    |
|  |  (fetches catalog) |     |  Config Store (SQLite)  |    |
|  +--------------------+     +------------------------+    |
+----------------------------------------------------------+
```

| Component | Role |
|-----------|------|
| REST API | Single entry point for GUI, CLI, and agents. Every action is an API call. |
| Web Dashboard | React SPA embedded in the binary. Browse, install, manage apps. |
| Container Manager | Talks to Docker via SDK. Creates containers, assigns ports, manages volumes, runs health checks. |
| Reverse Proxy | Routes `appname.selfstack.local` to the right container port. Handles TLS for tunnel. |
| Tunnel Manager | Configures Cloudflare Tunnel for remote access. One tunnel, all apps behind it. |
| Auth | Device trust registry. Manages trusted devices, session tokens, new device pairing. |
| Registry Client | Fetches the curated app catalog from GitHub. Caches locally. |
| Config Store | SQLite database tracking installed apps, port assignments, device keys, user config. |

## App Manifest Spec (`selfstack.yml`)

Any GitHub repo with a valid `selfstack.yml` can be installed by SelfStack.

```yaml
name: budget-tracker
display_name: Budget Tracker
description: Personal finance tracker. NerdWallet alternative.
version: 1.0.0
icon: icon.png

runtime:
  type: docker-compose
  entry: docker-compose.yml

expose:
  port: 3000          # internal container port (SelfStack manages host port)
  health: /api/health

volumes:
  - data:/app/data

config:
  - key: CURRENCY
    default: USD
    description: Default currency
```

**Key decisions:**
- `port` is the internal container port. SelfStack dynamically assigns host ports (e.g., 10001, 10002) to avoid conflicts. Users never see port numbers.
- Docker Compose is the standard runtime - most vibe-coded apps already have one.
- Single exposed HTTP port per app. SelfStack handles all routing.
- Named volumes for persistent data. SelfStack manages volume lifecycle.
- Config as simple key-value pairs injected as environment variables.

## App Lifecycle

### Install
```
User clicks "Install" (or: selfstack install budget-tracker)
  -> Registry lookup: resolve name -> GitHub repo URL
  -> Clone repo to ~/.selfstack/apps/budget-tracker/
  -> Parse selfstack.yml manifest
  -> Pull/build Docker image (docker compose build)
  -> Allocate host port (e.g., 10003)
  -> Create named volumes for persistent data
  -> Start container
  -> Wait for health check to pass
  -> Register route: budget-tracker.selfstack.local -> :10003
  -> Dashboard shows: Budget Tracker - Running
```

### Start / Stop
```
selfstack start budget-tracker   -> docker start + re-register route
selfstack stop budget-tracker    -> docker stop + de-register route
```

### Update
```
selfstack update budget-tracker
  -> git pull latest
  -> Rebuild image
  -> Stop old container (volumes preserved)
  -> Start new container with same volumes
  -> Health check -> re-register route
```

### Remove
```
selfstack remove budget-tracker
  -> Stop container
  -> Prompt: "Export data before removing?"
  -> If yes: export volume to ~/.selfstack/exports/budget-tracker-<date>.tar.gz
  -> Remove container + image
  -> Deallocate port
  -> Remove route
  -> Optionally remove volumes (user confirms)
```

### Data Export / Import
```
selfstack export budget-tracker     -> tar.gz of all volumes
selfstack import budget-tracker <file> -> restore volumes
```

## Networking & Remote Access

### Local Access (home network)
SelfStack runs a reverse proxy on port 80/443 of the host machine. Uses mDNS so devices on the same WiFi access apps by name:

```
http://selfstack.local          -> Dashboard
http://budget-tracker.local     -> Budget Tracker app
http://notes.local              -> Notes app
```

### Remote Access (anywhere)
Cloudflare Tunnel (free tier) creates an encrypted connection:

```
Mac Mini <--encrypted tunnel--> Cloudflare Edge <-- Your phone (anywhere)

URLs:
https://budget-tracker.your-id.selfstack.dev
https://notes.your-id.selfstack.dev
```

**Setup flow:**
1. During first run, user creates/links a free Cloudflare account
2. SelfStack generates a tunnel token and configures routes automatically
3. User gets a subdomain (e.g., `your-id.selfstack.dev`) or uses their own domain
4. Every installed app automatically gets a route
5. Auth layer ensures only trusted devices can access

### Port Management
- Apps declare their internal container port in `selfstack.yml`
- SelfStack assigns host ports dynamically (starting at 10001)
- Reverse proxy routes by app name - users never see port numbers
- No conflicts possible since each container has its own network namespace

## Authentication: Device-Based Trust

### First-Time Setup
1. User installs SelfStack on their Mac Mini
2. SelfStack generates a master key pair (Ed25519, stored in `~/.selfstack/keys/`)
3. Mac Mini is automatically the first trusted device
4. SelfStack displays a QR code + 6-word recovery phrase
5. User writes down recovery phrase (offline backup)

### Adding a New Device
1. User opens SelfStack dashboard from phone browser
2. Sees "New device detected - approve from a trusted device"
3. On Mac Mini, notification: "New device wants access: iPhone 15 - Approve / Deny"
4. User clicks Approve
5. Phone receives a device token (stored in browser/keychain)
6. Phone is now trusted - no login needed going forward

### Request Flow
```
Phone -> Cloudflare Tunnel -> SelfStack reverse proxy
  -> Auth middleware checks device token (cookie/header)
  -> If trusted: proxy to app container
  -> If untrusted: redirect to approval page
```

### Security Properties
- No passwords to remember or leak
- Device tokens are cryptographic (Ed25519 signed)
- Tokens can be revoked per-device from dashboard
- Recovery phrase bootstraps a new trusted device if all devices lost
- Optional PIN for extra security on approval step

## CLI & Agent Interface

### CLI Commands
```bash
# App management
selfstack install <app>              # Install from registry
selfstack list                       # List installed apps + status
selfstack start <app>                # Start an app
selfstack stop <app>                 # Stop an app
selfstack update <app>               # Pull latest + rebuild
selfstack remove <app>               # Remove an app
selfstack logs <app>                 # Stream app logs

# Data
selfstack export <app>               # Export app data
selfstack import <app> <file>        # Import app data

# System
selfstack status                     # Framework health + all apps
selfstack registry search <query>    # Search app catalog
selfstack devices list               # List trusted devices
selfstack devices revoke <id>        # Revoke a device

# Config
selfstack config <app> set KEY=val   # Set app config
selfstack config <app> get KEY       # Get app config
```

### REST API
```
GET    /api/apps                    -> list apps
POST   /api/apps/install            -> install app
POST   /api/apps/:id/start          -> start app
POST   /api/apps/:id/stop           -> stop app
DELETE /api/apps/:id                -> remove app
GET    /api/apps/:id/logs           -> stream logs
POST   /api/apps/:id/export         -> export data
GET    /api/status                  -> system health
GET    /api/registry/search?q=...   -> search catalog
GET    /api/devices                 -> list trusted devices
DELETE /api/devices/:id             -> revoke device
```

### Agent Integration
AI agents interact with SelfStack via:
1. **CLI** - agent runs shell commands (`selfstack install budget-tracker`)
2. **REST API** - agent makes HTTP calls to `http://localhost:8080/api/...`
3. **MCP Server** (V2) - SelfStack exposes itself as an MCP tool for native agent integration

## App Registry

A GitHub repository (`selfstack/registry`) containing `registry.json`:

```json
{
  "apps": [
    {
      "name": "budget-tracker",
      "display_name": "Budget Tracker",
      "description": "Personal finance tracker. NerdWallet alternative.",
      "category": "Finance",
      "repo": "https://github.com/selfstack-apps/budget-tracker",
      "icon": "https://raw.githubusercontent.com/.../icon.png",
      "verified": true
    }
  ],
  "categories": ["Finance", "Productivity", "Lifestyle", "Health", "Media", "Dev Tools"]
}
```

- Static metadata in JSON; dynamic stats (stars, forks, last updated) fetched from GitHub API at display time
- Cached locally for offline use and to avoid rate limits
- Community submits new apps via PR
- `verified: true` means a maintainer has validated the manifest and tested the app
- Dashboard displays apps in a grid with icons, categories, live GitHub stats, and install buttons

## Data Management (V1)

- **Persistent Docker volumes** for all app data (survives container restarts, updates, reinstalls)
- **Manual export/import** via CLI or dashboard (`selfstack export/import`)
- Exports are `.tar.gz` archives of volume contents
- No automated backup in V1

## V1 Scope

### Included
- Go binary with embedded React/TypeScript dashboard
- `selfstack.yml` manifest spec
- Curated app registry (GitHub JSON + dynamic GitHub API stats)
- Docker-based container management with automatic port allocation
- Reverse proxy routing (`appname.selfstack.local`)
- Cloudflare Tunnel for remote access
- Device-based trust authentication (QR pairing, recovery phrase)
- CLI with full feature parity to GUI
- REST API for agent access
- Persistent Docker volumes
- Manual data export/import
- Full app lifecycle: install, start, stop, update, remove
- Health checks and status monitoring
- App config via environment variables

### Deferred (V2+)
- Direct GitHub URL install with auto-manifest generation (potentially AI-powered)
- MCP server for native agent integration
- Multi-user / household support
- Automated encrypted backups
- User ratings/reviews in registry
- Version pinning and rollback
- Tailscale as alternative tunnel provider
- PWA / mobile wrapper for dashboard

## Tech Stack Summary

| Layer | Technology |
|-------|-----------|
| Binary / API | Go |
| Dashboard | React + TypeScript (embedded in binary) |
| Container orchestration | Docker SDK for Go |
| Reverse proxy | Caddy (embedded as Go library) |
| Remote access | Cloudflare Tunnel |
| Auth | Ed25519 device tokens |
| Data store | SQLite |
| App runtime | Docker Compose |
| Registry | GitHub-hosted JSON |
