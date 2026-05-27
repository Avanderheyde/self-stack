# SelfStack MVP Demo — Paradigm Fellowship

**Audience:** Paradigm reviewers. Optimize for the 30-second skim, but ship enough proof to back the claims.

**The thesis in one sentence:** One CLI command takes an AI-generated personal app from `localhost` to a private URL on a VPS you own, reachable from your phone over Tailscale, in under 90 seconds.

---

## 1. The demo (record this)

### Setup (one-time, off-camera)

```bash
# Founder already ran this once. Takes ~3 minutes total.
selfstack cloud setup
# → prompts for Hetzner API token (~$4/mo VPS)
# → optional Tailscale auth key
# → provisions cax11 VPS in fsn1, installs Docker + Tailscale + selfstack as systemd
#
# Before the first public release, use:
# selfstack cloud setup --install-url https://raw.githubusercontent.com/<owner>/self-stack/<branch>/scripts/install.sh
```

### On-camera demo (target: <90 s end-to-end)

```bash
# 1. Vibe-code an app with Claude in a fresh directory.
#    For the recording, scaffold a tiny expense tracker:
#      "Claude, build a single-file Node.js Express app that lists
#       expenses from a JSON file. Add a Dockerfile."
#    → 30 s of Claude generating code.

# 2. Deploy.
cd ~/code/demo-expenses
selfstack deploy
# expected output (SSE-streamed):
#   Detected node project: demo-expenses
#   Syncing to root@<vps-ip>:~/.selfstack/apps/demo-expenses/
#     ✓ Files synced
#     → generating Dockerfile
#     → docker compose build
#     → allocating port 10001
#     → starting container
#     → health check passing
#   ✓ demo-expenses deployed successfully on port 10001
#     Local: http://demo-expenses.selfstack.local
#     Direct: http://localhost:10001
#     Tailscale: https://<vps-machine>.<tailnet>.ts.net:10001

# 3. Open the Tailscale URL on the phone. Add an expense. Done.
```

### What the viewer should walk away with

- **One command.** No `ssh`, no `docker compose`, no nginx, no DNS, no `certbot`.
- **Your box.** The URL points at a VPS the founder owns. Not Vercel, not Lovable.
- **Private by default.** The URL only resolves inside the Tailscale network.
- **The dev didn't write any infra config.** Auto-detection generated the Dockerfile.

> **TODO before recording:** time three live runs. Capture the median. Numbers in this doc are *targets from the design*, not measured ([`docs/plans/2026-03-28-deploy-command-design.md`](plans/2026-03-28-deploy-command-design.md) §Success Criteria: <60 s redeploy, <3 min first deploy).

---

## 2. Proof bundle

Things to attach to the application:

| Artifact | Where it lives | Status |
|---|---|---|
| 90-second screen recording of the demo above | `docs/assets/demo.mp4` | TODO — record after timing dry runs |
| Source: `cmd/deploy.go` (196 LOC) | this repo | ✅ shipped |
| Source: `cmd/cloud.go` + `internal/cloud/hetzner.go` (Hetzner provisioning) | this repo | ✅ shipped |
| Source: `internal/tailscale/tailscale.go` (Tailscale Serve wrapper, local + remote) | this repo | ✅ shipped |
| Source: `internal/detect/` (project-type detection + Dockerfile/compose generation) | this repo | ✅ shipped |
| Design doc with autoplan review trail | [`docs/plans/2026-03-28-deploy-command-design.md`](plans/2026-03-28-deploy-command-design.md) | ✅ shipped |
| Original architecture design | [`docs/plans/2026-02-18-selfstack-design.md`](plans/2026-02-18-selfstack-design.md) | ✅ shipped |
| Integration test that builds + runs the sample app via Docker SDK | `test/e2e_test.go` (build tag `integration`) | ✅ shipped |
| Git history of the deploy command landing in one week | `git log --oneline` (commits `274a16b` → `069ba1d`) | ✅ verifiable |
| One non-founder using `selfstack deploy` end-to-end | n/a | TODO — design's stated success bar |
| List of apps the founder personally runs on his SelfStack VPS | TBD | TODO — claim references "vibe-costs" expense tracker and a Monarch/NerdWallet clone; verify currently running |

---

## 3. Metrics to collect before submitting

The application is stronger if we can put numbers next to each claim. Measure these in a clean run before recording the demo:

1. **First-deploy wall-clock time** (fresh project dir, no warm Docker cache on VPS).
   - Target: <3 minutes. Capture: median of 3 runs.
2. **Redeploy wall-clock time** (same project, code change, warm cache).
   - Target: <60 seconds. Capture: median of 3 runs.
3. **Time from `selfstack cloud setup` enter to "SelfStack Cloud ready!"**.
   - Captures Hetzner provision + cloud-init + selfstack systemd start.
   - Capture: one run on a fresh Hetzner project.
4. **Binary size** of the `selfstack` release artifact (macOS arm64).
   - Run: `ls -lh dist/` after `goreleaser release --snapshot`.
