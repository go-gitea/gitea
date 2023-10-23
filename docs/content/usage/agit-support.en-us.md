---
date: "	2022-09-01T20:50:42+0000"
title: "Agit Setup"
slug: "agit-setup"
sidebar_position: 12
toc: false
draft: false
aliases:
  - /en-us/agit-setup
menu:
  sidebar:
    parent: "usage"
    name: "Agit Setup"
    sidebar_position: 12
    identifier: "agit-setup"
---

# Agit Setup

In Gitea `1.13`, support for [agit](https://git-repo.info/en/2020/03/agit-flow-and-git-repo/) was added.
**Note**: git version 2.29 or higher is required on the server side for this to work.

## Creating PRs with Agit

Agit allows to create PRs while pushing code to the remote repo.
This can be done by pushing to the branch followed by a specific refspec (a location identifier known to git).
The following example illustrates this:

```shell
git push origin HEAD:refs/for/main
```

The command has the following structure:

- `HEAD`: The target branch
- `origin`: The target repository (not a fork!)
- `HEAD`: The local branch containing the changes you are proposing
- `refs/<for|draft|for-review>/<branch>`: The target PR type and configuration
  - `for`: Create a normal PR with `<branch>` as the target branch
  - `draft`/`for-review`: Currently ignored silently
  - `<branch>/`: The branch you want your changes to be merged into
- `-o <topic|title|description>`: Options for the PR
  - `topic`: The topic of this change. It will become the name of the branch holding the changes waiting for review.  This is REQUIRED to trigger a pull request.
  - `title`: The PR title (optional but recommended), only used for topics not already having an associated PR.
  - `description`: The PR description (optional but recommended), only used for topics not already having an associated PR.
  - `force-push`: confirm force update the target branch

Here's another advanced example for creating a new PR targeting `main` with `topic`, `title`, and `description`:

```shell
git push origin HEAD:refs/for/main -o topic="Topic of my PR" -o title="Title of the PR" -o description="# The PR Description\nThis can be **any** markdown content.\n- [x] Ok"
```
