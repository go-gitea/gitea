---
date: "2023-05-23T09:00:00+08:00"
title: "仓库镜像"
slug: "repo-mirror"
sidebar_position: 45
toc: false
draft: false
aliases:
  - /zh-cn/repo-mirror
menu:
  sidebar:
    parent: "usage"
    name: "仓库镜像"
    sidebar_position: 45
    identifier: "repo-mirror"
---

# 仓库镜像

仓库镜像允许将仓库与外部源之间进行镜像。您可以使用它在仓库之间镜像分支、标签和提交。

## 使用场景

以下是一些仓库镜像的可能使用场景：

- 您迁移到了 Gitea，但仍需要在其他源中保留您的项目。在这种情况下，您可以简单地设置它以进行镜像到 Gitea（拉取），这样您的 Gitea 实例中就可以获取到所有必要的提交历史、标签和分支。
- 您在其他源中有一些旧项目，您不再主动使用，但出于归档目的不想删除。在这种情况下，您可以创建一个推送镜像，以便您的活跃的 Gitea 仓库可以将其更改推送到旧位置。

## 从远程仓库拉取

对于现有的远程仓库，您可以按照以下步骤设置拉取镜像：

1. 在右上角的“创建...”菜单中选择“迁移外部仓库”。
2. 选择远程仓库服务。
3. 输入仓库的 URL。
4. 如果仓库需要身份验证，请填写您的身份验证信息。
5. 选中“该仓库将是一个镜像”复选框。
6. 选择“迁移仓库”以保存配置。

现在，该仓库会定期从远程仓库进行镜像。您可以通过在仓库设置中选择“立即同步”来强制进行同步。

:exclamation::exclamation: **注意：**您只能为尚不存在于您的实例上的仓库设置拉取镜像。一旦仓库创建成功，您就无法再将其转换为拉取镜像。:exclamation::exclamation:

## 推送到远程仓库

对于现有的仓库，您可以按照以下步骤设置推送镜像：

1. 在仓库中，转到**设置** > **仓库**，然后进入**镜像设置**部分。
2. 输入一个仓库的 URL。
3. 如果仓库需要身份验证，请展开**授权**部分并填写您的身份验证信息。请注意，所请求的**密码**也可以是您的访问令牌。
4. 选择**添加推送镜像**以保存配置。

该仓库现在会定期镜像到远程仓库。您可以通过选择**立即同步**来强制同步。如果出现错误，会显示一条消息帮助您解决问题。

:exclamation::exclamation: **注意：** 这将强制推送到远程仓库。这将覆盖远程仓库中的任何更改！ :exclamation::exclamation:

### 从 Gitea 向 GitHub 设置推送镜像

要从 Gitea 设置镜像到 GitHub，您需要按照以下步骤进行操作：

1. 创建一个具有选中 *public_repo* 选项的 [GitHub 个人访问令牌](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token)。
2. 在 GitHub 上创建一个同名的仓库。与 Gitea 不同，GitHub 不支持通过推送到远程来创建仓库。如果您的现有远程仓库与您的 Gitea 仓库具有相同的提交历史，您也可以使用现有的远程仓库。
3. 在您的 Gitea 仓库设置中，填写**Git 远程仓库 URL**：`https://github.com/<your_github_group>/<your_github_project>.git`。
4. 使用您的 GitHub 用户名填写**授权**字段，并将个人访问令牌作为**密码**。
5. （可选，适用于 Gitea 1.18+）选择`当推送新提交时同步`，这样一旦有更改，镜像将会及时更新。如果您愿意，您还可以禁用定期同步。
6. 选择**添加推送镜像**以保存配置。

仓库会很快进行推送。要强制推送，请选择**立即同步**按钮。

### 从 Gitea 向 GitLab 设置推送镜像

要从 Gitea 设置镜像到 GitLab，您需要按照以下步骤进行操作：

1. 创建具有 *write_repository* 作用域的 [GitLab 个人访问令牌](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)。
2. 填写**Git 远程仓库 URL**：`https://<destination host>/<your_gitlab_group_or_name>/<your_gitlab_project>.git`。
3. 在**授权**字段中填写 `oauth2` 作为**用户名**，并将您的 GitLab 个人访问令牌作为**密码**。
4. 选择**添加推送镜像**以保存配置。

仓库会很快进行推送。要强制推送，请选择**立即同步**按钮。

### 从 Gitea 向 Bitbucket 设置推送镜像

要从 Gitea 设置镜像到 Bitbucket，您需要按照以下步骤进行操作：

1. 创建一个具有选中 *Repository Write* 选项的 [Bitbucket 应用密码](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/)。
2. 填写**Git 远程仓库 URL**：`https://bitbucket.org/<your_bitbucket_group_or_name>/<your_bitbucket_project>.git`。
3. 使用您的 Bitbucket 用户名填写**授权**字段，并将应用密码作为**密码**。
4. 选择**添加推送镜像**以保存配置。

仓库会很快进行推送。要强制推送，请选择**立即同步**按钮。

### 镜像现有的 ssh 仓库

当前，Gitea 不支持从 ssh 仓库进行镜像。如果您想要镜像一个 ssh 仓库，您需要将其转换为 http 仓库。您可以使用以下命令将现有的 ssh 仓库转换为 http 仓库：

1. 确保运行 gitea 的用户有权限访问您试图从 shell 镜像到的 git 仓库。
2. 在 Web 界面的版本库设置 > git 钩子中为镜像添加一个接收后钩子。

```
#!/usr/bin/env bash
git push --mirror --quiet git@github.com:username/repository.git &>/dev/null &
echo "GitHub mirror initiated .."
```
