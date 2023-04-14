---
date: "2022-12-19T21:26:00+08:00"
title: "Secrets"
slug: "usage/secrets"
weight: 50
draft: false
toc: false
menu:
  sidebar:
    parent: "usage"
    name: "Secrets"
    weight: 50
    identifier: "usage-secrets"
---

# Secrets

Secrets allow you to store sensitive information in your user, organization or repository.
Secrets are available on Gitea 1.19+.

# Naming your secrets

The following rules apply to secret names:

- Secret names can only contain alphanumeric characters (`[a-z]`, `[A-Z]`, `[0-9]`) or underscores (`_`). Spaces are not allowed.

- Secret names must not start with the `GITHUB_` and `GITEA_` prefix.

- Secret names must not start with a number.

- Secret names are not case-sensitive.

- Secret names must be unique at the level they are created at.

For example, a secret created at the repository level must have a unique name in that repository, and a secret created at the organization level must have a unique name at that level.

If a secret with the same name exists at multiple levels, the secret at the lowest level takes precedence. For example, if an organization-level secret has the same name as a repository-level secret, then the repository-level secret takes precedence.
