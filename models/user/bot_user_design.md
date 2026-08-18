# Bot User Design

This document describes the design of **bot accounts** (`UserTypeBot`) in Gitea:
what they are, what they can and cannot do, and how they are created, converted and
managed. It exists to bound the model before the surface area grows, as requested
during review of the bot-account work.

## Definition

A bot account is a **local** user with `Type == UserTypeBot` (see
`models/user/user.go`). It is intended for automation that authenticates with
**access tokens** over the API and Git, never for interactive use by a human. A bot
has **no password** and is **not** linked to any external authentication source.

A bot is deliberately modelled as a `User` row (rather than a separate table) so that
it can own the same artifacts a human can — repositories, organization memberships,
issues, comments, access tokens — without duplicating every relationship. The type
field is what separates it from a human account; everything below defines the
consequences of that type.

This is distinct from the internal *system* users that also carry `UserTypeBot`, such
as the Actions user (`NewActionsUser`, ID `ActionsUserID == -2`, recognised by
`IsGiteaActions()`). Those are synthesized in code, never stored as admin-managed
accounts, and are out of scope here.

## Interactive sign-in: never allowed

A bot must never obtain an interactive (session) login on any authentication path.
Every path that resolves a user for a session either requires `IsIndividual()` or
cannot resolve a bot in the first place. The matrix reflects the **enforced** behaviour
in the current tree, not an aspiration:

| Auth path | Bot allowed? | Enforced at |
|-----------|--------------|-------------|
| Local password | No | `GetIndividualUserByName` + explicit `Type != UserTypeIndividual` guard (`services/auth/signin.go`) |
| External source fallback (LDAP / SMTP / PAM) | No | fallback loop checks `!IsIndividual()` (`services/auth/signin.go`) |
| OAuth2 / OIDC | No | external-login resolution requires `IsIndividual()`, stale non-individual links are dropped (`routers/web/auth/oauth.go`) |
| OpenID | No | a bot holds no OpenID identity — none is created for it, and conversion deletes any existing one (see below) |
| Reverse proxy (user header) | No | `getUserFromAuthUser` checks `IsIndividual()` (`services/auth/reverseproxy.go`) |
| Reverse proxy (email header) | No | `getUserFromAuthEmail` checks `IsIndividual()` (`services/auth/reverseproxy.go`) |
| SSPI (Windows) | No | `SSPI.Verify` checks `IsIndividual()` (`services/auth/sspi.go`) |
| Existing session cookie | No | `Session.Verify` checks `IsIndividual()` (`services/auth/session.go`) |

The external-source fallback, reverse-proxy, SSPI and session guards were previously
missing: a bot whose name (or email) matched an external, proxy or domain identity
could be handed back for a session, and a session opened before an account was
converted to a bot would outlive the conversion. These are closed with regression
tests in `services/auth/signin_test.go`, `services/auth/reverseproxy_test.go` and
`services/auth/session_test.go`.

The session store is keyed by session ID only and cannot be enumerated per user, so
open sessions cannot be revoked directly at conversion time. `Session.Verify` instead
rejects any non-individual on the next request, which makes the caller drop the
session — the same effect, evaluated lazily.

Auto-registration paths (reverse-proxy auto-register, LDAP/SMTP first-login user
creation) always create **individual** users, so they cannot mint bots.

## Capabilities

| Capability | Bot | Reference |
|------------|-----|-----------|
| Password | none | bots are created without a password |
| Interactive sign-in | no | see the matrix above |
| Site administrator | no | `ErrBotCanNotBeAdmin`, enforced in `UpdateUser` / `ConvertUserType` and `admin user create` |
| Access tokens | yes | `IsTokenAccessAllowed()` (`models/user/user.go`) |
| API / Git over token | yes | token auth applies to individuals and bots alike |
| Impersonation by an admin | no | non-interactive accounts cannot be impersonated (`routers/web/admin/users.go`) |
| OAuth2 application links | no | external links require `IsIndividual()` |
| Repos / org membership / issues / other owned content | unchanged from an individual | only the account type and its credentials differ |

### Known open question: email

