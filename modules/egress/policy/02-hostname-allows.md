---
status: "accepted"
date: 2026-10-24
decision-makers: thefox0x7
---

# Hostname-based allowlist

## Context and Problem Statement

Policy initially missed the ability to allowlist hosts by hostname only for the restricted pool.
This became an issue as users reported that they were unable to access certain services that had dynamic IP addresses
and were not able to be added to the allowlist by hostname only.

## Decision Drivers

- regression from the hostmatcher implementation
- valid and reasonable way of connecting to dynamically assigned hosts
- unreasonable expectation to allow entire range instead of a single host

## Considered Options

- Simple enable of hostname to match restricted class addresses
- Dedicated allow setting to unlock restricted addresses
- Allowlist syntax extension to allow hostnames in some specific IP range

## Decision Outcome

Chosen option: "Dedicated allow setting", because it makes the best tradeoff between convenience and security.

## Pros and Cons of the Options

### Simple enable of hostname

Have `ALLOWED_HOST_LIST = example.com` allow server to connect to example.com even if it is in a restricted pool.

* Good, because it doesn't require more configuration
* Good, because it is simpler to understand for the end user
* Bad, because it opens up the possibility of an attacker buying an expired domain
and pointing it at an address from a restricted pool, which would now be allowed if not on blocklist.
* Bad because there is no control over which IP can be allowed this way.

### Dedicated allow setting

Add a dedicated setting `ALLOWED_INTERNAL_RANGES = 10.0.0.0/8` which "unlocks" the restricted pool for the hostname-based
allowlist.

* Good, because it provides a way to allow a hostname in a specific range instead of opening the entire pool
* Neutral, because it requires more configuration
* Bad, because it is more complex to understand for the end user as it doesn't actually allow the addresses listed.
Only unlocks the pool for the allowlist.

### Allowlist syntax extension

Extend the syntax to support something like `ALLOWED_HOST_LIST = example.com in 10.0.0.0/8`

* Good, because it provides a way to allow a hostname in a specific range
* Neutral, because it requires more configuration
* Bad, because it is more complex to understand for the end user
* Bad, because it provides tiny benefits to overall security and it requires more code to implement
* Bad, because the ini format isn't the best for this kind of configuration

