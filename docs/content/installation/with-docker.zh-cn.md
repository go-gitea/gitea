---
date: "2016-12-01T16:00:00+02:00"
title: "使用 Docker 安装"
slug: "install-with-docker"
sidebar_position: 70
toc: false
draft: false
aliases:
  - /zh-cn/install-with-docker
menu:
  sidebar:
    parent: "installation"
    name: "使用 Docker 安装"
    sidebar_position: 70
    identifier: "install-with-docker"
---

# 使用 Docker 安装

Gitea 在其 Docker Hub 组织内提供自动更新的 Docker 镜像。可以始终使用最新的稳定标签或使用其他服务来更新 Docker 镜像。

该参考设置指导用户完成基于 `docker-compose` 的设置，但是 `docker-compose` 的安装不在本文档的范围之内。要安装 `docker-compose` 本身，请遵循官方[安装说明](https://docs.docker.com/compose/install/)。

## 基本

最简单的设置只是创建一个卷和一个网络，然后将 `gitea/gitea:latest` 镜像作为服务启动。由于没有可用的数据库，因此可以使用 SQLite3 初始化数据库。创建一个类似 `gitea` 的目录，并将以下内容粘贴到名为 `docker-compose.yml` 的文件中。请注意，该卷应由配置文件中指定的 UID/GID 的用户/组拥有。如果您不授予卷正确的权限，则容器可能无法启动。另请注意，标签 `:latest` 将安装当前的开发版本。对于稳定的发行版，您可以使用 `:1` 或指定某个发行版，例如 `@version@`。

```yaml
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:@version@
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

## 端口

要将集成的 openSSH 守护进程和 Web 服务器绑定到其他端口，请调整端口部分。通常，只需更改主机端口，容器内的端口保持原样即可。

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:@version@
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-     - "3000:3000"
-     - "222:22"
+     - "8080:3000"
+     - "2221:22"
```

## 数据库

### MySQL 数据库

要将 Gitea 与 MySQL 数据库结合使用，请将这些更改应用于上面创建的 `docker-compose.yml` 文件。

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:@version@
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
+      - GITEA__database__DB_TYPE=mysql
+      - GITEA__database__HOST=db:3306
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: mysql:8
+    restart: always
+    environment:
+      - MYSQL_ROOT_PASSWORD=gitea
+      - MYSQL_USER=gitea
+      - MYSQL_PASSWORD=gitea
+      - MYSQL_DATABASE=gitea
+    networks:
+      - gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

### PostgreSQL 数据库

要将 Gitea 与 PostgreSQL 数据库结合使用，请将这些更改应用于上面创建的 `docker-compose.yml` 文件。

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:@version@
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
+      - GITEA__database__DB_TYPE=postgres
+      - GITEA__database__HOST=db:5432
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: postgres:14
+    restart: always
+    environment:
+      - POSTGRES_USER=gitea
+      - POSTGRES_PASSWORD=gitea
+      - POSTGRES_DB=gitea
+    networks:
+      - gitea
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## 命名卷

要使用命名卷而不是主机卷，请在 `docker-compose.yml` 配置中定义并使用命名卷。此更改将自动创建所需的卷。您无需担心命名卷的权限；Docker 将自动处理该问题。

```diff
version: "3"

networks:
  gitea:
    external: false

+volumes:
+  gitea:
+    driver: local
+
services:
  server:
    image: gitea/gitea:@version@
    container_name: gitea
    restart: always
    networks:
      - gitea
    volumes:
-      - ./gitea:/data
+      - gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

MySQL 或 PostgreSQL 容器将需要分别创建。

## 启动

要基于 `docker-compose` 启动此设置，请执行 `docker-compose up -d`，以在后台启动 Gitea。使用 `docker-compose ps` 将显示 Gitea 是否正确启动。可以使用 `docker-compose logs` 查看日志。

要关闭设置，请执行 `docker-compose down`。这将停止并杀死容器。这些卷将仍然存在。

注意：如果在 http 上使用非 3000 端口，请更改 app.ini 以匹配 `LOCAL_ROOT_URL = http://localhost:3000/`。

## 安装

通过 `docker-compose` 启动 Docker 安装后，应该可以使用喜欢的浏览器访问 Gitea，以完成安装。访问 http://server-ip:3000 并遵循安装向导。如果数据库是通过上述 `docker-compose` 设置启动的，请注意，必须将 `db` 用作数据库主机名。

## 环境变量

您可以通过环境变量配置 Gitea 的一些设置：

（默认值以**粗体**显示）

- `APP_NAME`：**“Gitea: Git with a cup of tea”**：应用程序名称，在页面标题中使用。
- `RUN_MODE`：**prod**：应用程序运行模式，会影响性能和调试。"dev"，"prod"或"test"。
- `DOMAIN`：**localhost**：此服务器的域名，用于 Gitea UI 中显示的 http 克隆 URL。
- `SSH_DOMAIN`：**localhost**：该服务器的域名，用于 Gitea UI 中显示的 ssh 克隆 URL。如果启用了安装页面，则 SSH 域服务器将采用以下形式的 DOMAIN 值（保存时将覆盖此设置）。
- `SSH_PORT`：**22**：克隆 URL 中显示的 SSH 端口。
- `SSH_LISTEN_PORT`：**%(SSH_PORT)s**：内置 SSH 服务器的端口。
- `DISABLE_SSH`：**false**：如果不可用，请禁用 SSH 功能。如果要禁用 SSH 功能，则在安装 Gitea 时应将 SSH 端口设置为 `0`。
- `HTTP_PORT`：**3000**：HTTP 监听端口。
- `ROOT_URL`：**""**：覆盖自动生成的公共 URL。如果内部 URL 和外部 URL 不匹配（例如在 Docker 中），这很有用。
- `LFS_START_SERVER`：**false**：启用 git-lfs 支持。
- `DB_TYPE`：**sqlite3**：正在使用的数据库类型[mysql，postgres，mssql，sqlite3]。
- `DB_HOST`：**localhost:3306**：数据库主机地址和端口。
- `DB_NAME`：**gitea**：数据库名称。
- `DB_USER`：**root**：数据库用户名。
- `DB_PASSWD`：**"_empty_"** ：数据库用户密码。如果您在密码中使用特殊字符，请使用“您的密码”进行引用。
- `INSTALL_LOCK`：**false**：禁止访问安装页面。
- `SECRET_KEY`：**""** ：全局密钥。这应该更改。如果它具有一个值并且 `INSTALL_LOCK` 为空，则 `INSTALL_LOCK` 将自动设置为 `true`。
- `DISABLE_REGISTRATION`：**false**：禁用注册，之后只有管理员才能为用户创建帐户。
- `REQUIRE_SIGNIN_VIEW`：**false**：启用此选项可强制用户登录以查看任何页面。
- `USER_UID`：**1000**：在容器内运行 Gitea 的用户的 UID（Unix 用户 ID）。如果使用主机卷，则将其与 `/data` 卷的所有者的 UID 匹配（对于命名卷，则不需要这样做）。
- `USER_GID`：**1000**：在容器内运行 Gitea 的用户的 GID（Unix 组 ID）。如果使用主机卷，则将其与 `/data` 卷的所有者的 GID 匹配（对于命名卷，则不需要这样做）。

## 自定义

[此处](administration/customizing-gitea.md)描述的定制文件应放在 `/data/gitea` 目录中。如果使用主机卷，则访问这些文件非常容易；对于命名卷，可以通过另一个容器或通过直接访问 `/var/lib/docker/volumes/gitea_gitea/_data` 来完成。安装后，配置文件将保存在 `/data/gitea/conf/app.ini` 中。

## 升级

:exclamation::exclamation: **确保已将数据卷到 Docker 容器外部的某个位置** :exclamation::exclamation:

要将安装升级到最新版本：

```bash
# Edit `docker-compose.yml` to update the version, if you have one specified
# Pull new images
docker-compose pull
# Start a new container, automatically removes old one
docker-compose up -d
```

## 使用环境变量管理部署

除了上面的环境变量之外，`app.ini` 中的任何设置都可以使用以下形式的环境变量进行设置或覆盖：`GITEA__SECTION_NAME__KEY_NAME`。 每次 docker 容器启动时都会应用这些设置。 完整信息在[这里](https://github.com/go-gitea/gitea/tree/master/contrib/environment-to-ini)。

```bash
...
services:
  server:
    environment:
      - GITEA__mailer__ENABLED=true
      - GITEA__mailer__FROM=${GITEA__mailer__FROM:?GITEA__mailer__FROM not set}
      - GITEA__mailer__PROTOCOL=smtps
      - GITEA__mailer__HOST=${GITEA__mailer__HOST:?GITEA__mailer__HOST not set}
      - GITEA__mailer__USER=${GITEA__mailer__USER:-apikey}
      - GITEA__mailer__PASSWD="""${GITEA__mailer__PASSWD:?GITEA__mailer__PASSWD not set}"""
```

Gitea 将为每次新安装自动生成新的 `SECRET_KEY` 并将它们写入 `app.ini`。 如果您想手动设置 `SECRET_KEY`，您可以使用以下 docker 命令来使用 Gitea 内置的[方法](administration/command-line.md#generate)生成 `SECRET_KEY`。 安装后请妥善保管您的 `SECRET_KEY`，如若丢失则无法解密已加密的数据。

以下命令将向 `stdout` 输出一个新的 `SECRET_KEY` 和 `INTERNAL_TOKEN`，然后您可以将其放入环境变量中。

```bash
docker run -it --rm gitea/gitea:1 gitea generate secret SECRET_KEY
docker run -it --rm  gitea/gitea:1 gitea generate secret INTERNAL_TOKEN
```

```yaml
...
services:
  server:
    environment:
      - GITEA__security__SECRET_KEY=[value returned by generate secret SECRET_KEY]
      - GITEA__security__INTERNAL_TOKEN=[value returned by generate secret INTERNAL_TOKEN]
```

## SSH 容器直通

由于 SSH 在容器内运行，因此，如果需要 SSH 支持，则需要将 SSH 从主机传递到容器。一种选择是在非标准端口上运行容器 SSH（或将主机端口移至非标准端口）。另一个可能更直接的选择是将 SSH 连接从主机转发到容器。下面将说明此设置。

本指南假定您已经在名为 `git` 的主机上创建了一个用户，该用户与容器值 `USER_UID`/`USER_GID` 共享相同的 `UID`/`GID`。这些值可以在 `docker-compose.yml` 中设置为环境变量：

```bash
environment:
  - USER_UID=1000
  - USER_GID=1000
```

接下来将主机的 `/home/git/.ssh` 装入容器。否则，SSH 身份验证将无法在容器内运行。

```bash
volumes:
  - /home/git/.ssh/:/data/git/.ssh
```

现在，需要在主机上创建 SSH 密钥对。该密钥对将用于向主机验证主机上的 `git` 用户。

```bash
sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
```

在下一步中，需要在主机上创建一个名为 `/usr/local/bin/gitea` 的文件（具有可执行权限）。该文件将发出从主机到容器的 SSH 转发。将以下内容添加到 `/usr/local/bin/gitea`：

```bash
ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
```

为了使转发正常工作，需要将容器（22）的 SSH 端口映射到 `docker-compose.yml` 中的主机端口 2222。由于此端口不需要暴露给外界，因此可以将其映射到主机的 `localhost`：

```bash
ports:
  # [...]
  - "127.0.0.1:2222:22"
```

另外，主机上的 `/home/git/.ssh/authorized_keys` 需要修改。它需要以与 Gitea 容器内的 `authorized_keys` 相同的方式进行操作。因此，将您在上面创建的密钥（“Gitea 主机密钥”）的公共密钥添加到 `~/git/.ssh/authorized_keys`。这可以通过 `echo "$(cat /home/git/.ssh/id_rsa.pub)" >> /home/git/.ssh/authorized_keys` 完成。重要提示：来自 `git` 用户的公钥需要“按原样”添加，而通过 Gitea 网络界面添加的所有其他公钥将以 `command="/app [...]` 作为前缀。

该文件应该看起来像：

```bash
# SSH pubkey from git user
ssh-rsa <Gitea Host Key>

# other keys from users
command="/usr/local/bin/gitea --config=/data/gitea/conf/app.ini serv key-1",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <user pubkey>
```

这是详细的说明，当发出 SSH 请求时会发生什么：

1. 使用 `git` 用户向主机发出 SSH 请求，例如 `git clone git@domain:user/repo.git`。
2. 在 `/home/git/.ssh/authorized_keys` 中，该命令执行 `/usr/local/bin/gitea` 脚本。
3. `/usr/local/bin/gitea` 将 SSH 请求转发到端口 2222，该端口已映射到容器的 SSH 端口（22）。
4. 由于 `/home/git/.ssh/authorized_keys` 中存在 `git` 用户的公钥，因此身份验证主机 → 容器成功，并且 SSH 请求转发到在 docker 容器中运行的 Gitea。

如果在 Gitea Web 界面中添加了新的 SSH 密钥，它将以与现有密钥相同的方式附加到 `.ssh/authorized_keys` 中。

**注意**

SSH 容器直通仅在以下情况下有效

- 在容器中使用 `opensshd`
- 如果未将 `AuthorizedKeysCommand` 与 `SSH_CREATE_AUTHORIZED_KEYS_FILE = false` 结合使用以禁用授权文件密钥生成
- `LOCAL_ROOT_URL` 不变
