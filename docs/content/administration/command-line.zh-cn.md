---
date: "2023-05-23T09:00:00+08:00"
title: "Gitea 命令行"
slug: "command-line"
sidebar_position: 1
toc: false
draft: false
aliases:
  - /zh-cn/command-line
menu:
  sidebar:
    parent: "administration"
    name: "Gitea 命令行"
    sidebar_position: 1
    identifier: "command-line"
---

# 命令行

## 用法

`gitea [全局选项] 命令 [命令或全局选项] [参数...]`

## 全局选项

所有全局选项均可被放置在命令级别。

- `--help`，`-h`：显示帮助文本并退出。可选。
- `--version`，`-v`：显示版本信息并退出。可选。 (示例：`Gitea version 1.1.0+218-g7b907ed built with: bindata, sqlite`)。
- `--custom-path path`，`-C path`：Gitea 自定义文件夹的路径。可选。 (默认值：`AppWorkPath`/custom 或 `$GITEA_CUSTOM`)。
- `--config path`，`-c path`：Gitea 配置文件的路径。可选。 (默认值：`custom`/conf/app.ini)。
- `--work-path path`，`-w path`：Gitea 的 `AppWorkPath`。可选。 (默认值：LOCATION_OF_GITEA_BINARY 或 `$GITEA_WORK_DIR`)

注意：默认的 custom-path、config 和 work-path 也可以在构建时更改（如果需要）。

## 命令

### web

启动服务器：

- 选项：
  - `--port number`，`-p number`：端口号。可选。 (默认值：3000)。覆盖配置文件中的设置。
  - `--install-port number`：运行安装页面的端口号。可选。 (默认值：3000)。覆盖配置文件中的设置。
  - `--pid path`，`-P path`：Pid 文件的路径。可选。
  - `--quiet`，`-q`：只在控制台上输出 Fatal 日志，用于在设置日志之前发出的日志。
  - `--verbose`：在控制台上输出跟踪日志，用于在设置日志之前发出的日志。
- 示例：
  - `gitea web`
  - `gitea web --port 80`
  - `gitea web --config /etc/gitea.ini --pid /some/custom/gitea.pid`
- 注意：
  - Gitea 不应以 root 用户身份运行。要绑定到低于 1024 的端口，您可以在 Linux 上使用 setcap 命令：`sudo setcap 'cap_net_bind_service=+ep' /path/to/gitea`。每次更新 Gitea 都需要重新执行此操作。

### admin

管理员操作：

