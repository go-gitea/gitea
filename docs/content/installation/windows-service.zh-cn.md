---
date: "2016-12-21T15:00:00-02:00"
title: "注册为Windows服务"
slug: "windows-service"
sidebar_position: 50
toc: false
draft: false
aliases:
  - /zh-cn/windows-service
menu:
  sidebar:
    parent: "installation"
    name: "Windows服务"
    sidebar_position: 50
    identifier: "windows-service"
---

# 准备工作

在 C:\gitea\custom\conf\app.ini 中进行了以下更改：

```
RUN_USER = COMPUTERNAME$
```

将 Gitea 设置为以本地系统用户运行。

COMPUTERNAME 是从命令行中运行 `echo %COMPUTERNAME%` 后得到的响应。如果响应是 `USER-PC`，那么 `RUN_USER = USER-PC$`。

## 使用绝对路径

如果您使用 SQLite3，请将 `PATH` 更改为包含完整路径：

```
[database]
PATH     = c:/gitea/data/gitea.db
```

# 注册为Windows服务

要注册为Windows服务，首先以Administrator身份运行 `cmd`，然后执行以下命令：

```
sc.exe create gitea start= auto binPath= "\"C:\gitea\gitea.exe\" web --config \"C:\gitea\custom\conf\app.ini\""
```

别忘了将 `C:\gitea` 替换成你的 Gitea 安装目录。

之后在控制面板打开 "Windows Services"，搜索 "gitea"，右键选择 "Run"。在浏览器打开 `http://localhost:3000` 就可以访问了。（如果你修改了端口，请访问对应的端口，3000是默认端口）。

## 添加启动依赖项

要将启动依赖项添加到 Gitea Windows 服务（例如 Mysql、Mariadb），作为管理员，然后运行以下命令：

```
sc.exe config gitea depend= mariadb
```

这将确保在 Windows 计算机重新启动时，将延迟自动启动 Gitea，直到数据库准备就绪，从而减少启动失败的情况。

## 从Windows服务中删除

以Administrator身份运行 `cmd`，然后执行以下命令：

```
sc.exe delete gitea
```
