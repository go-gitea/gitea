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

* 停止 Gogs 的运行
* 拷贝 Gogs 的配置文件 `custom/conf/app.ini` 到 Gitea 的相应位置。
* 拷贝 Gitea 的 `options/` 到 Home 目录下。
* 如果你还有更多的自定义内容，比如templates和localization文件，你需要手工合并你的修改到 Gitea 的 Options 下对应目录。
* 拷贝 Gogs 的数据目录 `data/` 到 Gitea 相应位置。这个目录包含附件和头像文件。
* 运行 Gitea
* 登录 Gitea 并进入 管理面板, 运行 `重新生成 '.ssh/authorized_keys' 文件（警告：不是 Gitea 的密钥也会被删除）` 和 `重新生成所有仓库的 Update 钩子（用于自定义配置文件被修改）`。
