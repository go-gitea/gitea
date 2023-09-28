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

## Creating PRs with Agit

Agit allows to create PRs while pushing code to the remote repo.
This can be done by pushing to the branch followed by a specific refspec (a location identifier known to git).
The following example illustrates this:

```shell
git push origin HEAD:refs/for/master
```

The command has the following structure:

- `HEAD`: The target branch
- `refs/<for|draft|for-review>/<branch>`: The target PR type
  - `for`: Create a normal PR with `<branch>` as the target branch
  - `draft`/ `for-review`: Currently ignored silently
- `<branch>/<session>`: The target branch to open the PR
- `-o <topic|title|description>`: Options for the PR
  - `title`: The PR title
  - `topic`: The branch name the PR should be opened for
  - `description`: The PR description
  - `force-push`: confirm force update the target branch

Here's another advanced example for creating a new PR targeting `master` with `topic`, `title`, and `description`:

```shell
git push origin HEAD:refs/for/master -o topic="Topic of my PR" -o title="Title of the PR" -o description="# The PR Description\nThis can be **any** markdown content.\n- [x] Ok"
```