- 命令：
  - `user`：
    - `list`：
      - 选项：
        - `--admin`：仅列出管理员用户。可选。
      - 描述：列出所有现有用户。
      - 示例：
        - `gitea admin user list`
    - `delete`：
      - 选项：
        - `--email`：要删除的用户的电子邮件。
        - `--username`：要删除的用户的用户名。
        - `--id`：要删除的用户的ID。
        - 必须提供 `--id`、`--username` 或 `--email` 中的一个。如果提供多个，则所有条件必须匹配。
      - 示例：
        - `gitea admin user delete --id 1`
    - `create`：
      - 选项：
        - `--name value`：用户名。必填。自 Gitea 1.9.0 版本起，请改用 `--username` 标志。
        - `--username value`：用户名。必填。Gitea 1.9.0 新增。
        - `--password value`：密码。必填。
        - `--email value`：邮箱。必填。
        - `--admin`：如果提供此选项，将创建一个管理员用户。可选。
        - `--access-token`：如果提供，将为用户创建访问令牌。可选。（默认值：false）。
        - `--must-change-password`：如果提供，创建的用户将在初始登录后需要选择一个新密码。可选。（默认值：true）。
        - `--random-password`：如果提供，将使用随机生成的密码作为创建用户的密码。`--password` 的值将被忽略。可选。
        - `--random-password-length`：如果提供，将用于配置随机生成密码的长度。可选。（默认值：12）
      - 示例：
        - `gitea admin user create --username myname --password asecurepassword --email me@example.com`
    - `change-password`：
      - 选项：
        - `--username value`，`-u value`：用户名。必填。
        - `--password value`，`-p value`：新密码。必填。
      - 示例：
        - `gitea admin user change-password --username myname --password asecurepassword`
    - `must-change-password`：
      - 参数：
        - `[username...]`：需要更改密码的用户
      - 选项：
        - `--all`，`-A`：强制所有用户更改密码
        - `--exclude username`，`-e username`：排除给定的用户。可以多次设置。
        - `--unset`：撤销对给定用户的强制密码更改
  - `regenerate`：
    - 选项：
      - `hooks`：重新生成所有仓库的 Git Hooks。
      - `keys`：重新生成 authorized_keys 文件。
    - 示例：
      - `gitea admin regenerate hooks`
      - `gitea admin regenerate keys`
  - `auth`：
    - `list`：
      - 描述：列出所有存在的外部认证源。
      - 示例：
        - `gitea admin auth list`
    - `delete`：
      - 选项：
        - `--id`：要删除的源的 ID。必填。
      - 示例：
        - `gitea admin auth delete --id 1`
    - `add-oauth`：
      - 选项：
        - `--name`：应用程序名称。
        - `--provider`：OAuth2 提供者。
        - `--key`：客户端 ID（Key）。
        - `--secret`：客户端密钥。
        - `--auto-discover-url`：OpenID Connect 自动发现 URL（仅在使用 OpenID Connect 作为提供程序时需要）。
        - `--use-custom-urls`：在 GitLab/GitHub OAuth 端点上使用自定义 URL。
        - `--custom-tenant-id`：在 OAuth 端点上使用自定义租户 ID。
        - `--custom-auth-url`：使用自定义授权 URL（GitLab/GitHub 的选项）。
        - `--custom-token-url`：使用自定义令牌 URL（GitLab/GitHub 的选项）。
        - `--custom-profile-url`：使用自定义配置文件 URL（GitLab/GitHub 的选项）。
        - `--custom-email-url`：使用自定义电子邮件 URL（GitHub 的选项）。
        - `--icon-url`：OAuth2 登录源的自定义图标 URL。
        - `--skip-local-2fa`：允许源覆盖本地 2FA。（可选）
        - `--scopes`：请求此 OAuth2 源的附加范围。（可选）
        - `--required-claim-name`：必须设置的声明名称，以允许用户使用此源登录。（可选）
        - `--required-claim-value`：必须设置的声明值，以允许用户使用此源登录。（可选）
        - `--group-claim-name`：提供此源的组名的声明名称。（可选）
        - `--admin-group`：管理员用户的组声明值。（可选）
        - `--restricted-group`：受限用户的组声明值。（可选）
        - `--group-team-map`：组与组织团队之间的 JSON 映射。（可选）
        - `--group-team-map-removal`：根据组自动激活团队成员资格的删除。（可选）
      - 示例：
        - `gitea admin auth add-oauth --name external-github --provider github --key OBTAIN_FROM_SOURCE --secret OBTAIN_FROM_SOURCE`
    - `update-oauth`：
      - 选项：
        - `--id`：要更新的源的 ID。必填。
        - `--name`：应用程序名称。
        - `--provider`：OAuth2 提供者。
        - `--key`：客户端 ID（Key）。
        - `--secret`：客户端密钥。
        - `--auto-discover-url`：OpenID Connect 自动发现 URL（仅在使用 OpenID Connect 作为提供程序时需要）。
        - `--use-custom-urls`：在 GitLab/GitHub OAuth 端点上使用自定义 URL。
        - `--custom-tenant-id`：在 OAuth 端点上使用自定义租户 ID。
        - `--custom-auth-url`：使用自定义授权 URL（GitLab/GitHub 的选项）。
        - `--custom-token-url`：使用自定义令牌 URL（GitLab/GitHub 的选项）。
        - `--custom-profile-url`：使用自定义配置文件 URL（GitLab/GitHub 的选项）。
        - `--custom-email-url`：使用自定义电子邮件 URL（GitHub 的选项）。
        - `--icon-url`：OAuth2 登录源的自定义图标 URL。
        - `--skip-local-2fa`：允许源覆盖本地 2FA。（可选）
        - `--scopes`：请求此 OAuth2 源的附加范围。
        - `--required-claim-name`：必须设置的声明名称，以允许用户使用此源登录。（可选）
        - `--required-claim-value`：必须设置的声明值，以允许用户使用此源登录。（可选）
        - `--group-claim-name`：提供此源的组名的声明名称。（可选）
        - `--admin-group`：管理员用户的组声明值。（可选）
        - `--restricted-group`：受限用户的组声明值。（可选）
      - 示例：
        - `gitea admin auth update-oauth --id 1 --name external-github-updated`
    - `add-smtp`：
      - 选项：
        - `--name`：应用程序名称。必填。
        - `--auth-type`：SMTP 认证类型（PLAIN/LOGIN/CRAM-MD5）。默认为 PLAIN。
        - `--host`：SMTP 主机。必填。
        - `--port`：SMTP 端口。必填。
        - `--force-smtps`：SMTPS 始终在端口 465 上使用。设置此选项以强制在其他端口上使用 SMTPS。
        - `--skip-verify`：跳过 TLS 验证。
        - `--helo-hostname`：发送 HELO 时使用的主机名。留空以发送当前主机名。
        - `--disable-helo`：禁用 SMTP helo。
        - `--allowed-domains`：留空以允许所有域。使用逗号（','）分隔多个域。
        - `--skip-local-2fa`：跳过 2FA 登录。
        - `--active`：启用此认证源。
        备注：
        `--force-smtps`、`--skip-verify`、`--disable-helo`、`--skip-local-2fs` 和 `--active` 选项可以采用以下形式使用：
        - `--option`、`--option=true` 以启用选项
        - `--option=false` 以禁用选项
        如果未指定这些选项，则在 `update-smtp` 中不会更改值，或者在 `add-smtp` 中将使用默认的 `false` 值。
      - 示例：
        - `gitea admin auth add-smtp --name ldap --host smtp.mydomain.org --port 587 --skip-verify --active`
    - `update-smtp`：
      - 选项：
        - `--id`：要更新的源的 ID。必填。
        - 其他选项与 `add-smtp` 共享
      - 示例：
        - `gitea admin auth update-smtp --id 1 --host smtp.mydomain.org --port 587 --skip-verify=false`
        - `gitea admin auth update-smtp --id 1 --active=false`
    - `add-ldap`：添加新的 LDAP（通过 Bind DN）认证源
      - 选项：
        - `--name value`：认证名称。必填。
        - `--not-active`：停用认证源。
        - `--security-protocol value`：安全协议名称。必填。
        - `--skip-tls-verify`：禁用 TLS 验证。
        - `--host value`：LDAP 服务器的地址。必填。
        - `--port value`：连接到 LDAP 服务器时使用的端口。必填。
        - `--user-search-base value`：用户帐户将在其中搜索的 LDAP 基础路径。必填。
        - `--user-filter value`：声明如何查找试图进行身份验证的用户记录的 LDAP 过滤器。必填。
        - `--admin-filter value`：指定是否应授予用户管理员特权的 LDAP 过滤器。
        - `--restricted-filter value`：指定是否应将用户设置为受限状态的 LDAP 过滤器。
        - `--username-attribute value`：用户 LDAP 记录中包含用户名的属性。
        - `--firstname-attribute value`：用户 LDAP 记录中包含用户名字的属性。
        - `--surname-attribute value`：用户 LDAP 记录中包含用户姓氏的属性。
        - `--email-attribute value`：用户 LDAP 记录中包含用户电子邮件地址的属性。必填。
        - `--public-ssh-key-attribute value`：用户 LDAP 记录中包含用户公共 SSH 密钥的属性。
        - `--avatar-attribute value`：用户 LDAP 记录中包含用户头像的属性。
        - `--bind-dn value`：在搜索用户时绑定到 LDAP 服务器的 DN。
        - `--bind-password value`：绑定 DN 的密码（如果有）。
        - `--attributes-in-bind`：在绑定 DN 上下文中获取属性。
        - `--synchronize-users`：启用用户同步。
        - `--page-size value`：搜索页面大小。
      - 示例：
        - `gitea admin auth add-ldap --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-search-base "ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(|(uid=%[1]s)(mail=%[1]s)))" --email-attribute mail`
    - `update-ldap`：更新现有的 LDAP（通过 Bind DN）认证源
      - 选项：
        - `--id value`：认证源的 ID。必填。
        - `--name value`：认证名称。
        - `--not-active`：停用认证源。
        - `--security-protocol value`：安全协议名称。
        - `--skip-tls-verify`：禁用 TLS 验证。
        - `--host value`：LDAP 服务器的地址。
        - `--port value`：连接到 LDAP 服务器时使用的端口。
        - `--user-search-base value`：用户帐户将在其中搜索的 LDAP 基础路径。
        - `--user-filter value`：声明如何查找试图进行身份验证的用户记录的 LDAP 过滤器。
        - `--admin-filter value`：指定是否应授予用户管理员特权的 LDAP 过滤器。
        - `--restricted-filter value`：指定是否应将用户设置为受限状态的 LDAP 过滤器。
        - `--username-attribute value`：用户 LDAP 记录中包含用户名的属性。
        - `--firstname-attribute value`：用户 LDAP 记录中包含用户名字的属性。
        - `--surname-attribute value`：用户 LDAP 记录中包含用户姓氏的属性。
        - `--email-attribute value`：用户 LDAP 记录中包含用户电子邮件地址的属性。
        - `--public-ssh-key-attribute value`：用户 LDAP 记录中包含用户公共 SSH 密钥的属性。
        - `--avatar-attribute value`：用户 LDAP 记录中包含用户头像的属性。
        - `--bind-dn value`：在搜索用户时绑定到 LDAP 服务器的 DN。
        - `--bind-password value`：绑定 DN 的密码（如果有）。
        - `--attributes-in-bind`：在绑定 DN 上下文中获取属性。
        - `--synchronize-users`：启用用户同步。
        - `--page-size value`：搜索页面大小。
      - 示例：
        - `gitea admin auth update-ldap --id 1 --name "my ldap auth source"`
        - `gitea admin auth update-ldap --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`
    - `add-ldap-simple`：添加新的 LDAP（简单身份验证）认证源
      - 选项：
        - `--name value`：认证名称。必填。
        - `--not-active`：停用认证源。
        - `--security-protocol value`：安全协议名称。必填。
        - `--skip-tls-verify`：禁用 TLS 验证。
        - `--host value`：LDAP 服务器的地址。必填。
        - `--port value`：连接到 LDAP 服务器时使用的端口。必填。
        - `--user-search-base value`：用户帐户将在其中搜索的 LDAP 基础路径。
        - `--user-filter value`：声明如何查找试图进行身份验证的用户记录的 LDAP 过滤器。必填。
        - `--admin-filter value`：指定是否应授予用户管理员特权的 LDAP 过滤器。
        - `--restricted-filter value`：指定是否应将用户设置为受限状态的 LDAP 过滤器。
        - `--username-attribute value`：用户 LDAP 记录中包含用户名的属性。
        - `--firstname-attribute value`：用户 LDAP 记录中包含用户名字的属性。
        - `--surname-attribute value`：用户 LDAP 记录中包含用户姓氏的属性。
        - `--email-attribute value`：用户 LDAP 记录中包含用户电子邮件地址的属性。必填。
        - `--public-ssh-key-attribute value`：用户 LDAP 记录中包含用户公共 SSH 密钥的属性。
        - `--avatar-attribute value`：用户 LDAP 记录中包含用户头像的属性。
        - `--user-dn value`：用户的 DN。必填。
      - 示例：
        - `gitea admin auth add-ldap-simple --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-dn "cn=%s,ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(cn=%s))" --email-attribute mail`
    - `update-ldap-simple`：更新现有的 LDAP（简单身份验证）认证源
      - 选项：
        - `--id value`：认证源的 ID。必填。
        - `--name value`：认证名称。
        - `--not-active`：停用认证源。
        - `--security-protocol value`：安全协议名称。
        - `--skip-tls-verify`：禁用 TLS 验证。
        - `--host value`：LDAP 服务器的地址。
        - `--port value`：连接到 LDAP 服务器时使用的端口。
        - `--user-search-base value`：用户帐户将在其中搜索的 LDAP 基础路径。
        - `--user-filter value`：声明如何查找试图进行身份验证的用户记录的 LDAP 过滤器。
        - `--admin-filter value`：指定是否应授予用户管理员特权的 LDAP 过滤器。
        - `--restricted-filter value`：指定是否应将用户设置为受限状态的 LDAP 过滤器。
        - `--username-attribute value`：用户 LDAP 记录中包含用户名的属性。
        - `--firstname-attribute value`：用户 LDAP 记录中包含用户名字的属性。
        - `--surname-attribute value`：用户 LDAP 记录中包含用户姓氏的属性。
        - `--email-attribute value`：用户 LDAP 记录中包含用户电子邮件地址的属性。
        - `--public-ssh-key-attribute value`：用户 LDAP 记录中包含用户公共 SSH 密钥的属性。
        - `--avatar-attribute value`：用户 LDAP 记录中包含用户头像的属性。
        - `--user-dn value`：用户的 DN。
      - 示例：
        - `gitea admin auth update-ldap-simple --id 1 --name "my ldap auth source"`
        - `gitea admin auth update-ldap-simple --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`

