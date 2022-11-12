---
date: "2020-07-06T16:00:00+02:00"
title: "使用: Push Options"
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

Gitea 從 `1.13` 版開始支援某些 [push options](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt)
。

## 支援的 Options

- `repo.private` (true|false) - 修改儲存庫的可見性。

  與 push-to-create 一起使用時特別有用。

- `repo.template` (true|false) - 修改儲存庫是否為範本儲存庫。

以下範例修改儲存庫的可見性為公開：

```shell
git push -o repo.private=false -u origin master
```
