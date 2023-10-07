---
date: "2023-05-23T09:00:00+08:00"
title: "嵌入资源提取工具"
slug: "cmd-embedded"
sidebar_position: 20
toc: false
draft: false
aliases:
  - /zh-cn/cmd-embedded
menu:
  sidebar:
    parent: "administration"
    name: "嵌入资源提取工具"
    sidebar_position: 20
    identifier: "cmd-embedded"
---

# 嵌入资源提取工具

Gitea 的可执行文件包含了运行所需的所有资源：模板、图片、样式表和翻译文件。你可以通过在 `custom` 目录下的相应路径中放置替换文件来覆盖其中的任何资源（详见 [自定义 Gitea 配置](administration/customizing-gitea.md)）。

要获取嵌入资源的副本以进行编辑，可以使用 CLI 中的 `embedded` 命令，通过操作系统的 shell 执行。

**注意：** 嵌入资源提取工具包含在 Gitea 1.12 及以上版本中。

## 资源列表

要列出嵌入在 Gitea 可执行文件中的资源，请使用以下语法：

```sh
gitea embedded list [--include-vendored] [patterns...]
```

`--include-vendored` 标志使命令包括被供应的文件，这些文件通常被排除在外；即来自外部库的文件，这些文件是 Gitea 所需的（例如 [octicons](https://octicons.github.com/) 等）。

可以提供一系列文件搜索模式。Gitea 使用 [gobwas/glob](https://github.com/gobwas/glob) 作为其 glob 语法。以下是一些示例：

- 列出所有模板文件，无论在哪个虚拟目录下：`**.tmpl`
- 列出所有邮件模板文件：`templates/mail/**.tmpl`
- 列出 `public/img` 目录下的所有文件：`public/img/**`

不要忘记为模式使用引号，因为空格、`*` 和其他字符可能对命令行解释器有特殊含义。

如果未提供模式，则列出所有文件。

### 示例：列出所有嵌入文件

列出所有路径中包含 `openid` 的嵌入文件：

```sh
$ gitea embedded list '**openid**'
public/img/auth/openid_connect.svg
public/img/openid-16x16.png
templates/user/auth/finalize_openid.tmpl
templates/user/auth/signin_openid.tmpl
templates/user/auth/signup_openid_connect.tmpl
templates/user/auth/signup_openid_navbar.tmpl
templates/user/auth/signup_openid_register.tmpl
templates/user/settings/security_openid.tmpl
```

## 提取资源

要提取嵌入在 Gitea 可执行文件中的资源，请使用以下语法：

```sh
gitea [--config {file}] embedded extract [--destination {dir}|--custom] [--overwrite|--rename] [--include-vendored] {patterns...}
```

`--config` 选项用于告知 Gitea `app.ini` 配置文件的位置（如果不在默认位置）。此选项仅在使用 `--custom` 标志时使用。

`--destination` 选项用于指定提取文件的目标目录。默认为当前目录。

`--custom` 标志告知 Gitea 直接将文件提取到 `custom` 目录中。为使其正常工作，该命令需要知道 `app.ini` 配置文件的位置（通过 `--config` 指定），并且根据配置的不同，需要从 Gitea 通常启动的目录运行。有关详细信息，请参阅 [自定义 Gitea 配置](administration/customizing-gitea.md)。

`--overwrite` 标志允许覆盖目标目录中的任何现有文件。

`--rename` 标志告知 Gitea 将目标目录中的任何现有文件重命名为 `filename.bak`。之前的 `.bak` 文件将被覆盖。

至少需要提供一个文件搜索模式；有关模式的语法和示例，请参阅上述 `list` 子命令。

### 重要提示

请确保**只提取需要自定义的文件**。位于 `custom` 目录中的文件不会受到 Gitea 的升级过程的影响。当 Gitea 升级到新版本（通过替换可执行文件）时，许多嵌入文件将发生变化。Gitea 将尊重并使用在 `custom` 目录中找到的任何文件，即使这些文件是旧的和不兼容的。

### 示例：提取邮件模板

将邮件模板提取到临时目录：

```sh
$ mkdir tempdir
$ gitea embedded extract --destination tempdir 'templates/mail/**.tmpl'
Extracting to tempdir:
tempdir/templates/mail/auth/activate.tmpl
tempdir/templates/mail/auth/activate_email.tmpl
tempdir/templates/mail/auth/register_notify.tmpl
tempdir/templates/mail/auth/reset_passwd.tmpl
tempdir/templates/mail/issue/assigned.tmpl
tempdir/templates/mail/issue/default.tmpl
tempdir/templates/mail/notify/collaborator.tmpl
```