### cert

生成自签名的SSL证书。将输出到当前目录下的`cert.pem`和`key.pem`文件中，并且会覆盖任何现有文件。

- 选项：
  - `--host value`：逗号分隔的主机名和IP地址列表，此证书适用于这些主机。支持使用通配符。必填。
  - `--ecdsa-curve value`：用于生成密钥的ECDSA曲线。可选。有效选项为P224、P256、P384、P521。
  - `--rsa-bits value`：要生成的RSA密钥的大小。可选。如果设置了--ecdsa-curve，则忽略此选项。（默认值：3072）。
  - `--start-date value`：证书的创建日期。可选。（格式：`Jan 1 15:04:05 2011`）。
  - `--duration value`：证书有效期。可选。（默认值：8760h0m0s）
  - `--ca`：如果提供此选项，则证书将生成自己的证书颁发机构。可选。
- 示例：
  - `gitea cert --host git.example.com,example.com,www.example.com --ca`

### dump

将所有文件和数据库导出到一个zip文件中。输出文件将保存在当前目录下，类似于`gitea-dump-1482906742.zip`。

- 选项：
  - `--file name`，`-f name`：指定要创建的导出文件的名称。可选。（默认值：gitea-dump-[timestamp].zip）。
  - `--tempdir path`，`-t path`：指定临时目录的路径。可选。（默认值：/tmp）。
  - `--skip-repository`，`-R`：跳过仓库的导出。可选。
  - `--skip-custom-dir`：跳过自定义目录的导出。可选。
  - `--skip-lfs-data`：跳过LFS数据的导出。可选。
  - `--skip-attachment-data`：跳过附件数据的导出。可选。
  - `--skip-package-data`：跳过包数据的导出。可选。
  - `--skip-log`：跳过日志数据的导出。可选。
  - `--database`，`-d`：指定数据库的SQL语法。可选。
  - `--verbose`，`-V`：如果提供此选项，显示附加详细信息。可选。
  - `--type`：设置导出的格式。可选。（默认值：zip）
