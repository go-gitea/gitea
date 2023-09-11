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

## 使用环境变量配置Gitea内的用户

- `USER`: **git**：在容器中运行Gitea的用户的用户名。
- `USER_UID`: **1000**：在容器中运行Gitea的用户的UID（Unix用户ID）。如果使用主机卷（host volumes），请将其匹配到`/data`卷的所有者的UID（对于命名卷，这不是必需的）。
- `USER_GID`: **1000**：在容器中运行Gitea的用户的GID（Unix组ID）。如果使用主机卷（host volumes），请将其匹配到`/data`卷的所有者的GID（对于命名卷，这不是必需的）。

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

### 理解Gitea的SSH访问逻辑(不使用穿透)

要理解需要发生什么，首先需要了解不使用穿透的情况下会发生什么。因此，我们将尝试描述这一过程：

1. 客户端通过网页将他们的SSH公钥添加到Gitea。
2. Gitea将为此密钥在其运行的用户`git`的`.ssh/authorized_keys`文件中添加一个条目。
3. 此条目除了包含公钥，还包含一个`command=`选项。正是这个命令使得`Gitea`能够将此密钥与客户端用户匹配并进行身份验证。
4. 然后，客户端使用`git`用户进行SSH请求，例如`git clone git@domain:user/repo.git`。
5. 客户端将尝试与服务器进行身份验证，逐个将一个或多个公钥传递给服务器。
6. 对于客户端提供的每一个密钥，SSH 服务器都会首先检查其配置中的 `AuthorizedKeysCommand`（授权密钥命令），看看公钥是否匹配，然后再检查 `git`用户的 `authorized_keys`（授权密钥）文件。
7. 将选择与之匹配的第一个条目，假设这是一个`Gitea`条目，则`command=`将被执行。
8. SSH服务器为`git`用户创建一个用户会话，并使用`git`用户的shell运行`command=`
9. 这将运行`gitea serv`命令，它接管了余下的SSH会话并管理Gitea对git命令的身份验证和授权。

现在，为了使SSH穿透正常工作，我们需要使宿主机SSH与Gitea用户的公钥进行匹配，然后在Docker容器内上运行`gitea serv`。有多种方法可以实现这一点。但是，所有这些方法都需要将有关Docker的一些信息传递给宿主机。

### SSHing Shim(`authorized_keys`)

在这个方案中，宿主机只需使用 gitea 创建的`authorized_keys`，但在第 9 步，宿主机上运行的 `gitea`命令是一个shim，它实际上是运行ssh进入docker容器，然后运行真正的容器内部的`gitea`本身。

- 要使转发生效，需要在 `docker-compose.yml` 中将容器的 SSH 端口（22）映射到主机端口 2222。由于该端口不需要对外公开，因此可以映射到主机的 `localhost` 端口：

  ```yaml
  ports:
    # [...]
    - "127.0.0.1:2222:22"
  ```

- 然后在宿主机上创建 `git` 用户，容器的 `USER_UID`/ `USER_GID` 值需要与宿主机中`git`用户的 `UID`/`GID` 与保持一致。这些值可以在 `docker-compose.yml` 中通过环境变量进行设置：

  ```yaml
  environment:
    - USER_UID=1000
    - USER_GID=1000
  ```

- 将宿主机的 `/home/git/.ssh` 挂载到容器中。这将确保宿主机的 `git` 用户和容器的 `git` 用户共享 `authorized_keys` 文件，否则 SSH 身份验证就无法在容器内运行。

  ```yaml
  volumes:
    - /home/git/.ssh/:/data/git/.ssh
  ```

- 现在需要在宿主机上创建 SSH 密钥对。这个密钥对将用于验证宿主机上的 `git` 用户到容器的身份。以管理用户身份在宿主机上运行：（我们所说的管理用户是指可以 sudo 到 root 的用户）

  ```bash
  sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
  ```

- 请注意，根据`ssh`的本地版本，您可能需要考虑在此处使用 `-t ecdsa`。

- 现在需要修改宿主机上的 `/home/git/.ssh/authorized_keys`。它需要以与 Gitea 容器中的 `authorized_keys` 相同的方式运行。因此，将上文创建的密钥（`Gitea Host Key`）的公钥添加到 `~/git/.ssh/authorized_keys` 中。以管理用户身份在主机上运行：

  ```bash
  sudo -u git cat /home/git/.ssh/id_rsa.pub | sudo -u git tee -a /home/git/.ssh/authorized_keys
  sudo -u git chmod 600 /home/git/.ssh/authorized_keys
  ```

  **注意**： `git`用户的密钥需要 **按原样**添加，而通过 Gitea Web 界面添加的所有其他密钥将以 `command="/usr [...]` 为前缀。

  然后，"/home/git/.ssh/authorized_keys "应该看起来像这样

  ```bash
  # SSH pubkey from git user
  ssh-rsa <Gitea Host Key>

  # other keys from users
  command="/usr/local/bin/gitea --config=/data/gitea/conf/app.ini serv key-1",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <user pubkey>
  ```

