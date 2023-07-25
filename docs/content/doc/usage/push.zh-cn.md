---
date: "2023-05-23T09:00:00+08:00"
title: "推送"
slug: "push"
weight: 15
toc: false
draft: false
aliases:
  - /zh-cn/push-to-create
  - /zh-cn/push-options
menu:
  sidebar:
    parent: "usage"
    name: "推送"
    weight: 15
    identifier: "push"
---

**目录**

{{< toc >}}

在将提交推送到 Gitea 服务器时，还有一些额外的功能。

# 通过推送打开 PR

当您第一次将提交推送到非默认分支时，您将收到一个链接，您可以单击该链接访问分支与主分支的比较页面。
从那里，您可以轻松创建一个拉取请求，即使您想要将其目标指向另一个分支。

![Gitea 推送提示](/gitea-push-hint.png)

# 推送选项

在 Gitea `1.13` 版本中，添加了对一些 [推送选项](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt) 的支持。

## 支持的选项

- `repo.private` (true|false) - 更改仓库的可见性。

  这在与 push-to-create 结合使用时特别有用。

- `repo.template` (true|false) - 更改仓库是否为模板。

将仓库的可见性更改为公开的示例：

```shell
git push -o repo.private=false -u origin main
```

# 推送创建

推送创建是一项功能，允许您将提交推送到在 Gitea 中尚不存在的仓库。这对于自动化和允许用户创建仓库而无需通过 Web 界面非常有用。此功能默认处于禁用状态。

## 启用推送创建

在 `app.ini` 文件中，将 `ENABLE_PUSH_CREATE_USER` 设置为 `true`，如果您希望允许用户在自己的用户帐户和所属的组织中创建仓库，将 `ENABLE_PUSH_CREATE_ORG` 设置为 `true`。重新启动 Gitea 以使更改生效。您可以在 [配置速查表]({{< relref "doc/administration/config-cheat-sheet.zh-cn.md#repository-repository" >}}) 中了解有关这两个选项的更多信息。

## 使用推送创建

假设您在当前目录中有一个 git 仓库，您可以通过运行以下命令将提交推送到在 Gitea 中尚不存在的仓库：

```shell
# 添加要推送到的远程仓库
git remote add origin git@{domain}:{username}/{尚不存在的仓库名称}.git

# 推送到远程仓库
git push -u origin main
```

这假设您使用的是 SSH 远程，但您也可以使用 HTTPS 远程。

推送创建将默认使用 `app.ini` 中定义的可见性 `DEFAULT_PUSH_CREATE_PRIVATE`。