- 示例：
  - `gitea dump`
  - `gitea dump --verbose`

### generate

用于在配置文件中生成随机值和令牌。对于自动部署时生成值非常有用。

- 命令:
  - `secret`:
    - 选项:
      - `INTERNAL_TOKEN`: 用于内部 API 调用身份验证的令牌。
      - `JWT_SECRET`: 用于 LFS 和 OAUTH2 JWT 身份验证的密钥（LFS_JWT_SECRET 是此选项的别名，用于向后兼容）。
      - `SECRET_KEY`: 全局密钥。
    - 示例:
      - `gitea generate secret INTERNAL_TOKEN`
      - `gitea generate secret JWT_SECRET`
      - `gitea generate secret SECRET_KEY`

### keys

提供一个 SSHD AuthorizedKeysCommand。需要在 sshd 配置文件中进行配置:

```ini
...
# -e 的值和 AuthorizedKeysCommandUser 应与运行 Gitea 的用户名匹配
AuthorizedKeysCommandUser git
AuthorizedKeysCommand /path/to/gitea keys -e git -u %u -t %t -k %k
```

该命令将返回适用于提供的密钥的合适 authorized_keys 行。您还应在 `app.ini` 的 `[server]` 部分设置值 `SSH_CREATE_AUTHORIZED_KEYS_FILE=false`。

