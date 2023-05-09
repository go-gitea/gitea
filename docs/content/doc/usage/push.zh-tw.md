---
date: "2020-07-06T16:00:00+02:00"
title: "使用: Push"
slug: "push"
weight: 15
toc: false
draft: false
aliases:
  - /zh-tw/push-options
menu:
  sidebar:
    parent: "usage"
    name: "Push"
    weight: 15
    identifier: "push"
---

**Table of Contents**

{{< toc >}}

There are some additional features when pushing commits to Gitea server.

# Push Merge Hint

When you pushing commits to a non-default branch, you will get an information from
Gitea which is a link, you can click the link and go to a compare page. It's a quick
way to create a pull request or a code review yourself in the Gitea UI.

![Gitea Push Hint](/gitea-push-hint.png)

# Push Options

Gitea 從 `1.13` 版開始支援某些 [push options](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt)
。

## 支援的 Options

- `repo.private` (true|false) - 修改儲存庫的可見性。

  與 push-to-create 一起使用時特別有用。

- `repo.template` (true|false) - 修改儲存庫是否為範本儲存庫。

以下範例修改儲存庫的可見性為公開：

```shell
git push -o repo.private=false -u origin main
```

# Push To Create

Push to create is a feature that allows you to push to a repository that does not exist yet in Gitea. This is useful for automation and for allowing users to create repositories without having to go through the web interface. This feature is disabled by default.

## Enabling Push To Create

In the `app.ini` file, set `ENABLE_PUSH_CREATE_USER` to `true` and `ENABLE_PUSH_CREATE_ORG` to `true` if you want to allow users to create repositories in their own user account and in organizations they are a member of respectively. Restart Gitea for the changes to take effect. You can read more about these two options in the [Configuration Cheat Sheet]({{ < relref "doc/administration/config-cheat-sheet.zh-tw.md#repository-repository" > }}).

## Using Push To Create

Assuming you have a git repository in the current directory, you can push to a repository that does not exist yet in Gitea by running the following command:

```shell
# Add the remote you want to push to
git remote add origin git@{domain}:{username}/{repo name that does not exist yet}.git

# push to the remote
git push -u origin main
```

This assumes you are using an SSH remote, but you can also use HTTPS remotes as well.
