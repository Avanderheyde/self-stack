# SelfStack Public Launch Checklist

SelfStack should be open-sourced under `Avanderheyde/self-stack`.

Do not make the repository public until this checklist is complete.

## P0: Secret And History Safety

- [ ] Revoke/rotate the Anthropic API key found by the history audit.
- [ ] Use a clean public snapshot or verified-clean branch before publishing.
- [ ] Run `scripts/secret-audit.sh` on the exact branch or snapshot that will be
      pushed public.
- [ ] Confirm the audit exits `0`.

Current state: the normal branch/tag/remotes surface audits clean after local
Conductor checkpoint refs were deleted. Still prefer a fresh public snapshot
for the first OSS launch so internal branch history and local-only refs cannot
accidentally leak through a mirror-style push.

Recommended safe publication path:

```bash
# From a clean tree after the key is revoked/rotated.
mkdir -p /tmp/selfstack-public
rsync -a --delete \
  --exclude .git \
  --exclude dashboard/node_modules \
  --exclude dashboard/dist \
  --exclude .claude \
  --exclude .playwright-mcp \
  --exclude selfstack \
  /Users/jarvis/Code/self-stack/ /tmp/selfstack-public/

cd /tmp/selfstack-public
git init -b main
git add .
git commit -m "SelfStack v0.1.0 public launch"
./scripts/secret-audit.sh
```

Only push that clean snapshot after the audit passes.

## P0: Release And Install

- [ ] Make `Avanderheyde/self-stack` public or create it as the public canonical
      repository.
- [ ] Push the sanitized `main`.
- [ ] Tag `v0.1.0`.
- [ ] Confirm GitHub Actions/GoReleaser publishes:
  - `selfstack_darwin_amd64.tar.gz`
  - `selfstack_darwin_arm64.tar.gz`
  - `selfstack_linux_amd64.tar.gz`
  - `selfstack_linux_arm64.tar.gz`
  - `checksums.txt`
- [ ] From a clean machine, verify:

```bash
curl -fsSL https://raw.githubusercontent.com/Avanderheyde/self-stack/main/scripts/install.sh | sh
selfstack status
```

## P0: Paradigm Live Proof

- [ ] Provision a fresh Hetzner project/server with `selfstack cloud setup`.
- [ ] Deploy one proof-bundle app with `selfstack deploy`.
- [ ] Open the Tailscale URL from a phone.
- [ ] Capture terminal output, dashboard screenshot, phone screenshot, and timing.
- [ ] Update `docs/mvp-demo.md` with measured timings instead of design targets.

Do not use a shared reviewer URL unless a public tunnel is intentionally added.
The current demo story should be a recording plus source code, because the app
URL is private to the tailnet by design.

## P1: Public Repo Polish

- [x] Add license file.
- [x] Add concise contribution guidance.
- [x] Add security contact/reporting note.
- [x] Add release/change log.
- [x] SRE agent + eval harness moved to `Avanderheyde/self-stack-hosted`
      (commit 2026-05-27); OSS surface no longer contains `cmd/sre-agent`,
      `cmd/eval`, `internal/sreagent`, `internal/eval`, or `evals/`.
