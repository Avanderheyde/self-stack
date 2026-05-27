# Live Demo Runbook — measured proof for `docs/mvp-demo.md`

End-to-end sequence to produce **measured** numbers backing the demo
narrative. Replaces the design-target placeholders currently in
[`docs/mvp-demo.md`](../mvp-demo.md) and the README pitch.

Pre-flight: this *does* spend real money (~$0.01 of Hetzner per run for a
cax11 in fsn1 destroyed within the hour) and *does* burn a Tailscale auth
key. Do not run without explicit approval.

---

## Prereqs (one-time)

```bash
# A working selfstack binary in PATH.
make build
sudo mv selfstack /usr/local/bin/

# Tokens. Store these in your password manager, not in tracked files.
export HETZNER_TOKEN="hcloud_…"
export TAILSCALE_AUTH_KEY="tskey-auth-…"      # reusable, ephemeral, tag:selfstack-demo

# A demo app. The proof bundle's AgentBoard is the lightest to time.
cd ~/Code/agentboard
```

Sanity-check the local stack:

```bash
selfstack --version
docker compose version --short || docker-compose version --short
scripts/secret-audit.sh
```

All three must succeed before the timed runs.

---

## The three runs

For each run, capture a wall-clock time. The simplest reliable timer:

```bash
time selfstack cloud setup --install-url \
  "https://raw.githubusercontent.com/Avanderheyde/self-stack/main/scripts/install.sh"
```

### Run 1 — cold provision + first deploy

```bash
# 1a. Provision (target ~2 min, design)
time selfstack cloud setup            # capture T_provision

# 1b. First deploy (target <3 min, design)
cd ~/Code/agentboard
time selfstack deploy --name agentboard   # capture T_first_deploy

# 1c. Smoke-test the Tailscale URL.
selfstack list
# Open the Tailscale URL printed by `selfstack deploy` from the phone.
# If needed, inspect app status through the control-plane API:
curl -fsSL http://localhost:8080/api/apps
```

### Run 2 — warm redeploy (no code change)

```bash
cd ~/Code/agentboard
time selfstack deploy --name agentboard   # capture T_warm_redeploy_1
```

Target from the design: <60 s when the layer cache is warm.

### Run 3 — warm redeploy (single-line code change)

```bash
cd ~/Code/agentboard
echo "// touch $(date)" >> server/index.js
time selfstack deploy --name agentboard   # capture T_warm_redeploy_2
```

### Teardown

```bash
selfstack cloud destroy   # confirm with "destroy"
tailscale --socket=/var/run/tailscale/tailscaled.sock device delete \
  agentboard-vps || true
```

---

## What to record

Fill these into the table at the bottom of `docs/mvp-demo.md`:

| Phase | Measured | Design target | Run |
|---|---|---|---|
| Cold provision | T_provision | ~2 min | 1 |
| First deploy | T_first_deploy | <3 min | 1 |
| Warm redeploy (no change) | T_warm_redeploy_1 | <60 s | 2 |
| Warm redeploy (1-line change) | T_warm_redeploy_2 | <60 s | 3 |

Median of the two redeploys is the headline number. Replace the
"design target" disclaimer in:
- `README.md` line ~11 ("A minute later…")
- `docs/mvp-demo.md` line ~5 (thesis sentence)
- `docs/mvp-demo.md` line ~60 (TODO before recording)

with the measured median.

---

## What to screenshot

For the proof bundle:

1. Terminal showing each `time` block + final `selfstack list` output.
2. Tailscale admin console showing the new node + ACL.
3. Browser at `https://agentboard.<tailnet>.ts.net` rendering the kanban.
4. Hetzner dashboard showing the cax11 in `fsn1` with cost.

Save under `docs/assets/live-demo/` and reference from
`docs/proof-bundle.md`.

---

## Failure modes — what to do

| Symptom | First check | Fix |
|---|---|---|
| `cloud setup` hangs at "waiting for cloud-init" | `ssh root@<ip> cat /var/log/cloud-init-output.log` | Most often a transient Hetzner image-pull stall; retry. |
| Install script 404s on the VPS | curl-test `SELFSTACK_INSTALL_URL` from your laptop | Pass `--install-url` pointing at the raw branch file. |
| `docker compose build` fails on the VPS | `ssh root@<ip> docker compose version` | If only legacy `docker-compose` exists, the manager picks it up; if neither is installed, re-run cloud-init or `apt install docker-compose-plugin`. |
| Health-check never passes | `ssh root@<ip> docker compose -p selfstack-<app> logs --tail=200` | App-specific; commonly missing `PORT=$SELFSTACK_CONTAINER_PORT` wiring. |
| Tailscale URL 502s | `ssh root@<ip> tailscale serve status` | Re-run `selfstack deploy` after fixing the underlying container. |

---

## Cleanup checklist

```bash
selfstack cloud destroy                    # tears down the VPS
hcloud server list --token "$HETZNER_TOKEN" # confirm no orphans
tailscale status                            # confirm node removed
```

Then revoke the Tailscale auth key in the admin console (Settings → Keys).
