---
date: "2021-05-14T00:00:00-00:00"
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

Protected tags allow control over who has permission to create or update git tags. Each rule allows you to match either an individual tag name, or use an appropriate pattern to control multiple tags at once. 

**Table of Contents**

{{< toc >}}

## Setting up protected tags

To protect a tag, you need to follow these steps:

1. Go to the repository’s **Settings** > **Tags** page.
1. Type a regular expression pattern to match a name.
1. Choose the allowed users and/or teams. If you leave these fields empty noone is allowed to create or modify this tag.
1. Select **Save** to save the configuration.

## Pattern protected tags

The pattern uses regular expressions to match a tag name. Examples:

| Pattern Protected Tag  | Possible Matching Tags                  |
| ---------------------- | --------------------------------------- |
| `\Av`                  | `v`, `v-1`, `version2`                  |
| `\Av[0-9]\z`           | `v0`, `v1` up to `v9`                   |
| `\Av\d+\.\d+\.\d+\z`   | `v1.0.17`, `v2.1.0`                     |
| `\Av\d+(\.\d+){0,2}\z` | `v1`, `v2.1`, `v1.2.34`                 |
| `-release\z`           | `2.1-release`, `final-release`          |
| `gitea`                | `gitea`, `2.1-gitea`, `1_gitea-release` |
| `\Agitea\z`            | only `gitea`                            |
| `^gitea$`              | only `gitea`                            |
| `\A(v\|rel)-`           | `v-`, `v-1`, `v-final`, `rel-`, `rel-x` |
| `.+`                   | matches all possible tag names          |