5. **Cost per app, monthly.** Hetzner cax11 = ~$4/mo. Divided by the founder's app count (currently TBD, design says ≥2).
6. **Lines of code the user wrote** for the demo expense app vs. the deploy command output (i.e., the leverage ratio).

Record these as a single table in the recording's description or pinned comment.

---

## 4. Open questions before the deadline

1. **Is the installer branch/release published?** `selfstack cloud setup` now accepts `--install-url`; before public launch, push the review branch or create a release so a fresh VPS can curl the installer.
2. **Does the demo VPS need MagicDNS pre-enabled** for `tailscale serve --bg <port>` to produce a working `https://...ts.net:PORT` URL? The design assumes default-on; confirm on a fresh tailnet before recording.
3. **Recording medium.** Asciinema for the terminal portion + a phone-camera shot of the Tailscale URL loading, or a single screen-share with QuickTime? Asciinema is more honest (no cuts) but less visceral than seeing the phone.
4. **The non-founder user.** Design doc §Success Criteria item 3: "at least one person besides the founder deploys an app." That bar is not yet met. Either (a) close it before submitting, or (b) call it out honestly in the application as the next milestone.
5. **Should the demo app be live during the Paradigm review window?** If a reviewer asks for a URL we can't share a Tailscale link. Two options: (a) just attach the recording; (b) spin up a Cloudflare Tunnel for the demo URL — but `--tunnel cloudflare` isn't wired up yet (TODO in `cmd/deploy.go`).

---

## 5. Paradigm application paragraph

> SelfStack is a single Go binary that closes the deployment gap for AI-generated personal apps. You vibe-code an app with Claude, type `selfstack deploy`, and 60 seconds later it's running on a $4/mo Hetzner VPS you own, reachable from your phone over Tailscale — no public URL, no DNS, no `docker compose` knowledge required. One earlier command, `selfstack cloud setup`, provisions the VPS, installs Docker + Tailscale + selfstack as a systemd service, and configures everything end-to-end. The thesis: AI made building personal software free, but hosting it didn't get easier; every competitor (Vercel, Lovable, Bolt, Replit) deploys to *their* cloud and locks you into per-app pricing and a data tenancy you don't control. SelfStack inverts that — your hardware, your data, one binary, one command. The MVP ships the full deploy path today; the moat we're building toward is lifecycle management across the ten-plus personal apps a single user accumulates: backups, upgrades, secrets, health monitoring, and team-tier access via Tailscale ACLs.

(~165 words. Tighten or expand to fit Paradigm's actual character limit.)

> **TODO:** verify Paradigm's submission character limit; trim or expand accordingly. Variants for 50-word, 100-word, and 250-word slots live below.

### 50-word version

> SelfStack is one Go binary that takes an AI-generated personal app from `localhost` to a private URL on a $4/mo VPS you own, behind Tailscale, in under 90 seconds. No public URL, no DNS, no Docker knowledge. Vercel-style ergonomics for the apps you don't want on someone else's cloud.

### 100-word version

> SelfStack closes the deployment gap for AI-generated personal apps. You vibe-code something with Claude, type `selfstack deploy`, and 60 seconds later it's running on a $4/mo Hetzner VPS you own, reachable from your phone over Tailscale. `selfstack cloud setup` provisions the VPS end-to-end the first time. The thesis: AI made building software free, but every competitor (Vercel, Lovable, Bolt) deploys to their cloud and meters you per-app. SelfStack inverts that — your hardware, your data, one binary, one command. The moat we're building toward is lifecycle management for the ten-plus personal apps a single user accumulates.

---

## 6. Risks the reviewer will probably ask about

| Risk | Honest answer |
|---|---|
| "Any AI can write a deploy script in an afternoon." | True. The deploy command is a wedge. The defensible product is lifecycle management across many apps on one VPS, plus the team tier via Tailscale ACLs. See design §Premises 5–8. |
| "Why not just use Coolify / Dokku / CapRover?" | Those are self-hosting platforms for *existing* open-source software. SelfStack targets *new* AI-generated apps and intercepts the moment of creation in the Claude/Cursor workflow. Different distribution channel, different user. |
| "How is this not just `git push` to a Hetzner box?" | Auto-detection, Dockerfile generation, port allocation, health checks, Tailscale Serve config, redeploy with volume preservation, undeploy with proxy/tunnel cleanup — all collapsed into one verb. The savings compound across ten apps. |
| "Why Tailscale instead of a public URL?" | Personal apps don't need public exposure, and Tailscale eliminates the DNS/SSL/auth problem entirely. Public access remains on the roadmap via Cloudflare Tunnel (TODO: `--tunnel cloudflare` flag). |
| "Where's the demand evidence?" | Founder dogfooding only. Design doc §Demand Evidence is honest about this. The Paradigm demo *is* the next validation step; the assignment in `2026-03-28-deploy-command-design.md` §Assignment is to ship the demo and find three users. |

---

## 7. Changed / created files (for this docs revamp)

- `README.md` — new top-level README; 30-second thesis up top, full command surface below, shipped vs. TODO clearly split
- `docs/mvp-demo.md` — this file