- 下一步是创建宿主机的 `gitea` 命令，将命令从主机转发到容器。该文件的名称取决于 Gitea 的版本：
  - 对于 Gitea v1.16.0+。以管理用户身份在主机上运行：

    ```bash
    cat <<"EOF" | sudo tee /usr/local/bin/gitea
    #!/bin/sh
    ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
    EOF
    sudo chmod +x /usr/local/bin/gitea
    ```

下面详细解释了 SSH 请求时发生的情况：

1. 客户端通过网页将其 SSH 公钥添加到 Gitea。
2. 容器中的 Gitea 会在其运行用户 `git` 的 `.ssh/authorized_keys` 文件中添加该密钥的条目。
    - 不过，由于主机上的 `/home/git/.ssh/` 被挂载为 `/data/git/.ssh`，这意味着该密钥也已添加到宿主机 `git` 用户的 `authorized_keys` 文件中。
3. 该条目包含公钥，但也有一个 `command=` 选项。
    - 该命令所在的位置不仅需要与容器中 Gitea 二进制文件的位置相一致，也需要与主机上 shim 的位置相匹配。
4. 然后，客户端使用 `git`用户向宿主机的SSH服务器发出 SSH 请求，如 `git clone git@domain:user/repo.git`。
5. 客户端将尝试与服务器进行身份验证，依次向宿主机传递一个或多个公钥。
6. 对于客户端提供的每个密钥，主机 SSH 服务器都会首先检查其配置中的 `AuthorizedKeysCommand`，看是否与公钥匹配，然后再尝试匹配主机上 `git` 用户的 `authorized_keys`（授权密钥）文件。
    - 由于宿主机上的 `/home/git/.ssh/` 被挂载到容器内部的 `/data/git/.ssh`，这意味着他们添加到 Gitea web 的公钥密钥会被找到
7. 将选择第一个匹配的条目，假定这是一个 Gitea 条目，这将执行 `command=` 命令。
8. 主机 SSH 服务器会为`git`用户创建一个用户会话，并使用主机`git`用户的 shell 运行`command=`中指定的命令。
9. 这意味着主机会运行宿主机的 `/usr/local/bin/gitea` shim，该 shim 会打开从主机到容器的 SSH，并将其余命令参数直接传递给容器上的 `/usr/local/bin/gitea`。
10. 这意味着运行容器 `gitea serv`，接管 SSH 会话的其余控制权，并管理 gitea 认证和 git 命令的授权。

**注意**

使用 "authorized_keys "的 SSH 容器直通仅在以下情况下有效

- 容器中启用了`opensshd`服务
- 如果 `AuthorizedKeysCommand`未与`SSH_CREATE_AUTHORIZED_KEYS_FILE=false`结合使用，则会禁止授权文件密钥生成
- `LOCAL_ROOT_URL` 不会更改（取决于更改情况）

如果尝试在主机上运行 `gitea`命令，实际上会尝试 ssh 到容器，然后在容器内部运行 `gitea` 命令。

切勿将 `Gitea Host Key` 作为 SSH 密钥添加到 Gitea 界面上的用户。

### SSHing Shell (with authorized_keys)

在这个方案中，主机只需使用 gitea 创建的`authorized_keys`，但在上面的第 8 步，我们将主机运行的 shell 改为直接 ssh 到 docker，然后在那里运行 shell。这意味着运行的 `gitea` 才是真正的 docker `gitea`。

- 在这种情况下，我们的设置与 SSHing Shim 相同，只是不创建`/usr/local/bin/gitea`，而是为 git 用户创建一个新 shell。
以管理用户身份在主机上运行:

  ```bash
  cat <<"EOF" | sudo tee /home/git/ssh-shell
  #!/bin/sh
  shift
  ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $@"
  EOF
  sudo chmod +x /home/git/ssh-shell
  sudo usermod -s /home/git/ssh-shell git
  ```

  请注意，如果你以后尝试以 git 用户身份登录，你就会直接 ssh 到 docker。

