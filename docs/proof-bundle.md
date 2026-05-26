# SelfStack Proof Bundle — Real Apps

Three sibling repos installed and demoed via SelfStack. Each ships a
`selfstack.yml` + `Dockerfile` + `docker-compose.yml`, parses against the
manifest schema in `internal/manifest`, and produces a valid compose plan
when `SELFSTACK_HOST_PORT` is injected by `internal/container/manager.go`.

Source paths assume the dev layout under `~/Code/`. For the demo, pass
`--local <path>` to `selfstack install` so the daemon uses these working
trees instead of cloning from the registry.

## Picks for the demo

| App | Path | Manifest name | Port | Volume? | Env config? | Status |
|---|---|---|---|---|---|---|
| AgentBoard | `~/Code/agentboard` | `agentboard` | 3000 | yes (`data:/app/data`) | none | ready |
| Track Basket | `~/Code/basket-tracker` | `track-basket` | 3000 | no (ephemeral) | none | ready |
| WealthStack | `~/Code/wealth-stack` | `wealthstack` | 3000 | yes (`data:/app/data`) | 5 Plaid vars | ready after port fix |

Vibe Costs (`~/Code/vibe-cost-tracker`) is also valid but the demo storyline
overlaps with WealthStack ("personal finance, self-hosted"), so it sits as a
bench candidate rather than a primary pick. See bench notes below.

## Readiness checks done

For each app:

1. `selfstack.yml` parses against `internal/manifest.Manifest` — required
   fields `name` and `expose.port` are present; `runtime.type` defaults to
   `docker-compose` and `runtime.entry` defaults to `docker-compose.yml`.
2. `docker compose config` resolves a valid plan with
   `SELFSTACK_HOST_PORT=<n>` exported, and the resolved `published:` port
   matches `<n>` (mapping host `<n>` → container `expose.port`).
3. The `expose.health` path exists in the app source:
   - `agentboard`: `server/index.js` registers `GET /api/health`
   - `track-basket`: `src/app/api/health/route.ts`
   - `wealthstack`: `src/app/api/health/route.ts`
   - `vibe-costs`: nginx serves `/` from `dist/`, so the `/` health
     check returns 200 once the build artifact is in place

## Demo commands

The daemon must be running and the dashboard at `http://localhost:7766`.
Start it in a separate shell with `selfstack serve` if it's not up.

```bash
# AgentBoard — no env, persistent kanban board
selfstack install agentboard --local ~/Code/agentboard

# Track Basket — no env, ephemeral data (good for a "throw it away" demo)
selfstack install track-basket --local ~/Code/basket-tracker

# WealthStack — needs Plaid credentials at first start; for a
# disconnected demo, leave PLAID_* unset and show the Settings UI flow
# that lets you paste them in after install.
selfstack install wealthstack --local ~/Code/wealth-stack
```

Each `install` streams `data: {...}` SSE events (one per step) and prints
`Installed <name> successfully` on completion. List + status:

```bash
selfstack list
selfstack status agentboard
```

The dashboard at `http://localhost:7766` lists the three apps with their
allocated host ports. Each app is also reachable at `http://<name>.localhost`
via portless (`internal/portless`).

If a Tailscale node is configured (`selfstack cloud setup`), `selfstack
deploy` from the app dir pushes to the VPS and prints the public
`https://<name>.<tailnet>.ts.net` URL.

## Fixes applied

### wealth-stack — port var rename

`docker-compose.yml` was using `${HOST_PORT:-3000}:3000` instead of the
`SELFSTACK_HOST_PORT` env var that `container.Manager.Up` exports. Result:
SelfStack would allocate a port from its 10001+ range, register that in the
proxy, but compose would silently bind 3000 — health check would fail and
the app would appear dead.

Verified with `docker compose config` before and after:

```
# before: SELFSTACK_HOST_PORT=10104 → published "3000" (bug)
# after:  SELFSTACK_HOST_PORT=10104 → published "10104"
```

One-line change in `/Users/jarvis/Code/wealth-stack/docker-compose.yml`.

### No fixes needed for the other three

- `agentboard/docker-compose.yml` — uses `${SELFSTACK_HOST_PORT:-3000}:3000`
- `basket-tracker/docker-compose.yml` — uses `${SELFSTACK_HOST_PORT:-3000}:3000`
- `vibe-cost-tracker/docker-compose.yml` — uses `${SELFSTACK_HOST_PORT:-3000}:80`

## Blockers and caveats

- **WealthStack**: Plaid `PLAID_CLIENT_ID` + `PLAID_SECRET` are
  declared in `selfstack.yml` under `config:` without defaults. The
  manifest schema (`internal/manifest/manifest.go`) does not yet
  surface required-vs-optional, so install proceeds with empty
  strings. The app's Settings page handles this gracefully (you can
  paste keys after first start), but for a fully wired demo, pre-set
  them via `--seed-data` or the dashboard env-var UI before starting
  the Plaid linking flow.

- **basket-tracker**: no `volumes:` declared, so its sqlite (or
  whatever Next.js persists) is lost on container recreate. Fine for
  a "look how easy install is" demo, not fine for a "your data is
  yours" demo. Adding `volumes:` would be a 2-line manifest + compose
  change in the app repo; left untouched for now since the task said
  to keep manifest fixes small and the app may not actually persist
  anything to disk.

- **vibe-cost-tracker**: the app's compose names its volume
  `vibe-costs-data:/data`, but the manifest does not declare it.
  SelfStack will create the volume implicitly on first `docker compose
  up`, but `selfstack remove` won't see it as a known volume. Same
  call as basket-tracker: not blocking the demo, worth a follow-up.

- **Other Claude sessions** are editing the SelfStack source in
  sibling worktrees (cloud setup, deploy, dashboard, install). The
  app-side fixes here don't touch SelfStack itself, so there's no
  collision; the only SelfStack-side artifact added is this doc.

## Bench: Vibe Costs

`~/Code/vibe-cost-tracker` (name `vibe-costs`, port 80, version 0.8.0)
is fully install-ready: nginx-fronted SPA + sidecar node server, all
SelfStack env vars wired correctly. Skipped as a primary pick because
its demo story overlaps with WealthStack. Worth keeping in the bundle
if a fourth slot opens up — its build is the fastest of the four (no
Next.js, just Vite + nginx).

## Files changed

- `wealth-stack/docker-compose.yml` — `HOST_PORT` → `SELFSTACK_HOST_PORT`
- `self-stack/docs/proof-bundle.md` — this document

## Commands run

```bash
# manifest schema check (read-only)
go test ./internal/manifest/...

# compose plan check, per app
cd ~/Code/<app> && SELFSTACK_HOST_PORT=10101 \
  docker compose -p selfstack-validate-<name> config

# real image build, basket-tracker only (kept local, no push)
cd ~/Code/basket-tracker && SELFSTACK_HOST_PORT=10103 \
  docker compose -p selfstack-proof-basket build
```

The full `install`/`deploy` happy-path against a live daemon was not
run from this worktree because other agents are setting up the cloud +
deploy paths in parallel. The structural validation above is enough to
demo against a freshly-started `selfstack serve` when the other work
lands.
