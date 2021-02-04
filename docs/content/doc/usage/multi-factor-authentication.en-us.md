---
date: "2021-02-04T18:00:00+00:00"
title: "Usage: Multi-factor Authentication (MFA)"
slug: "multi-factor-authentication"
weight: 15
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Multi-factor Authentication (MFA)"
    weight: 15
    identifier: "multi-factor-authentication"
---

# Multi-factor Authentication (MFA)

Multi-factor Authentication (also referred to as MFA or 2FA) enhances security by requiring a time-sensitive set of credentials in addition to a password.
If a password were later to be compromised, Gitea would still not allow a successful login and the account would remain secure.
Gitea supports both TOTP (Time-based One-Time Password) tokens and FIDO-based hardware keys.

MFA can be configured within the "Security" tab of the user settings page.

## Using MFA

Enabling MFA on a user does affect how the Git HTTP protocol and the Gitea API can be used.
These interfaces do not support MFA, and trying to use a password normally will no longer be possible whilst MFA is enabled.
However, an access token can be generated within the "Applications" tab of the user settings page.
This access token can be used as if it were a password in order to use these interfaces.

> **Warning** - By its very nature, an access token sidesteps the security benefits of MFA.
> It must be kept secure and should only be used as a last resort.

Using Git over SSH is separate to the normal authentication process and will still function normally.
