---
date: "2020-07-06T16:00:00+02:00"
title: "Usage: Push Options"
slug: "push-options"
weight: 15
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Push Options"
    weight: 15
    identifier: "push-options"
---

# Push Options

In Gitea `1.13`, support for some [push options](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt)
were added.

## Supported Options

- `repo.private` (true|false) - Change the repository's visibility.

  This is particularly useful when combined with push-to-create.

- `repo.template` (true|false) - Change whether the repository is a template.

Example of changing a repository's visibility to public:

```shell
git push -o repo.private=false -u origin master
```
