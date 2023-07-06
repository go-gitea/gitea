---
date: "2023-07-06T12:30:00+05:00"
title: "Gitea Hardening"
slug: "hardening"
weight: 120
toc: false
draft: false
aliases:
  - /en-us/hardening
menu:
  sidebar:
    parent: "administration"
    name: "Gitea hardening"
    weight: 120
    identifier: "hardening"
---

# Gitea hardening

**Table of Contents**

{{< toc >}}

In a multi-user environment, or where Gitea is exposed to the internet, it may be desirable to harden your installation.

## Block anonymous users

To prevent anonymous users from accessing your Gitea server:

```ini
[service]
REQUIRE_SIGNIN_VIEW = true
```

## Block anonymous registrations

To prevent anonymous users from registering themselves on your Gitea server:

```ini
[service]
DISABLE_REGISTRATION     = true
SHOW_REGISTRATION_BUTTON = false
```

In such a case, users must be registered manually by a Gitea administrator, or via the Gitea API.

## Harden Gitea server, but allow access to CI/CD server

If Gitea is integrated with a CI/CD server, and both are exposed to the Internet, then additional steps must be taken:

- Update your config

  ```ini
  [service]
  REQUIRE_SIGNIN_VIEW      = true
  DISABLE_REGISTRATION     = true
  SHOW_REGISTRATION_BUTTON = false  
  [repository]
  DISABLE_HTTP_GIT         = false
  ```

- Create an [OAuth2 application]({{< relref "doc/development/oauth2-provider.en-us.md" >}}) for your CI/CD server.
- Ensure that you have applied appropriate access control to your repos, i.e. they are set to "limited" or "private".

That will ensure that:

- Manually registered users can access your Gitea server and clone its repos (via HTTPS or SSH), and
- Anonymous users cannot access your Gitea server, and
- Anonymous users cannot clone your repos (even the public ones) as they lack Gitea accounts, and
- Your CI/CD server can clone your repos (via HTTPS), as it has an OAuth2 application token.