下面详细解释了 SSH 请求时发生的情况：

1. 客户端通过网页将其 SSH 公钥添加到 Gitea。
2. 容器中的 Gitea 会在其运行用户 `git` 的 `.ssh/authorized_keys` 文件中添加该密钥的条目。
    - 不过，由于主机上的 `/home/git/.ssh/` 被挂载为 `/data/git/.ssh`，这意味着该密钥也已添加到宿主机 `git` 用户的 `authorized_keys` 文件中。
3. 该条目包含公钥，但也有一个 `command=` 选项。
    - 该命令所在的位置不仅需要与容器中 Gitea 二进制文件的位置相一致，也需要与主机上 shim 的位置相匹配。
4. 然后，客户端使用 `git`用户向宿主机的SSH服务器发出 SSH 请求，如 `git clone git@domain:user/repo.git`。
5. 客户端将尝试与服务器进行身份验证，依次向宿主机传递一个或多个公钥。
6. 对于客户端提供的每个密钥，主机 SSH 服务器都会首先检查其配置中的 `AuthorizedKeysCommand`，看是否与公钥匹配，然后再尝试匹配主机上 `git` 用户的 `authorized_keys`（授权密钥）文件。
    - 由于宿主机上的 `/home/git/.ssh/` 被挂载到容器内部的 `/data/git/.ssh`，这意味着他们添加到 Gitea web 的公钥密钥会被找到
7. 将选择第一个匹配的条目，假定这是一个 Gitea 条目，这将执行 `command=` 命令。
8. 主机 SSH 服务器会为`git`用户创建一个用户会话，并使用主机`git`用户的 shell 运行`command=`中指定的命令。
9. 主机 `git` 用户的 shell 现在是我们的 `ssh-shell`，它会打开主机到容器的 SSH 连接（为容器 `git` 打开容器上的 shell）。
10. 这意味着运行容器 `gitea serv`，接管 SSH 会话的其余控制权，并管理 gitea 认证和 git 命令的授权。

**注意**

使用 "authorized_keys "的 SSH 容器直通仅在以下情况下有效

- 容器中启用了`opensshd`服务
- 如果 `AuthorizedKeysCommand`未与`SSH_CREATE_AUTHORIZED_KEYS_FILE=false`结合使用，则会禁止授权文件密钥生成
- `LOCAL_ROOT_URL` 不会更改（取决于更改情况）

如果以后在主机上尝试以 `git` 用户登录，就会直接 ssh 到 docker。

切勿将 `Gitea Host Key` 作为 SSH 密钥添加到 Gitea 界面上的用户。

### Docker Shell with AuthorizedKeysCommand

`AuthorizedKeysCommand`路由提供了另一种选择，它不需要对组成文件或 `authorized_keys` 进行太多更改，但需要更改宿主机的SSH配置文件 `/etc/sshd_config`。

在这个方案中，宿主机的SSH服务使用的是`AuthorizedKeysCommand`，而不是依赖于共享 gitea容器的`authorized_keys`文件。在上面的第 8 步中，我们继续使用一个特殊的 shell 来执行进入 docker，然后在那里运行 shell。这意味着随后运行的 `gitea` 才是真正的 docker `gitea`。

- 在宿主机上创建有权限执行 `docker exec`命令的 `git` 用户。
- 在这里我们假设运行Gitea实例的容器是的名称为`gitea`
- 修改 `git` 用户的 shell，使用 `docker exec` 将命令转发给容器内的 `sh` 可执行文件。 以管理用户身份在主机上运行：

  ```bash
  cat <<"EOF" | sudo tee /home/git/docker-shell
  #!/bin/sh
  /usr/bin/docker exec -i --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
  EOF
  sudo chmod +x /home/git/docker-shell
  sudo usermod -s /home/git/docker-shell git
  ```

现在，在宿主机上以 `git` 用户登录的所有尝试都会被转发到 docker容器中，包括 `SSH_ORIGINAL_COMMAND`。现在，我们需要在宿主机上设置 SSH 身份验证。

我们将利用 [SSH AuthorizedKeysCommand]（administration/command-line.md#keys）来匹配 Gitea 接受的密钥。