注意: opensshd 要求 Gitea 程序由 root 拥有，并且不可由组或其他人写入。程序必须使用绝对路径指定。
注意: Gitea 必须在运行此命令时处于运行状态才能成功。

### migrate

迁移数据库。该命令可用于在首次启动服务器之前运行其他命令。此命令是幂等的。

### doctor check

对 Gitea 实例进行诊断，可以修复一些可修复的问题。
默认只运行部分检查，额外的检查可以参考：

- `gitea doctor check --list` - 列出所有可用的检查
- `gitea doctor check --all` - 运行所有可用的检查
- `gitea doctor check --default` - 运行默认的检查
- `gitea doctor check --run [check(s),]...` - 运行指定的名字的检查

有些问题可以通过设置 `--fix` 选项进行自动修复。
额外的日志可以通过 `--log-file=...` 进行设置。

#### doctor recreate-table

有时，在迁移时，旧的列和默认值可能会在数据库模式中保持不变。这可能会导致警告，如下所示:

```
2020/08/02 11:32:29 ...rm/session_schema.go:360:Sync() [W] Table user Column keep_activity_private db default is , struct default is 0
```

您可以通过以下方式让 Gitea 重新创建这些表，并将旧数据复制到新表中，并适当设置默认值：

```
gitea doctor recreate-table user
```

