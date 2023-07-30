---
date: "2023-05-23T09:00:00+08:00"
title: "Agit 设置"
slug: "agit-setup"
sidebar_position: 12
toc: false
draft: false
aliases:
  - /zh-cn/agit-setup
menu:
  sidebar:
    parent: "usage"
    name: "Agit 设置"
    sidebar_position: 12
    identifier: "agit-setup"
---

# Agit 设置

在 Gitea `1.13` 版本中，添加了对 [agit](https://git-repo.info/zh/2020/03/agit-flow-and-git-repo/) 的支持。

## 使用 Agit 创建 PR

Agit 允许在推送代码到远程仓库时创建 PR（合并请求）。
通过在推送时使用特定的 refspec（git 中已知的位置标识符），可以实现这一功能。
下面的示例说明了这一点：

```shell
git push origin HEAD:refs/for/master
```

该命令的结构如下：

- `HEAD`：目标分支
- `refs/<for|draft|for-review>/<branch>`：目标 PR 类型
  - `for`：创建一个以 `<branch>` 为目标分支的普通 PR
  - `draft`/`for-review`：目前被静默忽略
- `<branch>/<session>`：要打开 PR 的目标分支
- `-o <topic|title|description>`：PR 的选项
  - `title`：PR 的标题
  - `topic`：PR 应该打开的分支名称
  - `description`：PR 的描述
  - `force-push`：确认强制更新目标分支

下面是另一个高级示例，用于创建一个以 `topic`、`title` 和 `description` 为参数的新 PR，目标分支是 `master`：

```shell
git push origin HEAD:refs/for/master -o topic="Topic of my PR" -o title="Title of the PR" -o description="# The PR Description\nThis can be **any** markdown content.\n- [x] Ok"
```