在宿主机上的 `/etc/ssh/sshd_config` 中添加以下代码：

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/docker exec -i gitea /usr/local/bin/gitea keys -c /data/gitea/conf/app.ini -e git -u %u -t %t -k %k
```

(从 1.16.0 版起，无需设置 `-c /data/gitea/conf/app.ini` 选项)。

最后重启 SSH 服务器。以管理用户身份在主机上运行

```bash
sudo systemctl restart sshd
```

下面详细解释了 SSH 请求时发生的情况：

1. 客户端通过网页将其 SSH 公钥添加到 Gitea。
2. 容器中的 Gitea 会在其数据库中添加该密钥的条目。
3. 然后，客户端使用`git`用户向主机 SSH 服务器发出 SSH 请求，例如 `git clone git@domain:user/repo.git`。
4. 客户端会尝试与服务器进行身份验证，依次向主机传递一个或多个公钥。
5. 对于客户端提供的每一个密钥，主机 SSH 服务器都会检查其配置中的`AuthorizedKeysCommand`(授权密钥命令)。
6. 宿主机运行`AuthorizedKeysCommand`中指定的命令，执行 docker，然后运行 `gitea keys`命令。
7. docker 上的 Gitea 会在它的数据库中查找公钥是否匹配，并返回一个类似于 `authorized_keys` 命令的条目。
8. 这个条目包含公钥，但也有一个 `command=` 选项，与容器上的 Gitea 二进制文件的位置相匹配。
9. 主机 SSH 服务器会为 `git` 用户创建一个用户会话，并使用主机 `git` 用户的 shell 运行 `command=` 命令。
10. 主机 `git` 用户的 shell 现在是我们的 `docker-shell`，它使用 `docker exec` 为容器上的 `git` 用户打开 shell。
11. 容器 shell 现在会运行 `command=` 选项，这意味着容器 `gitea serv` 会被运行，接管 SSH 会话的其余部分，并管理 gitea 认证和 git 命令的授权。

**注意**

只有在以下情况下，使用 `AuthorizedKeysCommand` 的 Docker shell 直通才会起作用

- 主机上的 `git` 用户被允许运行 `docker exec` 命令。

如果你以后尝试以主机上的 `git` 用户登录，你就会直接执行 `docker exec` 到 docker。

与上述类似，也可以创建一个执行 Docker 的 shim。

### SSH Shell with AuthorizedKeysCommand

如上所述，为主机上的 `git` 用户创建一个密钥，并将其添加到 docker 的 `/data/git/.ssh/authorized_keys` 中，最后如上所述创建并设置 `ssh-shell`。

在主机上的 `/etc/ssh/sshd_config` 中添加以下代码块：

```bash
Match git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 /usr/local/bin/gitea keys -c /data/gitea/conf/app.ini -e git -u %u -t %t -k %k
```

(从 1.16.0 版起，无需设置 `-c /data/gitea/conf/app.ini` 选项)。

最后重启 SSH 服务器。以管理用户身份在主机上运行

```bash
sudo systemctl restart sshd
```

以下是 SSH 请求发生时的详细说明：

1. 客户端使用网页向 Gitea 添加 SSH 公钥。
2. 容器中的 Gitea 将在其数据库中添加该密钥的条目。
3. 然后，客户端使用 `git`用户向主机 SSH 服务器发出 SSH 请求，例如 `git clone git@domain:user/repo.git`。
4. 客户端会尝试与服务器进行身份验证，依次向主机传递一个或多个公钥。
5. 对于客户端提供的每一个密钥，主机 SSH 服务器都会检查其配置中的`AuthorizedKeysCommand`。
6. 主机运行上述 `AuthorizedKeysCommand`，SSH 登录 docker，然后运行`gitea keys`命令。
7. docker 上的 Gitea 会在它的数据库中查找公钥是否匹配，并返回一个类似于 `authorized_keys` 命令的条目。
8. 该条目包含公钥，但也有一个 `command=` 选项，与容器上的 Gitea 二进制文件的位置相匹配。
9. 主机 SSH 服务器会为 `git` 用户创建一个用户会话，并使用主机 `git` 用户的 shell 运行 `command=` 命令。
10. 主机 `git` 用户的 shell 现在就是我们的 `git-shell`，它使用 SSH 为容器上的 `git` 用户打开 shell。
11. 容器 shell 现在会运行 `command=` 选项，这意味着容器 `gitea serv` 会被运行，接管 SSH 会话的其余部分，并管理 gitea 认证和 git 命令的授权。

**注意**

使用 `AuthorizedKeysCommand` 的 SSH 容器直通仅在以下情况下有效

- 容器上运行了 `opensshd

如果以后尝试在主机上以 `git` 用户登录，就会直接 `ssh` 到 docker。

切勿在 Gitea 界面上将 `Gitea 主机密钥 ` 添加为用户的 SSH 密钥。

SSH shims 的创建方法与上述类似。