您可以使用以下方式让 Gitea 重新创建多个表：

```
gitea doctor recreate-table table1 table2 ...
```

如果您希望 Gitea 重新创建所有表，请直接调用：

```
gitea doctor recreate-table
```

强烈建议在运行这些命令之前备份您的数据库。

### doctor convert

将现有的 MySQL 数据库从 utf8 转换为 utf8mb4，或者把 MSSQL 数据库从 varchar 转换为 nvarchar。

### manager

管理运行中的服务器操作：

- 命令:
  - `shutdown`: 优雅地关闭运行中的进程
  - `restart`: 优雅地重新启动运行中的进程（对于Windows服务器尚未实现）
  - `flush-queues`: 刷新运行中的进程中的队列
    - 选项:
      - `--timeout value`: 刷新过程的超时时间（默认值: 1m0s）
      - `--non-blocking`: 设置为true，以在返回之前不等待刷新完成
  - `logging`: 调整日志命令
    - 命令:
      - `pause`: 暂停日志记录
        - 注意:
          - 如果日志级别低于此级别，日志级别将被临时提升为INFO。
          - Gitea将在一定程度上缓冲日志，并在超过该点后丢弃日志。
      - `resume`: 恢复日志记录
      - `release-and-reopen`: 使Gitea释放和重新打开用于日志记录的文件和连接（相当于向Gitea发送SIGUSR1信号）。
      - `remove name`: 删除指定的日志记录器
        - 选项:
          - `--group group`, `-g group`: 从中删除子记录器的组（默认为`default`）
      - `add`: 添加日志记录器
        - 命令:
          - `console`: 添加控制台日志记录器
            - 选项:
              - `--group value`, `-g value`: 要添加日志记录器的组 - 默认为"default"
              - `--name value`, `-n value`: 新日志记录器的名称 - 默认为模式
              - `--level value`, `-l value`: 新日志记录器的日志级别
              - `--stacktrace-level value`, `-L value`: 堆栈跟踪日志级别
              - `--flags value`, `-F value`: 日志记录器的标志
              - `--expression value`, `-e value`: 日志记录器的匹配表达式
              - `--prefix value`, `-p value`: 日志记录器的前缀
              - `--color`: 在日志中使用颜色
              - `--stderr`: 将控制台日志输出到stderr - 仅适用于控制台
          - `file`: 添加文件日志记录器
            - 选项:
              - `--group value`, `-g value`: 要添加日志记录器的组 - 默认为"default"
              - `--name value`, `-n value`: 新日志记录器的名称 - 默认为模式
              - `--level value`, `-l value`: 新日志记录器的日志级别
              - `--stacktrace-level value`, `-L value`: 堆栈跟踪日志级别
              - `--flags value`, `-F value`: 日志记录器的标志
              - `--expression value`, `-e value`: 日志记录器的匹配表达式
              - `--prefix value`, `-p value`: 日志记录器的前缀
              - `--color`: 在日志中使用颜色
              - `--filename value`, `-f value`: 日志记录器的文件名
              - `--rotate`, `-r`: 轮转日志
              - `--max-size value`, `-s value`: 在轮转之前的最大大小（以字节为单位）
              - `--daily`, `-d`: 每天轮转日志
              - `--max-days value`, `-D value`: 保留的每日日志的最大数量
              - `--compress`, `-z`: 压缩轮转的日志
              - `--compression-level value`, `-Z value`: 使用的压缩级别
          - `conn`: 添加网络连接日志记录器
            - 选项:
              - `--group value`, `-g value`: 要添加日志记录器的组 - 默认为"default"
              - `--name value`, `-n value`: 新日志记录器的名称 - 默认为模式
              - `--level value`, `-l value`: 新日志记录器的日志级别
              - `--stacktrace-level value`, `-L value`: 堆栈跟踪日志级别
              - `--flags value`, `-F value`: 日志记录器的标志
              - `--expression value`, `-e value`: 日志记录器的匹配表达式
              - `--prefix value`, `-p value`: 日志记录器的前缀
              - `--color`: 在日志中使用颜色
              - `--reconnect-on-message`, `-R`: 对于每个消息重新连接主机
              - `--reconnect`, `-r`: 连接中断时重新连接主机
              - `--protocol value`, `-P value`: 设置要使用的协议：tcp、unix或udp（默认为tcp）
              - `--address value`, `-a value`: 要连接到的主机地址和端口（默认为:7020）
          - `smtp`: 添加SMTP日志记录器
            - 选项:
              - `--group value`, `-g value`: 要添加日志记录器的组 - 默认为"default"
              - `--name value`, `-n value`: 新日志记录器的名称 - 默认为模式
              - `--level value`, `-l value`: 新日志记录器的日志级别
              - `--stacktrace-level value`, `-L value`: 堆栈跟踪日志级别
              - `--flags value`, `-F value`: 日志记录器的标志
              - `--expression value`, `-e value`: 日志记录器的匹配表达式
              - `--prefix value`, `-p value`: 日志记录器的前缀
              - `--color`: 在日志中使用颜色
              - `--username value`, `-u value`: 邮件服务器用户名
              - `--password value`, `-P value`: 邮件服务器密码
              - `--host value`, `-H value`: 邮件服务器主机（默认为: 127.0.0.1:25）
              - `--send-to value`, `-s value`: 要发送到的电子邮件地址
              - `--subject value`, `-S value`: 发送电子邮件的主题标题
  - `processes`: 显示 Gitea 进程和 Goroutine 信息
    - 选项:
      - `--flat`: 以平面表格形式显示进程，而不是树形结构
      - `--no-system`: 不显示系统进程
      - `--stacktraces`: 显示与进程关联的 Goroutine 的堆栈跟踪
      - `--json`: 输出为 JSON 格式
      - `--cancel PID`: 向具有 PID 的进程发送取消命令（仅适用于非系统进程）

