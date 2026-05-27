# Contributing

SelfStack is early. Small, focused changes are easiest to review.

## Development

```bash
go test ./...
go vet ./...
```

Run the Docker integration test when touching deploy/install behavior:

```bash
SELFSTACK_INTEGRATION=1 go test -tags integration ./test
```

Build the CLI:

```bash
go build -o selfstack .
```

## Before Opening A PR

- Keep changes scoped to one behavior or document.
- Add or update focused tests for behavior changes.
- Do not commit local state, generated binaries, `.env` files, logs, databases,
  or credentials.
- Run `scripts/secret-audit.sh` if the change touched docs, examples, release
  scripts, or anything involving credentials.

## Hosted Product Boundary

The public repo is the local-first CLI/control plane. Hosted account creation,
billing, token vaults, fleet management, and guided provisioning belong in the
private `Avanderheyde/self-stack-hosted` repo.

