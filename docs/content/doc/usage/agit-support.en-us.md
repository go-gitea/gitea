---
date: "	2022-09-01T20:50:42+0000"
title: "Usage: Agit Setup"
slug: "agit-setup"
weight: 12
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Agit Setup"
    weight: 12
    identifier: "agit-setup"
---

# Agit Setup

In Gitea `1.13`, support for [agit](https://git-repo.info/en/2020/03/agit-flow-and-git-repo/) was added.

## Creating PR with Agit
Agit allows creating a PR while pushing code to the remote repo. This requires using a speacial command refspec.

- `HEAD`

  Target branch

- `refs/<for|drafts|for-review>/`

  Target PR Type.
  * `for` - Normal PR
  * `draft` - Draft PR
  * `for-review` - Generate a PR ID for updating existing PR.

- `<target-branch>/<session>`

  Target remote branch to open a PR.

- `-o <topic|title|description>`

  Options for the PR
  * `title` - Title of the PR.
  * `topic` - Topic of the PR.
  * `description` - Description of the PR. (Contents in Markdown format)

## Examples

Example of pushing a repo with a PR:

```shell
git push origin HEAD:refs/for/master
```

Example of pushing a repo with a PR `topic`, `title` and `description`:

```shell
git push origin HEAD:refs/for/master -o topic="Topic of my PR" -o title="Title of the PR" -o description="# The PR Description\nThis can be markdown formatted.\n[x] Ok"
```