### dump-repo

`dump-repo` 从 Git/GitHub/Gitea/GitLab 中转储存储库数据：

- 选项：
  - `--git_service service`：Git 服务，可以是 `git`、`github`、`gitea`、`gitlab`。如果 `clone_addr` 可以被识别，则可以忽略此选项。
  - `--repo_dir dir`，`-r dir`：存储数据的存储库目录路径。
  - `--clone_addr addr`：将被克隆的 URL，目前可以是 git/github/gitea/gitlab 的 http/https URL。例如：https://github.com/lunny/tango.git
  - `--auth_username lunny`：访问 `clone_addr` 的用户名。
  - `--auth_password <password>`：访问 `clone_addr` 的密码。
  - `--auth_token <token>`：访问 `clone_addr` 的个人令牌。
  - `--owner_name lunny`：如果非空，数据将存储在具有所有者名称的目录中。
  - `--repo_name tango`：如果非空，数据将存储在具有存储库名称的目录中。
  - `--units <units>`：要迁移的项目，一个或多个项目应以逗号分隔。允许的项目有 wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments。如果为空，则表示所有项目。

### restore-repo

`restore-repo` 从磁盘目录中还原存储库数据：

- 选项：
  - `--repo_dir dir`，`-r dir`：还原数据的存储库目录路径。
  - `--owner_name lunny`：还原目标所有者名称。
  - `--repo_name tango`：还原目标存储库名称。
  - `--units <units>`：要还原的项目，一个或多个项目应以逗号分隔。允许的项目有 wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments。如果为空，则表示所有项目。

### actions generate-runner-token

生成一个供 Runner 使用的新令牌，用于向服务器注册。

- 选项：
  - `--scope {owner}[/{repo}]`，`-s {owner}[/{repo}]`：限制 Runner 的范围，没有范围表示该 Runner 可用于所有仓库，但你也可以将其限制为特定的仓库或所有者。

要注册全局 Runner：

```
gitea actions generate-runner-token
```

要注册特定组织的 Runner，例如 `org`：

```
gitea actions generate-runner-token -s org
```

要注册特定仓库的 Runner，例如 `username/test-repo`：

```
gitea actions generate-runner-token -s username/test-repo
```
