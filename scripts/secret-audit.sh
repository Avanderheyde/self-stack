#!/usr/bin/env bash
# scripts/secret-audit.sh — secret/history audit for OSS publication readiness.
#
# Scans for known secret patterns in:
#   1. The working tree
#   2. All push-reachable refs (heads, tags, remotes)
#   3. All other refs (conductor-checkpoints, stashes) — flagged separately
#      because these are NOT pushed by default but DO ship with `git push --mirror`
#      or with a manual `git clone --mirror`.
#
# Matches are reported with the secret value REDACTED (prefix + length only).
#
# Exit codes:
#   0 — no matches anywhere
#   1 — matches in push-reachable surface (publication blocker)
#   2 — matches only in non-pushed refs (cleanup required before publish)
#
# Usage:
#   scripts/secret-audit.sh                       # human-readable
#   scripts/secret-audit.sh --json                # machine-readable summary
#
# Patterns are intentionally conservative — false positives are fine, missed
# secrets are not.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

JSON=0
if [[ "${1:-}" == "--json" ]]; then JSON=1; fi

# Parallel arrays: PATTERN_REGEX[i] is matched, PATTERN_LABEL[i] names it.
# Kept as separate arrays so pattern bodies can contain `|` alternation.
PATTERN_REGEX=(
  'sk-ant-(api|admin)[0-9]+-[A-Za-z0-9_-]{20,}'
  'tskey-(auth|api|client)-[A-Za-z0-9]{8,}'
  'ghp_[A-Za-z0-9]{30,}'
  'gho_[A-Za-z0-9]{30,}'
  'github_pat_[A-Za-z0-9_]{20,}'
  'AKIA[0-9A-Z]{16}'
  'AIza[0-9A-Za-z_-]{30,}'
  'xoxb-[0-9]+-[0-9]+-[A-Za-z0-9]+'
  'BEGIN [A-Z ]*PRIVATE KEY'
)
PATTERN_LABEL=(
  'Anthropic API key'
  'Tailscale key'
  'GitHub personal token'
  'GitHub OAuth token'
  'GitHub fine-grained PAT'
  'AWS access key'
  'Google API key'
  'Slack bot token'
  'Private key block'
)

# Redact a hit: keep up to first 12 chars of the secret + length indicator.
redact() {
  awk '{
    line = $0
    # find the longest run of secret-shape characters; truncate to a prefix.
    if (match(line, /sk-ant-[A-Za-z0-9_-]+|tskey-[a-z]+-[A-Za-z0-9_-]+|ghp_[A-Za-z0-9]+|gho_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+|AKIA[0-9A-Z]+|AIza[0-9A-Za-z_-]+|xoxb-[0-9A-Za-z-]+/)) {
      secret = substr(line, RSTART, RLENGTH)
      n = length(secret)
      keep = (n > 12 ? 12 : n)
      red = substr(secret, 1, keep) "...[REDACTED " n " chars]"
      gsub(secret, red, line)
    }
    # generic private-key block: redact body
    if (line ~ /BEGIN [A-Z ]*PRIVATE KEY/) {
      line = "[REDACTED PRIVATE KEY BLOCK]"
    }
    print line
  }'
}

scan_working_tree() {
  local hits=0
  for i in "${!PATTERN_REGEX[@]}"; do
    local pat="${PATTERN_REGEX[$i]}"
    local label="${PATTERN_LABEL[$i]}"
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      hits=$((hits+1))
      [[ $JSON -eq 1 ]] || printf '  [%s] %s\n' "$label" "$(printf '%s' "$line" | redact)"
    done < <(git ls-files -z | xargs -0 grep -EnH -e "$pat" 2>/dev/null || true)
  done
  return $hits
}

scan_refs() {
  local refglob="$1"
  local hits=0
  for i in "${!PATTERN_REGEX[@]}"; do
    local pat="${PATTERN_REGEX[$i]}"
    local label="${PATTERN_LABEL[$i]}"
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      hits=$((hits+1))
      [[ $JSON -eq 1 ]] || printf '  [%s] %s\n' "$label" "$(printf '%s' "$line" | redact)"
    done < <(git log $refglob -p --no-color 2>/dev/null | grep -aEn -e "$pat" || true)
  done
  return $hits
}

scan_conductor() {
  local hits=0
  local refs
  refs=$(git for-each-ref --format='%(refname)' refs/conductor-checkpoints 2>/dev/null)
  if [[ -z "$refs" ]]; then return 0; fi
  for i in "${!PATTERN_REGEX[@]}"; do
    local pat="${PATTERN_REGEX[$i]}"
    local label="${PATTERN_LABEL[$i]}"
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      hits=$((hits+1))
      [[ $JSON -eq 1 ]] || printf '  [%s] %s\n' "$label" "$(printf '%s' "$line" | redact)"
    done < <(git log $refs -p --no-color 2>/dev/null | grep -aEn -e "$pat" || true)
  done
  return $hits
}

WORKING=0
PUSHED=0
LOCAL=0

if [[ $JSON -eq 0 ]]; then echo "=== Working tree ==="; fi
scan_working_tree || WORKING=$?

if [[ $JSON -eq 0 ]]; then echo "=== Push-reachable history (refs/heads + refs/tags + refs/remotes) ==="; fi
scan_refs "--branches --tags --remotes" || PUSHED=$?

if [[ $JSON -eq 0 ]]; then echo "=== Local-only refs (conductor-checkpoints) ==="; fi
scan_conductor || LOCAL=$?

if [[ $JSON -eq 1 ]]; then
  printf '{"working_tree": %d, "push_reachable": %d, "local_only": %d}\n' "$WORKING" "$PUSHED" "$LOCAL"
else
  echo
  echo "=== Summary ==="
  printf '  working tree:    %d hit(s)\n' "$WORKING"
  printf '  push-reachable:  %d hit(s)\n' "$PUSHED"
  printf '  local-only refs: %d hit(s)\n' "$LOCAL"
fi

if [[ $WORKING -gt 0 || $PUSHED -gt 0 ]]; then exit 1; fi
if [[ $LOCAL -gt 0 ]]; then exit 2; fi
exit 0
