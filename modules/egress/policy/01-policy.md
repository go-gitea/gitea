---
status: "accepted"
date: 2026-10-24
decision-makers: thefox0x7
---

# Policy module

## Context and Problem Statement

Gitea needs a centralized secure way to connect to external services that is clear to understand
and straightforward to use properly to secure against SSRF attacks.
The prior hostmatcher based approach was mixing two approaches (allow and block) in a single list,
making it difficult to interpret without running the code.

## Decision Drivers

- High volume of reports about SSRF bypasses
- Difficultly to understand the existing hostmatcher as it could be interpreted in two ways
- Hostmatching was using a filepath library which could introduce attack vectors if misconfigured

## Decision outcome

A consolidated struct which hides the inner lists ensuring it's harder to make errors in usage,
and expands the feature to enable fuller lockdown via modes.
Using two lists instead of one which help readers understand which context the list is used at the time,
while the backing implementation remains the same for both.

Matching hosts is done via string comparison, using curl's NO_PROXY behavior as a reference.

Proxy remains a non-target for the module, and it serves as a bypass of it in case the administrator wants to allow
some more specific addresses or has knowledge on how to scan an embedding range.

