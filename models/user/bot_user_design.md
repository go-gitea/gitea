# Bot User Design

This document describes the design of **bot accounts** (`UserTypeBot`) in Gitea:
what they are, what they can and cannot do, and how they are managed. It exists to
clarify the model before the surface area grows, as requested in
[#38181](https://github.com/go-gitea/gitea/pull/38181) and
[#33758](https://github.com/go-gitea/gitea/pull/33758#issuecomment-2692128102).

## Definition

A bot account is a **local** user with `Type == UserTypeBot` (see `user.go`). It is
intended for automation that authenticates via **access tokens** (and the API),
not for interactive use by a human. A bot has **no password** and is **not** linked
to any external authentication source.

This is distinct from the built-in *system* bot users such as the Actions user
(`NewActionsUser`, ID `-2`), which are internal and not created by admins.

## Interactive sign-in: never allowed

A bot must never obtain an interactive (session) login on any authentication path.
The matrix below reflects the enforced behaviour:

| Auth path | Bot allowed? | Enforced at |
|-----------|--------------|-------------|
| Local password | No | `UserSignIn` → `GetUserByName`/`GetIndividualUser` (`models/user/user.go`) |
| External fallback (LDAP / SMTP / PAM) | No | `UserSignIn` fallback loop checks `IsIndividual()` (`services/auth/signin.go`) |
| OAuth2 / OpenID Connect | No | callback checks `IsIndividual()` (`routers/web/auth/oauth.go`) |
| Reverse proxy (user header) | No | `getUserFromAuthUser` checks `IsIndividual()` (`services/auth/reverseproxy.go`) |
| Reverse proxy (email header) | No | `getUserFromAuthEmail` checks `IsIndividual()` (`services/auth/reverseproxy.go`) |

The LDAP/SMTP/PAM fallback and reverse-proxy guards were previously missing: a bot
whose name (or email) matched an external/proxy identity could be returned for a
session. This was closed alongside this design (with regression tests in
`services/auth/signin_test.go` and `services/auth/reverseproxy_test.go`).

Auto-registration paths (reverse-proxy auto-register, LDAP/SMTP first-login user
creation) always create **individual** users, so they cannot mint bots.

## Capabilities

| Capability | Bot | Reference |
|------------|-----|-----------|
| Password | none | bots are created without a password |
| Interactive sign-in | no | see matrix above |
| Access tokens | yes | `IsTokenAccessAllowed()` (`models/user/user.go`) |
| API / Git over token | yes | token auth applies to individuals and bots |
| OAuth2 application links | no | external links require `IsIndividual()` |
| Repo / org ownership | not restricted at model level today | documented as future work, not part of this design |

### Open question: email

`IsMailable()` currently does not exclude bots, so a bot with a valid address could
receive notification email. Bots are non-interactive, so this is likely undesirable;
it is called out here as a known open question rather than changed in this iteration.

## Converting between individual and bot

A site admin can convert an existing account between **individual** and **bot** via:

- the admin *Edit User* page (the *User Type* dropdown);
- the API: `POST /admin/users/{username}/convert-type` with `{"user_type": "bot"|"individual"}`;
- the CLI: `gitea admin user change-type --username <name> --user-type bot|individual`.

All three call the same `ConvertUserType` (`services/user/update.go`).

**Only individual ↔ bot is allowed.** Organizations and reserved types cannot be
converted, and there is no individual ↔ organization conversion. (An earlier attempt,
[#33758](https://github.com/go-gitea/gitea/pull/33758), allowed converting *any* type
into *any* other — including org → individual — and was rejected for being unclear and
unsafe. This design deliberately restricts the matrix and spells out every side effect
below.)

### Individual → Bot

The account becomes a non-interactive, local, token-only account. The table makes the
fate of every credential / auth artifact explicit (the exact question raised on #33758):

| Artifact | Result | Why |
|----------|--------|-----|
| Password (`passwd`/`salt`/`passwd_hash_algo`) | **cleared** | a bot has no interactive login |
| `must_change_password` | reset to false | no password to change |
| Auth source (`login_type`/`login_source`/`login_name`) | reset to **local** | bots are local accounts, never externally synced |
| Sign-in sessions (`auth_token`) | **revoked** | the former individual must not stay logged in |
| Access tokens (`access_token`) | **kept** | they are the entire purpose of a bot |
| OAuth2 applications + grants | **removed** (`DeleteOAuth2RelictsByUserID`) | a token-only account cannot run OAuth2 flows |
| External login links (OAuth2/LDAP/...) | **removed** (`RemoveAllAccountLinks`) | the account is now local |
| Repositories, org membership, issues, other owned content | **unchanged** | only the account's type/credentials change |

### Bot → Individual

The type is switched back. The account has no password (bots never have one), so the
admin must set a password on the same *Edit User* form for the account to be usable for
interactive sign-in. No content is removed.

## Token management

Bots cannot generate their own tokens (no interactive login). Therefore a
**site administrator** manages a bot's access tokens from the admin user view:

- `POST /-/admin/users/{userid}/access_tokens` — create a scoped token
- `POST /-/admin/users/{userid}/access_tokens/delete` — delete a token

Both routes are guarded to act only on `UserTypeBot` accounts.

### Scope of this design: admin-only

Management is intentionally limited to **site admins** in this iteration.
Organization-admin-managed bots (so org admins can run automation without involving
a site admin) is a recognised request but is **out of scope** here: it expands the
permission and ownership model considerably (who owns the bot, who can rotate its
tokens, visibility across orgs) and deserves its own design. Keeping the first
iteration admin-only bounds the blast radius and can be layered on later without
breaking this model.
