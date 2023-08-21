---
date: "2016-12-01T16:00:00+02:00"
title: "从 Gogs 升级"
slug: "upgrade-from-gogs"
sidebar_position: 101
toc: false
draft: false
aliases:
  - /zh-cn/upgrade-from-gogs
menu:
  sidebar:
    parent: "installation"
    name: "从 Gogs 升级"
    sidebar_position: 101
    identifier: "upgrade-from-gogs"
---

# 从 Gogs 升级

如果你正在运行Gogs 0.9.146以下版本，你可以平滑的升级到Gitea。该升级需要如下的步骤：

* 使用 `gogs backup` 创建 Gogs 备份。这会创建一个名为 `gogs-backup-[时间戳].zip` 的文件，其中包含所有重要的 Gogs 数据。如果您将来想要返回到 `gogs`，您会需要这个备份文件。
* 从 [下载页面](https://dl.gitea.com/gitea/) 下载适用于目标平台的文件。应该选择 `1.0.x` 版本。从 `gogs` 迁移到其他任何版本是不可能的。
* 将二进制文件放置在所需的安装位置。
* 将 `gogs/custom/conf/app.ini` 复制到 `gitea/custom/conf/app.ini`。
* 将 `gogs/custom/` 中的自定义 `templates, public` 复制到 `gitea/custom/`。
* 对于其他自定义文件夹，例如 `gogs/custom/conf` 中的 `gitignore, label, license, locale, readme`，将它们复制到 `gitea/custom/options`。
* 将 `gogs/data/` 复制到 `gitea/data/`。其中包含问题附件和头像。
* 使用 `gitea web` 启动 Gitea 进行验证。
* 在 UI 上进入 Gitea 管理面板，运行 `Rewrite '.ssh/authorized_keys' file`。
* 启动每个主要版本的二进制文件（例如 `1.1.4` → `1.2.3` → `1.3.4` → `1.4.2` → 等）以迁移数据库。
* 如果自定义或配置路径已更改，请运行 `Rewrite all update hook of repositories`。

## 更改特定于 Gogs 的信息

* 将 `gogs-repositories/` 重命名为 `gitea-repositories/`
* 将 `gogs-data/` 重命名为 `gitea-data/`
* 在 `gitea/custom/conf/app.ini` 中进行更改：
从:

 ```ini
  [database]
  PATH = /home/:USER/gogs/data/:DATABASE.db
  [attachment]
  PATH = /home/:USER/gogs-data/attachments
  [picture]
  AVATAR_UPLOAD_PATH = /home/:USER/gogs-data/avatars
  [log]
  ROOT_PATH = /home/:USER/gogs/log
  ```

到:

  ```ini
  [database]
  PATH = /home/:USER/gitea/data/:DATABASE.db
  [attachment]
  PATH = /home/:USER/gitea-data/attachments
  [picture]
  AVATAR_UPLOAD_PATH = /home/:USER/gitea-data/avatars
  [log]
  ROOT_PATH = /home/:USER/gitea/log
  ```

* 使用 `gitea web` 启动 Gitea 进行验证

## 升级到最新版本的 `gitea`

在成功从 `gogs` 迁移到 `gitea 1.0.x` 之后，可以通过两步过程将 `gitea` 升级到现代版本。

首先升级到 [`gitea 1.6.4`](https://dl.gitea.com/gitea/1.6.4/)。从 [下载页面](https://dl.gitea.com/gitea/1.6.4/) 下载适用于目标平台的文件，并替换二进制文件。至少运行一次 Gitea 并检查是否一切正常。

然后重复这个过程，但这次使用 [最新版本](https://dl.gitea.com/gitea/@version@/)。

## 从较新的 Gogs 版本升级

从较新的 Gogs 版本（最高到 `0.11.x`）可能也是可能的，但需要更多的工作。
请参见 [#4286](https://github.com/go-gitea/gitea/issues/4286)，其中包括各种 Gogs `0.11.x` 版本。

从 Gogs `0.12.x` 及更高版本升级将变得越来越困难，因为项目在配置和架构上逐渐分歧。

## 故障排除

* 如果在 `gitea/custom/templates` 文件夹中遇到与自定义模板相关的错误，请尝试逐个移除引发错误的模板。
  它们可能与 Gitea 或更新不兼容。

## 将 Gitea 添加到 Unix 的启动项

从 [gitea/contrib](https://github.com/go-gitea/gitea/tree/main/contrib) 更新适当的文件，确保正确的环境变量。

对于使用 systemd 的发行版：

* 将更新后的脚本复制到 `/etc/systemd/system/gitea.service`
* 使用以下命令将服务添加到启动项：`sudo systemctl enable gitea`
* 禁用旧的 gogs 启动脚本：`sudo systemctl disable gogs`

对于使用 SysVinit 的发行版：

* 将更新后的脚本复制到 `/etc/init.d/gitea`
* 使用以下命令将服务添加到启动项：`sudo rc-update add gitea`
* 禁用旧的 gogs 启动脚本：`sudo rc-update del gogs`