`IsMailable()` currently excludes only the Actions user and the ghost user, not
regular bots, so a bot with a valid, active address could still receive notification
email. Bots are non-interactive, so this is likely undesirable, but it is called out
here as a known open question rather than changed in this iteration.

## Converting between individual and bot

A site admin can convert an existing account between **individual** and **bot** via:

- the admin *Edit User* page (the danger zone at the bottom);
- the API: `POST /admin/users/{username}/convert-type` with `{"user_type": "bot"|"individual"}`;
- the CLI: `gitea admin user change-type --username <name> --user-type bot|individual`.

All three funnel through `ConvertUserType` (`services/user/update.go`), guarded by
`CheckConvertUserType`. The whole type flip plus credential teardown runs inside a
single `db.WithTx`, so a mid-sequence failure leaves the account fully intact rather
than half-converted; on failure the caller's in-memory `*User` is left unmodified.

Two accounts are off limits:

- **Site administrators**, because automation does not need site-wide root access
  (`ErrBotCanNotBeAdmin`). The admin permission has to be removed first, which is a
  deliberate second step rather than a silent side effect of the conversion.
- **Yourself** (web and API), because converting your own account clears your
  credentials and drops your session. Both entry points reject `doer == target`.

**Only individual ↔ bot is allowed.** Organizations, reserved and remote types cannot
be converted, and there is no individual ↔ organization conversion. An earlier attempt
allowed converting *any* type into *any* other — including organization → individual —
and was rejected as unclear and unsafe. This design deliberately restricts the matrix
and spells out every side effect below.

### Individual → Bot

The account becomes a non-interactive, local, token-only account. The table makes the
fate of every credential and auth artifact explicit:

| Artifact | Result | Why |
|----------|--------|-----|
| Password (`passwd` / `salt` / `passwd_hash_algo`) | **cleared** | a bot has no interactive login |
| `must_change_password` | reset to false | there is no password to change |
| Auth source (`login_type` / `login_source` / `login_name`) | reset to **local** | bots are local accounts, never externally synced |
| Remember-me / auth tokens | **revoked** (`DeleteAuthTokensByUserID`) | the former individual must not stay logged in |
| Open browser sessions | **rejected on next request** | see the session note above |
| Access tokens | **kept** | they are the entire purpose of a bot |
| OAuth2 applications + grants | **removed** (`DeleteOAuth2RelictsByUserID`) | a token-only account cannot run OAuth2 flows |
| External login links (OAuth2 / LDAP / …) | **removed** (`RemoveAllAccountLinks`) | the account is now local |
| OpenID identities (`user_open_id`) | **removed** | an OpenID URI can sign the account in, so it must not survive |
| Notifications | **deleted** | a bot has no inbox and cannot read them; this is irreversible |
| Repositories, org membership, issues, other owned content | **unchanged** | only the account type and credentials change |

Notifications are deleted rather than kept because a bot has no way to read or clear
them; this is intentional but irreversible, and is the one destructive step that
removes user-visible data rather than credentials.

### Bot → Individual

The type is switched back and nothing is removed. The account still has no password
(bots never have one), so the admin must set one on the same *Edit User* form before
the account can sign in interactively. The email is retained across both directions —
bots are required to have one at creation — so no field needs to be re-entered.

## Token management

Bots cannot generate their own tokens (no interactive login), so a **site
administrator** manages a bot's access tokens from the admin user view:

- `POST /-/admin/users/{userid}/access_tokens` — create a scoped token;
- `POST /-/admin/users/{userid}/access_tokens/delete` — delete a token.

Both routes act only on `UserTypeBot` accounts. The `admin` scope category is filtered
out of the token form, because a bot can never be a site administrator and such a token
could never pass `reqSiteAdmin`.

### Scope of this design: admin-only

Management is intentionally limited to **site admins** in this iteration.
Organization-admin-managed bots — so org admins can run automation without involving a
site admin — is a recognised request but is **out of scope** here: it expands the
permission and ownership model considerably (who owns the bot, who may rotate its
tokens, how it is scoped and made visible across orgs) and deserves its own design.
Keeping the first iteration admin-only bounds the blast radius and can be layered on
later without breaking this model. Whether bots should own repositories on their own is
likewise left open.
