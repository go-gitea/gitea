---
date: "2019-09-06T01:35:00-03:00"
title: "Protected tags"
slug: "protected-tags"
weight: 45
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Protected tags"
    weight: 45
    identifier: "protected-tags"
---

# Protected tags

Protected tags allow control over who has permission to create or update git tags. Each rule allows you to match either an individual tag name, or use wildcards to control multiple tags at once. 

**Table of Contents**

{{< toc >}}

## Setting up protected tags

To protect a tag, you need to follow these steps:

1. Go to the repository’s **Settings** > **Tags** page.
2. Type the name of specific tag or use a pattern to match multiple tags at once.
3. Choose the allowed users and/or teams. If you leave these fields empty noone is allowed to create or modify this tag.
4. Select **Save** to save the configuration.

## Wildcard protected tags

You can specify a wildcard protected tag, which protects all tags matching the wildcard. For example:

| Wildcard Protected Tag | Matching Tags                           |
| ---------------------- | --------------------------------------- |
| `v*`                   | `v`, `v-1`, `version2`                  |
| `v[0-9]`               | `v0`, `v1` up to `v9`                   |
| `*-release`            | `2.1-release`, `final-release`          |
| `*gitea*`              | `gitea`, `2.1-gitea`, `1_gitea-release` |
| `{v,rel}-*`            | `v-`, `v-1`, `v-final`, `rel-`, `rel-x` |
| `*`                    | matches all possible tag names          |

See [github.com/gobwas/glob](https://pkg.go.dev/github.com/gobwas/glob#Compile) documentation for syntax.
