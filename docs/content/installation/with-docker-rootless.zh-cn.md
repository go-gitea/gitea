---
date: "2020-02-09T20:00:00+02:00"
title: "使用 Docker 安装 (rootless)"
slug: "install-with-docker-rootless"
sidebar_position: 60
toc: false
draft: false
aliases:
  - /zh-cn/install-with-docker-rootless
menu:
  sidebar:
    parent: "installation"
    name: "使用 Docker 安装 (rootless)"
    sidebar_position: 60
    identifier: "install-with-docker-rootless"
---

# 使用 Docker 安装

Gitea 在其 Docker Hub 组织中提供自动更新的 Docker 镜像。您可以始终使用最新的稳定标签，或使用其他处理 Docker 镜像更新的服务。

rootless 镜像使用 Gitea 内部 SSH 功能来提供 Git 协议，但不支持 OpenSSH。

本参考设置指南将用户引导通过基于 `docker-compose` 的设置。但是，`docker-compose` 的安装超出了本文档的范围。要安装`docker-compose` 本身， 请按照官方的 [安装说明](https://docs.docker.com/compose/install/)进行操作。

## 基础设置

最简单的设置只需创建一个卷和一个网络，并将 `gitea/gitea:latest-rootless` 镜像作为服务启动。由于没有可用的数据库，可以使用 SQLite3 来初始化一个。

创建一个名为 `data` 和 `config`:

```sh
mkdir -p gitea/{data,config}
cd gitea
touch docker-compose.yml
```

然后将以下内容粘贴到名为 `docker-compose.yml` 的文件中：

```yaml
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

请注意，卷应由在配置文件中指定的UID/GID的用户/组所有。默认情况下，Docker中的Gitea将使用uid:1000 gid:1000。如果需要，您可以使用以下命令设置这些文件夹的所有权：

```sh
sudo chown 1000:1000 config/ data/
```

> 如果未为卷设置正确的权限，容器可能无法启动。

对于稳定版本，您可以使用 `:latest-rootless`、`:1-rootless`，或指定特定的版本，如: `@version@-rootless`。如果您想使用最新的开发版本，则可以使用 `:dev-rootless` 标签。如果您想运行发布分支的最新提交，可以使用 `:1.x-dev-rootless` 标签，其中 x是 Gitea 的次要版本号（例如:`1.16-dev-rootless`）。

## 自定义端口

要将集成的SSH和Web服务器绑定到不同的端口，请调整端口部分。通常只需更改主机端口并保持容器内的端口不变。

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-      - "3000:3000"
-      - "2222:2222"
+      - "80:3000"
+      - "22:2222"
```

## MySQL 数据库

要将 Gitea 与 MySQL 数据库结合使用，请对上面创建的 `docker-compose.yml` 文件进行以下更改。

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
+    environment:
+      - GITEA__database__DB_TYPE=mysql
+      - GITEA__database__HOST=db:3306
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
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
+    volumes:
+      - ./mysql:/var/lib/mysql
```

## PostgreSQL 数据库

要将 Gitea 与 PostgreSQL 数据库结合使用，请对上面创建的 `docker-compose.yml` 文件进行以下更改。

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    environment:
+      - GITEA__database__DB_TYPE=postgres
+      - GITEA__database__HOST=db:5432
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
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
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## 命名卷 (Named Volumes)

要使用命名卷 (Named Volumes) 而不是主机卷 (Host Volumes)，请在 `docker-compose.yml` 配置中定义和使用命名卷。这样的更改将自动创建所需的卷。您不需要担心权限问题，Docker 会自动处理。

```diff
version: "2"

+volumes:
+  gitea-data:
+    driver: local
+  gitea-config:
+    driver: local
+
services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
    volumes:
-      - ./data:/var/lib/gitea
+      - gitea-data:/var/lib/gitea
-      - ./config:/etc/gitea
+      - gitea-config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

MySQL 或 PostgreSQL 容器需要单独创建。

## 自定义用户

你可以选择使用自定义用户 (遵循 --user 标志定义 https://docs.docker.com/engine/reference/run/#user)。
例如，要克隆主机用户 `git` 的定义，请使用命令 `id -u git` 并将其添加到 `docker-compose.yml` 文件中：
请确用户对保挂载的文件夹具有写权限。

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
+    user: 1001
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

## 启动

要启动基于 `docker-compose` 的这个设置，请执行 `docker-compose up -d`，以在后台启动 Gitea。使用 `docker-compose ps` 命令可以查看 Gitea 是否正确启动。可以使用 `docker-compose logs` 命令查看日志。

要关闭设置，请执行 `docker-compose down` 命令。这将停止和终止容器，但卷仍将存在。

注意：如果在 HTTP 上使用的是非 3000 端口，请将 app.ini 更改为匹配 `LOCAL_ROOT_URL = http://localhost:3000/`。

## 安装

在通过 `docker-compose` 启动 Docker 设置后，可以使用喜爱的浏览器访问 Gitea，完成安装过程。访问 `http://<服务器-IP>:3000` 并按照安装向导进行操作。如果数据库是使用上述文档中的 `docker-compose` 设置启动的，请注意必须使用 `db` 作为数据库主机名。

# 自定义

自定义文件的位置位于 `/var/lib/gitea/custom` 目录中，可以在这里找到有关自定义的文件说明。如果使用主机卷（host volumes），很容易访问这些文件；如果使用命名卷（named volumes），则可以通过另一个容器或直接访问 `/var/lib/docker/volumes/gitea_gitea/_/var_lib_gitea` 来进行访问。在安装后，配置文件将保存在 `/etc/gitea/app.ini` 中。

# 升级

:exclamation::exclamation: **确保您已将数据卷迁移到 Docker 容器之外的其他位置** :exclamation::exclamation:

要将安装升级到最新版本，请按照以下步骤操作：

```
# 如果在 docker-compose.yml 中指定了版本，请编辑该文件以更新版本
# 拉取新的镜像
docker-compose pull
# 启动一个新的容器，自动移除旧的容器
docker-compose up -d
```

# 从标准镜像升级

- 备份您的设置
- 将卷挂载点从 `/data` 更改为 `/var/lib/gitea`
- 如果使用了自定义的 `app.ini`，请将其移动到新的挂载到 `/etc/gitea` 的卷中
- 将卷中的文件夹（gitea）重命名为 custom
- 如果需要，编辑 `app.ini`
  - 设置 `START_SSH_SERVER = true`
- 使用镜像 `gitea/gitea:@version@-rootless`

## 使用环境变量管理部署

除了上述的环境变量外，`app.ini` 中的任何设置都可以通过形式为 `GITEA__SECTION_NAME__KEY_NAME` 的环境变量进行设置或覆盖。这些设置在每次 Docker 容器启动时都会生效。完整信息请参考[这里](https://github.com/go-gitea/gitea/tree/main/contrib/environment-to-ini).

这些环境变量可以在 `docker-compose.yml` 中传递给 Docker 容器。以下示例将启用 SMTP 邮件服务器，如果主机上设置了所需的环境变量 GITEA__mailer__FROM、GITEA__mailer__HOST、GITEA__mailer__PASSWD，或者在与 `docker-compose.yml` 相同目录中的 `.env` 文件中设置了这些环境变量：

```bash
...
services:
  server:
    environment:
      - GITEA__mailer__ENABLED=true
      - GITEA__mailer__FROM=${GITEA__mailer__FROM:?GITEA__mailer__FROM not set}
      - GITEA__mailer__PROTOCOL=smtp
      - GITEA__mailer__HOST=${GITEA__mailer__HOST:?GITEA__mailer__HOST not set}
      - GITEA__mailer__IS_TLS_ENABLED=true
      - GITEA__mailer__USER=${GITEA__mailer__USER:-apikey}
      - GITEA__mailer__PASSWD="""${GITEA__mailer__PASSWD:?GITEA__mailer__PASSWD not set}"""
```

要设置所需的 TOKEN 和 SECRET 值，可以使用 Gitea 的内置[生成使用函数](administration/command-line.md#generate).

# SSH 容器透传

由于 SSH 在容器内运行，如果需要 SSH 支持，需要将 SSH 从主机透传到容器。一种选择是在容器内运行 SSH，并使用非标准端口（或将主机端口移动到非标准端口）。另一种可能更直接的选择是将主机上的 SSH 命令转发到容器。下面解释了这种设置。

本指南假设您已在主机上创建了一个名为 `git` 的用户，并具有运行 `docker exec` 的权限，并且 Gitea 容器的名称为 `gitea`。您需要修改该用户的 shell，以将命令转发到容器内的 `sh` 可执行文件，使用 `docker exec`。

首先，在主机上创建文件 `/usr/local/bin/gitea-shell`，并填入以下内容：

```bash
#!/bin/sh
/usr/bin/docker exec -i --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
```

注意上述 docker 命令中的 `gitea` 是容器的名称。如果您的容器名称不同，请记得更改。

还应确保正确设置了 shell 包装器的权限：

```bash
sudo chmod +x /usr/local/bin/gitea-shell
```

一旦包装器就位，您可以将其设置为 `git` 用户的 shell：

```bash
sudo usermod -s /usr/local/bin/gitea-shell git
```

现在，所有的 SSH 命令都会被转发到容器，您需要在主机上设置 SSH 认证。这可以通过利用 [SSH AuthorizedKeysCommand](administration/command-line.md#keys) 来匹配 Gitea 接受的密钥。在主机的 `/etc/ssh/sshd_config` 文件中添加以下代码块：

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/docker exec -i gitea /usr/local/bin/gitea keys -c /etc/gitea/app.ini -e git -u %u -t %t -k %k
```

（从 1.16.0 开始，您将不需要设置 `-c /etc/gitea/app.ini` 选项。）

剩下的就是重新启动 SSH 服务器：

```bash
sudo systemctl restart sshd
```

**注意**

这实际上并没有使用 Docker 的 SSH，而是仅仅使用了围绕它的命令。
从理论上讲，您可以不运行内部的 SSH 服务器。
