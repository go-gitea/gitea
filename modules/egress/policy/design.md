---
updated: 2026-10-24
owners: thefox0x7
scope: `@modules/egress/policy`, `@modules/egress/`
---

# Policy module design and decisions

This document is meant to be an overview of what the policy module aims to do, how and why.
It also should record all decisions regarding it without polluting global space.

The structure of the document is based on [MADR](https://github.com/adr/madr) templates. This is not an ADR,
but a document that serves as an explanation of the design and decisions meant to be consulted
if the aim of the function is not clear.

## Context and problem statement

Gitea needs a centralized secure way to connect to external services that is clear to understand and straightforward
to use properly to secure against SSRF attacks.
The prior hostmatcher based approach was mixing two approaches (allow and block) in a single list,
making it difficult to interpret without running the code, leading to unclear behaviors.

Policy module is designed as a replacement taking the parts hostmatcher did correctly and improving on the capabilities,
readability and flow.

# Contract

An instance of policy is capable of secure dialing of an external address according to configuration.
It also provides an interface to screen the remote address without dialing.
It is meant to be used as a precheck of an address; actual dialing should be used with the transport provided by policy.

## Address pools

Policy relies on classification of addresses into pools: reserved, restricted and public. All can be found in [pools.go](pools.go)

Reserved pool keeps addresses which are never allowed by the policy for various reasons like
unscannable embedded addresses or addresses which are not meant to be dialed like broadcast or Azure WireServer.
Notably, it also matches IPv6 with zone as it has no valid use in this context as far as the module author is aware.

Restricted pool keeps addresses which are meant to be private or not in public use.
It may contain possibly dangerous addresses as long as they have some valid use like link-local.
Notably, apart from the addresses listed in code, it also contains private, loopback, cgnat and link-local unicast addresses.

The remaining addresses are classified as public.

For details on the classifier, see classifyAddr function.

## The verification flow

The policy's primary check takes hostname and `netip.AddrPort` as input or infers them from `*url.URL` if possible,
and returns an error detailing why the address cannot be dialed if it cannot.

The overall flow of the check looks as follows:
1. The address is classified into a pool, rejected if the address is reserved.
2. The address and host are checked against the blocklist
3. The address and host are checked against the allowlist.

The policy mode modifies the flow. In Lax mode, addresses classified as public are not checked
against the allowlist as they are assumed to be allowed implicitly. Strict mode requires everything to be on the allowlist.

## Rules

Policy uses rules to make the decisions. Those vary in behaviors with modes.
Each rule consists of a pattern or the prefix as well as the port range it matches.

Rules made for the blocklist default to all ports, Lax also uses this behavior.
Strict mode defaults to 80 and 443. Any other port must be given explicitly.

## On hostname allows

Hostname-based allows are supported via another setting. Adding the range to `ALLOWED_INTERNAL_RANGES`
does not allow the address to be dialed directly, but it allows it to be dialed via hostnames.
This is done to safeguard a module from an allowed domain (or subdomain) being hijacked and used to bypass safeguards,
using the domain to dial a malicious address. This only unlocks restricted addresses.

While it might be more confusing, it is the overall best approach.
See [01-hostname-allows.md](01-hostname-allows.md) for why this solution was chosen.

## Modes

Policy has two modes of operation. Lax which allows all public addresses without needing to be on the allowlist,
and strict which requires everything to be on the allowlist.
Denylist works the same in both modes taking precedence.
Lax policy also assumes all ports are allowed unless specified otherwise.

## Host matching

Host matching is based on TLS rules - `example.com` matches only itself,
`*.example.com` matches all subdomains of example.com.
This is done to avoid confusion and to make it clear what is allowed, while supporting fine-grained control.
Blank allows like `*.` or `**` are not supported by design as they would negate the purpose of the module.
