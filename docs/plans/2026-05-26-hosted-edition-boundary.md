# Hosted Edition Boundary

SelfStack should launch as an open-source, local-first CLI/control plane under
`Avanderheyde/self-stack`. The hosted product should be developed as a separate
layer around that core, not mixed into the OSS MVP.

## Open-source core

Keep these in the public repo:

- CLI commands: `install`, `update`, `cloud setup`, `init`, `deploy`, `serve`, app lifecycle.
- Local dashboard and API for a user's own SelfStack instance.
- Hetzner provisioning for a user-provided token.
- Tailscale setup for a user-provided auth key.
- Docker/project detection, generated app config, deploy hardening.
- Example apps, demo docs, proof bundle, and architecture docs.

The open-source promise is: you own the VPS, data, tailnet, and binary.

## Hosted layer

Build these outside the OSS MVP boundary:

- Marketing site and docs site.
- Account creation, login, billing, and team/org management.
- Secure token vault for Hetzner/Tailscale credentials.
- Hosted onboarding wizard that drives `cloud setup` on the user's behalf.
- Fleet registry of user-owned SelfStack instances.
- Remote status/health sync, telemetry, alerts, and support tooling.
- Premium templates, managed updates, backup/restore, and migration services.
- Autonomous SRE agent (Docker GC, OOM recovery, disk-pressure watcher,
  quarantine) plus its eval harness — these were originally drafted as
  OSS-core but moved to the hosted layer on 2026-05-27 because the agent
  is designed to keep multi-tenant fleet containers alive, which is
  outside the OSS-core "you own the binary" promise.

This layer lives in a separate private repo: `Avanderheyde/self-stack-hosted`.

## Integration contract

The hosted layer should call stable OSS interfaces instead of reaching into
internals:

- `selfstack cloud setup --json`
- `selfstack deploy --json`
- `selfstack status --json`
- `selfstack update --json`
- local dashboard/API with token auth
- optional hosted callback URL for provisioning progress

If an interface is needed by hosted onboarding, add it to the OSS CLI/API as a
general-purpose capability first.

## Repo strategy

1. Public: `Avanderheyde/self-stack`
   - Apache-2.0 or MIT license.
   - Release artifacts, install script, CLI, dashboard, docs, examples.
2. Private: `Avanderheyde/self-stack-hosted`
   - Web app, auth, billing, token vault, provisioning orchestrator.
   - Depends on released SelfStack binaries or the public Go module.
3. Optional public website repo later:
   - Static marketing/docs if it becomes cleaner than keeping docs in the main
     repo.

## Near-term path

For the Paradigm application, ship the open-source proof first:

1. Make `Avanderheyde/self-stack` public after a secret/history audit.
2. Cut `v0.1.0` and verify the install script from a clean machine.
3. Run and record the Hetzner/Tailscale end-to-end proof.
4. Publish a simple website/docs page with install, demo, and architecture.
5. Keep hosted onboarding as the next milestone, not a blocker for OSS launch.
