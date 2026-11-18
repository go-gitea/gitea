---
status: "accepted"
date: 2026-10-24
decision-makers: thefox0x7
---

# Policy module

## Context and Problem Statement

The prior hostmatcher mixed allow and block rules in a single list, making it hard to interpret without running the code.
See [design.md](design.md) for the full context and the current behavior.

## Decision Drivers

- High volume of reports about SSRF bypasses
- Difficulty to understand the existing hostmatcher as it could be interpreted in two ways
- Hostmatching was using a filepath library which could introduce attack vectors if misconfigured

## Decision Outcome

A consolidated `Policy` struct which hides the inner lists ensuring it's harder to make errors in usage,
and expands the feature to enable fuller lockdown via modes.
Using two lists instead of one which help readers understand which context the list is used at the time,
while the backing implementation remains the same for both.

Matching hosts is done via string comparison, using curl's NO_PROXY behavior as a reference.

Proxies are operator configuration, not user input: the dial to the proxy is unchecked, while every direct
dial is screened — including a direct request whose target is the proxy's own address. Proxied requests
are not IP-checked locally (the proxy resolves the target); see [design.md](design.md#proxy).

## Consequences

- Positive: one struct per use case with the lists hidden behind options, dial-time enforcement that DNS rebinding cannot bypass, and per-flow settings (see the enforcement map in [design.md](design.md#where-the-policy-is-enforced)).
- Negative: operators must understand the mode plus the allowlist and blocklist per flow; restricted targets fail closed until explicitly allowed, which breaks existing internal webhooks/migrations/remotes on upgrade until they are allow-listed.
