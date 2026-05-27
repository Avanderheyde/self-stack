# OSS Launch Runbook — `Avanderheyde/self-stack`

Pre-flight checklist for making the SelfStack repo public on GitHub. Work
through P0 → P1 → P2 in order. Do not flip the repo to public until every
P0 item is checked and `scripts/secret-audit.sh` exits 0 from a clean
clone-mirror.

This runbook is **local-only**. None of the commands push, publish, or
otherwise touch shared infrastructure unless explicitly noted.

---

## P0 — Hard blockers (must complete before public flip)

### P0.1 Rotate the leaked Anthropic API key

A working `ANTHROPIC_API_KEY` was committed to `.env` on **2026-05-13** in a
Conductor checkpoint blob. The key is **REDACTED** in this doc — see
`scripts/secret-audit.sh` output for the prefix + length.

Reachability today (2026-05-26, after local Conductor checkpoint cleanup):

| Surface | Status |
|---|---|
| Working tree | clean (`.env` is gitignored) |
| `refs/heads/*` (branches) | clean |
| `refs/tags/*` | clean |
| `refs/remotes/*` (origin/master, origin/main, fork/*) | clean |
| `refs/conductor-checkpoints/*` | **cleaned** — 36 refs deleted; `make audit` + a fresh `git clone --mirror` both report 0 hits |

`make audit` exits 0. The remaining work in P0.1 is **external**: revoke the key in the Anthropic console (step 1 below). The local cleanup steps (3–7) are kept for documentation and for the next time this happens.

A default `git push` does **not** push `refs/conductor-checkpoints/*`. Risk
vectors that *would* leak the key:

- `git push --mirror` to a public remote.
- `git clone --mirror` then re-push.
- Backup or tarball of `.git/` shared externally.
- A teammate pulling the local repo via direct copy.

#### Steps (in order)

1. **Revoke the key in the Anthropic console.** Even though it's not
   push-reachable, treat it as exposed. Anthropic console → API keys →
   Revoke the key matching the prefix from the audit output.
   *External action — Alderik to confirm complete.*

2. **Generate a replacement key** and store it in `~/.selfstack/secrets/`
   or your password manager. Do not put it back in a tracked `.env`.

3. **Delete the local conductor-checkpoint refs** (after Conductor sessions
   have been closed in any UIs that depend on them):

   ```bash
   git for-each-ref --format='%(refname)' refs/conductor-checkpoints \
     | xargs -n1 git update-ref -d
   ```

4. **Drop the matching reflog entries**:

   ```bash
   git reflog expire --expire=now --all
   ```

5. **Garbage-collect dangling objects**:

   ```bash
   git gc --prune=now --aggressive
   ```

6. **Re-run the audit and confirm exit 0**:

   ```bash
   scripts/secret-audit.sh
   echo "exit=$?"   # must be 0
   ```

7. **Re-run inside a fresh clone** (this is the real test of what GitHub will
   see):

   ```bash
   tmp=$(mktemp -d)
   git clone . "$tmp/repo"
   ( cd "$tmp/repo" && scripts/secret-audit.sh )
   rm -rf "$tmp"
   ```

### P0.2 Confirm `.env` and other secret files are gitignored

```bash
grep -E '^\.env|secrets/|credentials' .gitignore
git check-ignore -v .env .env.local internal/cloud/secrets/
```

Already true at HEAD; re-verify after any history rewrite.

### P0.3 No third-party code without licence headers

```bash
git grep -nE 'Copyright \(c\) [0-9]{4}' -- '*.go' | head
```

If any file is verbatim from a third-party project, add or preserve its
LICENSE notice in `THIRD_PARTY_NOTICES.md`.

---

## P1 — Required for a polished launch

### P1.1 Add OSS scaffolding files

Done in this PR:

- `LICENSE` (Apache-2.0)
- `SECURITY.md` (responsible disclosure)
- `CONTRIBUTING.md` (dev setup, test, commit norms)
- `CHANGELOG.md` (starts with the unreleased section)

### P1.2 Cut the first GitHub release

Repo must still be private at this step — `goreleaser` happily releases from
a private repo.

```bash
git tag v0.1.0
git push origin v0.1.0           # only after the repo is approved for public
# CI workflow at .github/workflows/release.yml runs goreleaser
```

Then verify `https://api.github.com/repos/Avanderheyde/self-stack/releases/latest`
returns HTTP 200 and the `selfstack_{darwin,linux}_{amd64,arm64}.tar.gz`
assets are downloadable.

Until the release exists, `scripts/install.sh` fails fast with the 3-option
remediation message (build from source, set `SELFSTACK_REPO`, or pin
`SELFSTACK_VERSION`).

### P1.3 Wire `selfstack.dev/install`

Either:
- Stand up the redirect (`selfstack.dev/install` → raw install.sh on the
  release branch), **or**
- Update README to drop the friendly URL in favor of the raw GitHub URL
  that already works.

The cloud-init in `internal/cloud/hetzner.go` already prefers
`SELFSTACK_INSTALL_URL`, so this only affects the marketing copy.

### P1.4 Live proof bundle

Run `docs/runbooks/live-demo.md` end-to-end at least once, capture the
median timings of three deploys, and replace the "design target"
disclaimers in `README.md` and `docs/mvp-demo.md` with measured numbers.

---

## P2 — Polish (can ship hot)

- README badges (build, latest release, license).
- `docs/architecture.md` extracted from the design plans.
- Discussions enabled, issue templates added.
- `THIRD_PARTY_NOTICES.md` if Apache-2.0 dependencies need attribution.

---

## Gate: ready-to-publish checklist

Run all of these locally; every line must be ✓ before clicking "make public".

```text
[ ] scripts/secret-audit.sh exits 0 on a fresh clone
[ ] LICENSE present and committed
[ ] SECURITY.md present with a working contact address
[ ] CONTRIBUTING.md present
[ ] CHANGELOG.md has an entry for the first tag
[ ] README install snippet is verified against a clean machine
[ ] go test ./... passes
[ ] SELFSTACK_INTEGRATION=1 go test -tags integration ./test passes
[ ] At least one live deploy is documented in docs/mvp-demo.md
[ ] The leaked Anthropic key is revoked in the Anthropic console
```

Only Alderik can mark the last item complete (external system).
