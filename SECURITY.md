# Security

SelfStack is pre-launch software for deploying personal apps to user-owned
infrastructure. We take vulnerability reports seriously.

## Reporting

Please report security issues privately to the repository owner rather than
opening a public issue. Include:

- affected version or commit
- reproduction steps
- expected and actual behavior
- any logs or screenshots with secrets redacted

We aim to acknowledge reports within 3 business days and to ship a fix or
mitigation within 14 days for high-severity issues.

## In scope

- Remote code execution on the SelfStack control plane (`selfstack serve`).
- Authentication / authorization bypass on the dashboard API.
- Unauthenticated access to deployed apps that should be Tailscale-only.
- Secret exposure: Hetzner API tokens, Tailscale auth keys, SSH keys, or
  app environment variables leaked outside the user's tailnet.
- Path traversal, SSRF, or command injection in the install / deploy /
  cloud flows.
- Docker socket misuse from generated `docker-compose.yml`.

## Out of scope

- Attacks requiring pre-existing root on the user's laptop or VPS.
- Social engineering of the maintainers.
- DoS against the local control plane.
- Issues in upstream Docker, Tailscale, or Hetzner — please report those
  upstream.

## Secrets

Never commit real API keys, Tailscale auth keys, Hetzner tokens, SSH private
keys, `.env` files, SQLite databases, or logs containing credentials.

Before making release branches public, or before opening a fork as public,
run:

```bash
scripts/secret-audit.sh
```

If a secret is found in history, revoke or rotate it before publishing, then
publish from a sanitized history or clean snapshot. See
[`docs/runbooks/oss-launch.md`](docs/runbooks/oss-launch.md) for the
detailed remediation sequence.

## Operator responsibilities

SelfStack runs as root on the user's VPS by design (it manages Docker and
systemd). Operators should:

- Keep their Tailscale ACL tight; SelfStack-deployed apps default to
  per-app Tailscale Serve routes.
- Rotate Hetzner API tokens at least every 90 days.
- Never commit `.env` files or `~/.selfstack/secrets/`.

