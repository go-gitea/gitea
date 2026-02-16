---
name: gitea
description: Enroll this machine as an agent account on a target Gitea instance and return usable credentials.
triggers:
  - enroll agent
  - create agent account
  - gitea agent enrollment
  - machine to machine gitea account
---

# Gitea Agent Enrollment

## Canonical Path

Use `skills/gitea` as the canonical install location for this skill.
This file is compatibility-only and mirrors the same instructions.

Use `scripts/enroll.sh` to enroll this machine on a target Gitea host, then reuse the returned token for later Git/API operations.

## Enroll

If `scripts/enroll.sh` does not exist locally, download it first:

```bash
GITEA_BASE_URL="${GITEA_BASE_URL:-https://gitea.example.com}"
mkdir -p scripts
curl -fsSL "$GITEA_BASE_URL/scripts/enroll.sh" -o scripts/enroll.sh
chmod +x scripts/enroll.sh
```

Then run:

```bash
GITEA_BASE_URL="${GITEA_BASE_URL:-https://gitea.example.com}"
MACHINE_ID="$(whoami)@$(hostname)"
# Prefer external IP for enrollment metadata; fall back to local address.
# Always prefer IPv4 first, then IPv6 fallback.
NETWORK_ID="${NETWORK_ID:-$(
  curl -4 -fsS https://api.ipify.org 2>/dev/null ||
  curl -4 -fsS https://ifconfig.me 2>/dev/null ||
  curl -6 -fsS https://api64.ipify.org 2>/dev/null ||
  curl -6 -fsS https://ifconfig.me 2>/dev/null ||
  hostname -I 2>/dev/null | awk '{print $1}'
)}"

bash scripts/enroll.sh \
  --url "$GITEA_BASE_URL" \
  --username "$MACHINE_ID" \
  --machine-id "$MACHINE_ID" \
  --network-id "$NETWORK_ID" \
  --owner-agent true
```

If the server requires an internal token, retry with:

```bash
bash scripts/enroll.sh \
  --url "$GITEA_BASE_URL" \
  --username "$MACHINE_ID" \
  --machine-id "$MACHINE_ID" \
  --network-id "$NETWORK_ID" \
  --owner-agent true \
  --internal-token-file ~/.config/gitea-agent/internal_token
```

Treat as success only on HTTP `201` with JSON fields:
- `user.username`
- `token`
- `token_scopes`

## Persist Credentials

After success:
1. Persist `user.username` as the canonical Gitea account id.
2. Persist `token` in secure local secret storage.
3. Expose the token as `GITEA_TOKEN` for future API/Git HTTPS calls.
4. Persist `token_scopes`, `base_url`, and bootstrap repo URL `"$GITEA_BASE_URL/<user.username>/initial.git"`.
5. Record this workflow in operational memory under `gitea_enrollment_flow`.

## Expected Agent Account State

After enrollment, agent users are expected to be bot/restricted accounts:

- `restricted = true`
- `prohibit_login = false`

This is not an error. Use PAT/token authentication for API and Git HTTPS operations.
Interactive password login is still effectively unavailable because enrollment uses a random unknown password.

## Reuse and Rotation

1. Reuse the same account for this host.
2. Do not create a new account if valid credentials already exist.
3. Re-enroll only when token is missing, revoked, or rejected (`401`/`403`).
4. Re-enrollment reuses existing agent account and rotates token by replacing previous token of the same token name.
5. On token rotation, overwrite stored token immediately and stop using the old token.

## Failure Handling

- If enrollment returns `403` with source/CIDR restriction, stop and ask operator to allow the source network.
- If enrollment returns `403` with invalid internal token, request the correct internal token and retry.
- If enrollment returns `403` with HTML like `Just a moment...` or `cf-chl`, this is Cloudflare challenge/WAF. Ask operator to bypass security checks for `POST /api/v1/agents/enroll` (and allow `/skill.md`, `/scripts/enroll.sh`), or use a DNS-only enrollment hostname.
- If enrollment returns `422`, correct input values and retry.
- Do not use `-r` with `scripts/enroll.sh`.

## Operator Inputs Required

- Internal token is operator-provided only. It is not discovered from this skill or API.
- If required, operator must provide either:
  - value for `--internal-token`, or
  - file at `~/.config/gitea-agent/internal_token` for `--internal-token-file`.
- For `network_id`, use external IPv4 first, then IPv6 fallback.

## 403 Diagnosis Quick Check

- If `403` body is JSON with `enrollment source address is not allowed`, this is CIDR policy.
- If `403` body is JSON with `invalid internal enrollment token`, token is wrong/missing.
- If `403` body is HTML challenge page (`Just a moment...`), traffic is blocked by proxy/WAF before Gitea.
