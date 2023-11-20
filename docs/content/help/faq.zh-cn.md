---
date: "2023-05-25T16:00:00+02:00"
title: "常见问题"
slug: "faq"
sidebar_position: 5
toc: false
draft: false
aliases:
  - /zh-cn/faq
menu:
  sidebar:
    parent: "help"
    name: "常见问题"
    sidebar_position: 5
    identifier: "faq"
---

# 常见问题

本页面包含一些常见问题和答案。

有关更多帮助资源，请查看所有[支持选项](help/support.md)。

## 1.x和1.x.x下载之间的区别

以1.7.x版本为例。

**注意：**此示例也适用于Docker镜像！

在我们的[下载页面](https://dl.gitea.com/gitea/)上，您会看到一个1.7目录，以及1.7.0、1.7.1、1.7.2、1.7.3、1.7.4、1.7.5和1.7.6的目录。

1.7目录和1.7.0目录是**不同**的。1.7目录是在每个合并到[`release/v1.7`](https://github.com/go-gitea/gitea/tree/release/v1.7)分支的提交上构建的。

然而，1.7.0目录是在创建[`v1.7.0`](https://github.com/go-gitea/gitea/releases/tag/v1.7.0)标签时创建的构建。

这意味着1.x的下载会随着提交合并到各自的分支而改变（将其视为每个版本的单独的“main”分支）。

另一方面，1.x.x的下载应该永远不会改变。

## 如何从Gogs/GitHub等迁移到Gitea

要从Gogs迁移到Gitea：

- [Gogs版本0.9.146或更低](installation/upgrade-from-gogs.md)
- [Gogs版本0.11.46.0418](https://github.com/go-gitea/gitea/issues/4286)

要从GitHub迁移到Gitea，您可以使用Gitea内置的迁移表单。

为了迁移诸如问题、拉取请求等项目，您需要至少输入您的用户名。

[Example (requires login)](https://try.gitea.io/repo/migrate)

要从GitLab迁移到Gitea，您可以使用这个非关联的工具：

https://github.com/loganinak/MigrateGitlabToGogs

## Gitea存储文件的位置

- _`AppWorkPath`_
  - `--work-path`标志
  - 或者环境变量`GITEA_WORK_DIR`
  - 或者在构建时设置的内置值
  - 或者包含Gitea二进制文件的目录
- `%(APP_DATA_PATH)`（数据库、索引器等的默认路径）
  - `app.ini`中的`APP_DATA_PATH`
  - 或者_`AppWorkPath`_`/data`
- _`CustomPath`_（自定义模板）
  - `--custom-path`标志
  - 或者环境变量`GITEA_CUSTOM`
  - 或者在构建时设置的内置值
  - 或者_`AppWorkPath`_`/custom`
- HomeDir
  - Unix：环境变量`HOME`
  - Windows：环境变量`USERPROFILE`，或者环境变量`HOMEDRIVE`+`HOMEPATH`
- RepoRootPath
  - `app.ini`中\[repository]部分的`ROOT`（如果是绝对路径）
  - 否则_`AppWorkPath`_`/ROOT`(如果`app.ini`中\[repository]部分的`ROOT`是相对路径）
  - 默认值为`%(APP_DATA_PATH)/gitea-repositories`
- INI（配置文件）
  - `--config`标志
  - 或者在构建时设置的可能内置值
  - 或者 _`CustomPath`_`/conf/app.ini`
- SQLite数据库
  - app.ini中database部分的PATH
  - 或者`%(APP_DATA_PATH)/gitea.db`

## 看不到克隆URL或克隆URL不正确

有几个地方可能会导致显示不正确。

1. 如果使用反向代理，请确保按照[反向代理指南](administration/reverse-proxies.md)中的正确说明进行设置。
2. 确保在`app.ini`的`server`部分中正确设置了`ROOT_URL`。

如果某些克隆选项未显示（HTTP/S或SSH），可以在`app.ini中`

- `DISABLE_HTTP_GIT`: 如果设为true, 将会没有HTTP/HTTPS链接
- `DISABLE_SSH`: 如果设为true, 将会没有SSH链接
- `SSH_EXPOSE_ANONYMOUS`: 如果设为false, SSH链接将会对匿名用户隐藏

## 文件上传失败：413 Request Entity Too Large

当反向代理限制文件上传大小时，会出现此错误。

有关使用nginx解决此问题，请参阅[反向代理指南](administration/reverse-proxies.md)。

## 自定义模板无法加载或运行错误

Gitea的自定义模板必须将其添加到正确的位置，否则Gitea将无法找到并使用自定义模板。

模板的正确路径应该相对于`CustomPath`。

1. 要找到`CustomPath`，请在站点管理 -> 配置 中查找自定义文件根路径。

    如果找不到，请尝试`echo $GITEA_CUSTOM`。

2. 如果仍然找不到，默认值可以被[计算](help/faq.md#where-does-gitea-store-what-file)
3. 如果仍然找不到路径，则可以参考[自定义Gitea](administration/customizing-gitea.md)页面，将模板添加到正确的位置。

## Gitea是否有"GitHub/GitLab Pages"功能？

Gitea不提供内置的Pages服务器。您需要一个专用的域名来提供静态页面，以避免CSRF安全风险。

对于简单的用法，您可以使用反向代理来重写和提供Gitea的原始文件URL中的静态内容。

还有一些已经可用的第三方服务，比如独立[pages server](https://codeberg.org/Codeberg/pages-server)的或[caddy plugin](https://github.com/42wim/caddy-gitea)，可以提供所需的功能。

## 活跃用户与禁止登录用户

在Gitea中，"活跃用户"是指通过电子邮件激活其帐户的用户。

"禁止登录用户"是指不允许再登录到Gitea的用户。

## 设置日志记录

- [官方文档](administration/logging-config.md)

## 什么是Swagger？

[Swagger](https://swagger.io/) 是Gitea用于其API文档的工具。

所有Gitea实例都有内置的API，无法完全禁用它。
但是，您可以在app.ini的api部分将ENABLE_SWAGGER设置为false，以禁用其文档显示。
有关更多信息，请参阅Gitea的[API文档](development/api-usage.md)。

您可以在上查看最新的API（例如）https://try.gitea.io/api/swagger

您还可以在上查看`swagger.json`文件的示例 https://try.gitea.io/swagger.v1.json

## 调整服务器用于公共/私有使用

### 防止垃圾邮件发送者

有多种方法可以组合使用来防止垃圾邮件发送者：

1. 通过设置电子邮件域名的白名单或黑名单。
2. 通过设置一些域名或者OpenID白名单（见下文）。
3. 在您的`app.ini`中将`ENABLE_CAPTCHA`设置为`true`，并正确配置`RECAPTCHA_SECRET`和 `RECAPTCHA_SITEKEY`。
4. 将`DISABLE_REGISTRATION`设置为`true`，并通过 [CLI](administration/command-line.md)、[API](development/api-usage.md) 或 Gitea 的管理界面创建新用户。

### 仅允许/阻止特定的电子邮件域名

您可以在`app.ini`中的`[service]`下的配置`EMAIL_DOMAIN_WHITELIST` 或 `EMAIL_DOMAIN_BLOCKLIST`。

### 仅允许/阻止特定的 OpenID 提供商

您可以在`app.ini`的`[openid]`下配置`WHITELISTED_URI`或`BLACKLISTED_URIS`。

**注意**： 白名单优先，如果白名单非空，则忽略黑名单。

### 仅允许发布问题的用户

目前实现这一点的方法是创建/修改一个具有最大仓库创建限制为 0 的用户。

### 受限制的用户

受限制的用户仅能访问其组织/团队成员和协作所在的内容的子集，而忽略组织/仓库等的公共标志。

示例用例：一个公司运行一个需要登录的 Gitea 实例。大多数仓库是公开的（所有同事都可以访问/浏览）。

在某些情况下，某个客户或第三方需要访问特定的仓库，并且只能访问该仓库。通过将此类客户帐户设置为受限制帐户，并使用团队成员身份和/或协作来授予所需的任何访问权限，可以简单地实现这一点，而无需使所有内容都变为私有。

### 启用 Fail2ban

使用 [Fail2Ban](administration/fail2ban-setup.md) 监视并阻止基于日志模式的自动登录尝试或其他恶意行为。

## 如何添加/使用自定义主题

Gitea 目前支持三个官方主题，分别是 `gitea`（亮色）、`arc-green`（暗色）和 `auto`（根据操作系统设置自动切换前两个主题）。
要添加自己的主题，目前唯一的方法是提供一个完整的主题（不仅仅是颜色覆盖）。

假设我们的主题是 `arc-blue`（这是一个真实的主题，可以在[此问题](https://github.com/go-gitea/gitea/issues/6011)中找到）

将`.css`文件命名为`theme-arc-blue.css`并将其添加到`custom/public/css`文件夹中

通过将`arc-blue`添加到`app.ini`中的`THEMES`列表中，允许用户使用该主题

## SSHD vs 内建SSH

SSHD是大多数Unix系统上内建的SSH服务器。

Gitea还提供了自己的SSH服务器，用于在SSHD不可用时使用。

## Gitea运行缓慢

导致此问题的最常见原因是加载联合头像。

您可以通过在`app.ini`中将`ENABLE_FEDERATED_AVATAR`设置为`false`来关闭此功能。

还有一个可能需要更改的选项是在`app.ini`中将`DISABLE_GRAVATAR`设置为`true`。

## 无法创建仓库/文件

请确保Gitea具有足够的权限来写入其主目录和数据目录。

参见[AppDataPath 和 RepoRootPath](help/faq.md#where-does-gitea-store-what-file)

**适用于Arch用户的注意事项：**在撰写本文时，Arch软件包的systemd文件包含了以下行：

`ReadWritePaths=/etc/gitea/app.ini`

这将使得Gitea无法写入其他路径。

## 翻译不正确/如何添加更多翻译

我们当前的翻译是在我们的[Crowdin项目](https://crowdin.com/project/gitea)上众包进行的

无论您想要更改翻译还是添加新的翻译，都需要在Crowdin集成中进行，因为所有翻译都会被CI覆盖。

## 推送钩子/ Webhook未运行

如果您可以推送但无法在主页仪表板上看到推送活动，或者推送不触发Webhook，有几种可能性：

1. Git钩子不同步：在站点管理面板上运行“重新同步所有仓库的pre-receive、update和post-receive钩子”
2. Git仓库（和钩子）存储在一些不支持脚本执行的文件系统上（例如由NAS挂载），请确保文件系统支持`chmod a+x any-script`
3. 如果您使用的是Docker，请确保Docker Server（而不是客户端）的版本 >= 20.10.6

## SSH问题

如果无法通过`ssh`访问仓库，但`https`正常工作，请考虑以下情况。

首先，请确保您可以通过SSH访问Gitea。

`ssh git@myremote.example`

如果连接成功，您应该会收到以下错误消息：

```
Hi there, You've successfully authenticated, but Gitea does not provide shell access.
If this is unexpected, please log in with password and setup Gitea under another user.
```

如果您收到以上消息但仍然连接成功，这意味着您的 SSH 密钥**没有**由 Gitea 管理。这意味着钩子不会运行，在其他一些潜在问题中也包括在内。

如果您无法连接，可能是因为您的 SSH 密钥在本地配置不正确。
这是针对 SSH 而不是 Gitea 的问题，因此在此不涉及。

### SSH 常见错误

```
Permission denied (publickey).
fatal: Could not read from remote repository.
```

此错误表示服务器拒绝登录尝试，
请检查以下事项：

- 在客户端：
  - 确保公钥和私钥已添加到正确的 Gitea 用户。
  - 确保远程 URL 中没有任何问题。特别是，请确保∂
  Git 用户（@ 之前的部分）的名称拼写正确。
  - 确保客户端机器上的公钥和私钥正确无误。
- 在服务器上：
  - 确保存储库存在并且命名正确。
  - 检查系统用户主目录中的 `.ssh` 目录的权限。
  - 验证正确的公钥是否已添加到 `.ssh/authorized_keys` 中。

  尝试在 Gitea 管理面板上运行
  `Rewrite '.ssh/authorized_keys' file (for Gitea SSH keys)`。
- 查看 Gitea 日志。
- 查看 /var/log/auth（或类似的文件）。
- 检查存储库的权限。

以下是一个示例，其中缺少公共 SSH 密钥，
认证成功，但是其他设置导致 SSH 无法访问正确的
存储库。

```
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.
```

在这种情况下，请检查以下设置：

- 在服务器上：
  - 确保`git`系统用户设置了可用的 shell
    - 使用`getent passwd git | cut -d: -f7`进行验证
    - 可以使用`usermod`或`chsh`进行修改。
  - 确保`.ssh/authorized_keys`中的`gitea serv`命令使用
    正确的配置文件。

## 迁移带有标签的存储库后缺失发布版本

要迁移带有所有标签的存储库，您需要执行两个操作：

- 推送标签到存储库：

```
 git push --tags
```

- 在 Gitea 中重新同步所有存储库的标签：

```
gitea admin repo-sync-releases
```

## LFS 问题

针对涉及 LFS 数据上传的问题

```
batch response: Authentication required: Authorization error: <GITEA_LFS_URL>/info/lfs/objects/batch
Check that you have proper access to the repository
error: failed to push some refs to '<GIT_REPO_URL>'
```

检查`app.ini`文件中的`LFS_HTTP_AUTH_EXPIRY`值。

默认情况下，LFS 令牌在 20 分钟后过期。如果您的连接速度较慢或文件较大（或两者都是），可能无法在时间限制内完成上传。

您可以将此值设置为`60m`或`120m`。

## 如何在启动 Gitea 之前创建用户

Gitea 提供了一个子命令`gitea migrate`来初始化数据库，然后您可以使用[管理 CLI 命令](administration/command-line.md#admin)像正常情况下添加用户。

## 如何启用密码重置

没有密码重置的设置。当配置了[邮件服务](administration/email-setup.md)时，密码重置将自动启用；否则将被禁用。

## 如何更改用户的密码

- 作为管理员，您可以更改任何用户的密码（并可选择强制其在下次登录时更改密码）...
  - 转到您的`站点管理 -> 用户账户`页面并编辑用户。
- 使用[管理 CLI 命令](administration/command-line.md#admin)。

  请注意，大多数命令还需要一个[全局标志](administration/command-line.md#global-options)来指向正确的配置。
- 作为**用户**，您可以更改密码...
  - 在您的账户的`设置 -> 账户`页面（此方法**需要**您知道当前密码）。
  - 使用`忘记密码`链接。

  如果`忘记密码/账户恢复`页面被禁用，请联系管理员配置[邮件服务](administration/email-setup.md)。

## 为什么我的 Markdown 显示错误

在 Gitea 版本 `1.11` 中，我们转换为使用[goldmark](https://github.com/yuin/goldmark)进行 Markdown 渲染，它符合[CommonMark](https://commonmark.org/)标准。

如果您在版本`1.11`之前的Markdown正常工作，但在升级后无法正常工作，请仔细阅读CommonMark规范，看看问题是由错误还是非兼容的语法引起的。

如果是后者，通常规范中会列出一种符合标准的替代方法。

## 使用 MySQL 进行升级时出现的错误

如果在使用 MySQL 升级 Gitea 时收到以下错误：

> `ORM engine initialization failed: migrate: do migrate: Error: 1118: Row size too large...`

请运行`gitea convert`或对数据库中的每个表运行`ALTER TABLE table_name ROW_FORMAT=dynamic;`。

潜在问题是默认行格式分配给每个表的索引空间
太小。Gitea 要求其表的`ROWFORMAT`为`DYNAMIC`。

如果收到包含`Error 1071: Specified key was too long; max key length is 1000 bytes...`
的错误行，则表示您正在尝试在使用 ISAM 引擎的表上运行 Gitea。尽管在先前版本的 Gitea 中可能是凑巧能够工作的，但它从未得到官方支持，
您必须使用 InnoDB。您应该对数据库中的每个表运行`ALTER TABLE table_name ENGINE=InnoDB;`。

如果您使用的是 MySQL 5，另一个可能的修复方法是：

```mysql
SET GLOBAL innodb_file_format=Barracuda;
SET GLOBAL innodb_file_per_table=1;
SET GLOBAL innodb_large_prefix=1;
```

## 为什么 MySQL 上的 Emoji 显示错误

不幸的是，MySQL 的`utf8`字符集不完全允许所有可能的 UTF-8 字符，特别是 Emoji。
他们创建了一个名为 `utf8mb4`的字符集和校对规则，允许存储 Emoji，但使用
utf8 字符集的表和连接将不会使用它。

请运行 `gitea convert` 或对数据库运行`ALTER DATABASE database_name CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;`
并对每个表运行
`ALTER TABLE table_name CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;`。

您还需要将`app.ini`文件中的数据库字符集设置为`CHARSET=utf8mb4`。

## 为什么 Emoji 只显示占位符或单色图像

Gitea 需要系统或浏览器安装其中一个受支持的 Emoji 字体，例如 Apple Color Emoji、Segoe UI Emoji、Segoe UI Symbol、Noto Color Emoji 和 Twemoji Mozilla。通常，操作系统应该已经提供了其中一个字体，但特别是在 Linux 上，可能需要手动安装它们。

## SystemD 和 Docker 上的标准输出日志

SystemD 上的标准输出默认会写入日志记录中。您可以尝试使用 `journalctl`、`journalctl -u gitea` 或 `journalctl <path-to-gitea-binary>`来查看。

类似地，Docker 上的标准输出可以使用`docker logs <container>`来查看。

要收集日志以进行帮助和问题报告，请参阅[支持选项](help/support.md)。

## 初始日志记录

在 Gitea 读取配置文件并设置其日志记录之前，它会将一些内容记录到标准输出，以帮助调试日志记录无法工作的情况。

您可以通过设置`--quiet`或`-q`选项来停止此日志记录。请注意，这只会在 Gitea 设置自己的日志记录之前停止日志记录。

如果您报告了错误或问题，必须提供这些信息以恢复初始日志记录。

只有在完全配置了所有内容之后，您才应该设置此选项。

## 在数据库启动期间出现有关结构默认值的警告

有时，在迁移过程中，旧列和默认值可能在数据库架构中保持不变。
这可能会导致警告，例如：

```
2020/08/02 11:32:29 ...rm/session_schema.go:360:Sync2() [W] Table user Column keep_activity_private db default is , struct default is 0
```

可以安全地忽略这些警告，但您可以通过让 Gitea 重新创建这些表来停止这些警告，使用以下命令：

```
gitea doctor recreate-table user
```

这将导致 Gitea 重新创建用户表并将旧数据复制到新表中，
并正确设置默认值。

您可以使用以下命令要求 Gitea 重新创建多个表：

```
gitea doctor recreate-table table1 table2 ...
```

如果您希望 Gitea 重新创建所有表，请使用以下命令：

```
gitea doctor recreate-table
```

在运行这些命令之前，强烈建议您备份数据库。

## 为什么查看文件时制表符/缩进显示错误

如果您正在使用 Cloudflare，请在仪表板中关闭自动缩小选项。

`Speed` -> `Optimization` -> 在 `Auto-Minify` 设置中取消选中 `HTML`。

## 如何从磁盘采用存储库

- 将您的（裸）存储库添加到正确的位置，即您的配置所在的地方（`repository.ROOT`），确保它们位于正确的布局`<REPO_ROOT>/[user]/[repo].git`。
  - **注意：**目录名必须为小写。
  - 您还可以在`<ROOT_URL>/admin/config`中检查存储库根路径。
- 确保存在要采用存储库的用户/组织。
- 作为管理员，转到`<ROOT_URL>/admin/repos/unadopted`并搜索。
- 用户也可以通过配置[`ALLOW_ADOPTION_OF_UNADOPTED_REPOSITORIES`](administration/config-cheat-sheet.md#repository) 获得类似的权限。
- 如果上述步骤都正确执行，您应该能够选择要采用的存储库。
  - 如果没有找到存储库，请启用[调试日志记录](administration/config-cheat-sheet.md#repository)以检查是否有特定错误。
