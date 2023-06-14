---
date: "2020-01-16"
title: "数据库准备"
slug: "database-prep"
weight: 10
toc: false
draft: false
aliases:
  - /zh-cn/database-prep
menu:
  sidebar:
    parent: "installation"
    name: "数据库准备"
    weight: 10
    identifier: "database-prep"
---

# 数据库准备

在使用 Gitea 前，您需要准备一个数据库。Gitea 支持 PostgreSQL（>=10）、MySQL（>=5.7）、SQLite 和 MSSQL（>=2008R2 SP3）这几种数据库。本页将指导您准备数据库。由于 PostgreSQL 和 MySQL 在生产环境中被广泛使用，因此本文档将仅涵盖这两种数据库。如果您计划使用 SQLite，则可以忽略本章内容。

数据库实例可以与 Gitea 实例在相同机器上（本地数据库），也可以与 Gitea 实例在不同机器上（远程数据库）。

注意：以下所有步骤要求您的选择的数据库引擎已安装在您的系统上。对于远程数据库设置，请在数据库实例上安装服务器应用程序，在 Gitea 服务器上安装客户端程序。客户端程序用于测试 Gitea 服务器与数据库之间的连接，而 Gitea 本身使用 Go 提供的数据库驱动程序完成相同的任务。此外，请确保服务器和客户端使用相同的引擎版本，以使某些引擎功能正常工作。出于安全原因，请使用安全密码保护 `root`（MySQL）或 `postgres`（PostgreSQL）数据库超级用户。以下步骤假设您在数据库和 Gitea 服务器上都使用 Linux。

**目录**

{{< toc >}}

## MySQL

1. 对于远程数据库设置，您需要让 MySQL 监听您的 IP 地址。编辑数据库实例上的 `/etc/mysql/my.cnf` 文件中的 `bind-address` 选项为：

    ```ini
    bind-address = 203.0.113.3
    ```

2. 在数据库实例上，使用 `root` 用户登录到数据库控制台：

    ```
    mysql -u root -p
    ```

    按提示输入密码。

3. 创建一个将被 Gitea 使用的数据库用户，并使用密码进行身份验证。以下示例中使用了 `'gitea'` 作为密码。请为您的实例使用一个安全密码。

    对于本地数据库：

    ```sql
    SET old_passwords=0;
    CREATE USER 'gitea' IDENTIFIED BY 'gitea';
    ```

    对于远程数据库：

    ```sql
    SET old_passwords=0;
    CREATE USER 'gitea'@'192.0.2.10' IDENTIFIED BY 'gitea';
    ```

    其中 `192.0.2.10` 是您的 Gitea 实例的 IP 地址。

    根据需要替换上述用户名和密码。

4. 使用 UTF-8 字符集和排序规则创建数据库。确保使用 `**utf8mb4**` 字符集，而不是 `utf8`，因为前者支持 _Basic Multilingual Plane_ 之外的所有 Unicode 字符（包括表情符号）。排序规则根据您预期的内容选择。如果不确定，可以使用 `unicode_ci` 或 `general_ci`。

    ```sql
    CREATE DATABASE giteadb CHARACTER SET 'utf8mb4' COLLATE 'utf8mb4_unicode_ci';
    ```

    根据需要替换数据库名称。

5. 将数据库上的所有权限授予上述创建的数据库用户。

    对于本地数据库：

    ```sql
    GRANT ALL PRIVILEGES ON giteadb.* TO 'gitea';
    FLUSH PRIVILEGES;
    ```

    对于远程数据库：

    ```sql
    GRANT ALL PRIVILEGES ON giteadb.* TO 'gitea'@'192.0.2.10';
    FLUSH PRIVILEGES;
    ```

6. 通过 exit 退出数据库控制台。

7. 在您的 Gitea 服务器上，测试与数据库的连接：

    ```
    mysql -u gitea -h 203.0.113.3 -p giteadb
    ```

    其中 `gitea` 是数据库用户名，`giteadb` 是数据库名称，`203.0.113.3` 是数据库实例的 IP 地址。对于本地数据库，省略 -h 选项。

    到此您应该能够连接到数据库了。

## PostgreSQL

1. 对于远程数据库设置，通过编辑数据库实例上的 postgresql.conf 文件中的 listen_addresses 将 PostgreSQL 配置为监听您的 IP 地址：

    ```ini
    listen_addresses = 'localhost, 203.0.113.3'
    ```

2. PostgreSQL 默认使用 `md5` 质询-响应加密方案进行密码身份验证。现在这个方案不再被认为是安全的。改用 SCRAM-SHA-256 方案，通过编辑数据库服务器上的` postgresql.conf` 配置文件：

    ```ini
    password_encryption = scram-sha-256
    ```

    重启 PostgreSQL 以应用该设置。

3. 在数据库服务器上，以超级用户身份登录到数据库控制台：

    ```
    su -c "psql" - postgres
    ```

4. 创建具有登录权限和密码的数据库用户（在 PostgreSQL 术语中称为角色）。请使用安全的、强密码，而不是下面的 `'gitea'`：

    ```sql
    CREATE ROLE gitea WITH LOGIN PASSWORD 'gitea';
    ```

    根据需要替换用户名和密码。

5. 使用 UTF-8 字符集创建数据库，并由之前创建的数据库用户拥有。可以根据预期内容使用任何 `libc` 排序规则，使用 `LC_COLLATE` 和 `LC_CTYPE` 参数指定：

    ```sql
    CREATE DATABASE giteadb WITH OWNER gitea TEMPLATE template0 ENCODING UTF8 LC_COLLATE 'en_US.UTF-8' LC_CTYPE 'en_US.UTF-8';
    ```

    根据需要替换数据库名称。

6. 通过将以下身份验证规则添加到 `pg_hba.conf`，允许数据库用户访问上面创建的数据库。

    对于本地数据库：

    ```ini
    local    giteadb    gitea    scram-sha-256
    ```

    对于远程数据库：

    ```ini
    host    giteadb    gitea    192.0.2.10/32    scram-sha-256
    ```

    根据您自己的数据库名称、用户和 Gitea 实例的 IP 地址进行替换。

    注意：`pg_hba.conf` 上的规则按顺序评估，也就是第一个匹配的规则将用于身份验证。您的 PostgreSQL 安装可能附带了适用于所有用户和数据库的通用身份验证规则。如果是这种情况，您可能需要将此处提供的规则放置在此类通用规则之上。

    重启 PostgreSQL 以应用新的身份验证规则。

7. 在您的 Gitea 服务器上，测试与数据库的连接。

    对于本地数据库：

    ```
    psql -U gitea -d giteadb
    ```

    对于远程数据库：

    ```
    psql "postgres://gitea@203.0.113.3/giteadb"
    ```

    其中 `gitea` 是数据库用户，`giteadb` 是数据库名称，`203.0.113.3` 是您的数据库实例的 IP 地址。

    您应该会被提示输入数据库用户的密码，并连接到数据库。

## 使用 TLS 进行数据库连接

如果 Gitea 和您的数据库实例之间的通信是通过私有网络进行的，或者如果 Gitea 和数据库运行在同一台服务器上，那么可以省略本节，因为 Gitea 和数据库实例之间的安全性不会受到严重威胁。但是，如果数据库实例位于公共网络上，请使用 TLS 对数据库连接进行加密，以防止第三方拦截流量数据。

### 先决条件

- 您需要两个有效的 TLS 证书，一个用于数据库实例（数据库服务器），一个用于 Gitea 实例（数据库客户端）。两个证书都必须由受信任的 CA 签名。
- 数据库证书必须在 `X509v3 Extended Key Usage` 扩展属性中包含 `TLS Web Server Authentication`，而客户端证书则需要在相应的属性中包含 `TLS Web Client Authentication`。
- 在数据库服务器证书中，`Subject Alternative Name` 或 `Common Name` 条目之一必须是数据库实例的完全限定域名（FQDN）（例如 `db.example.com`）。在数据库客户端证书中，上述提到的条目之一必须包含 Gitea 将用于连接的数据库用户名。
- 您需要将 Gitea 和数据库服务器的域名映射到它们各自的 IP 地址。可以为它们设置 DNS 记录，也可以在每个系统上的 `/etc/hosts`（Windows 中的 `%WINDIR%\System32\drivers\etc\hosts`）中添加本地映射。这样可以通过域名而不是 IP 地址进行数据库连接。有关详细信息，请参阅您系统的文档。

### PostgreSQL

Gitea 使用的 PostgreSQL 驱动程序支持双向 TLS。在双向 TLS 中，数据库客户端和服务器通过将各自的证书发送给对方进行验证来相互认证。换句话说，服务器验证客户端证书，客户端验证服务器证书。

1. 在数据库实例所在的服务器上，放置以下凭据：

    - `/path/to/postgresql.crt`: 数据库实例证书
    - `/path/to/postgresql.key`: 数据库实例私钥
    - `/path/to/root.crt`: 用于验证客户端证书的CA证书链

2. 在 `postgresql.conf` 中添加以下选项：

    ```ini
    ssl = on
    ssl_ca_file = '/path/to/root.crt'
    ssl_cert_file = '/path/to/postgresql.crt'
    ssl_key_file = '/path/to/postgresql.key'
    ssl_min_protocol_version = 'TLSv1.2'
    ```

3. 根据 PostgreSQL 的要求，调整凭据的所有权和权限：

    ```
    chown postgres:postgres /path/to/root.crt /path/to/postgresql.crt /path/to/postgresql.key
    chmod 0600 /path/to/root.crt /path/to/postgresql.crt /path/to/postgresql.key
    ```

4. 编辑 `pg_hba.conf` 规则，仅允许 Gitea 数据库用户通过SSL连接，并要求客户端证书验证。

    对于PostgreSQL 12：

    ```ini
    hostssl    giteadb    gitea    192.0.2.10/32    scram-sha-256    clientcert=verify-full
    ```

    对于PostgreSQL 11及更早版本：

    ```ini
    hostssl    giteadb    gitea    192.0.2.10/32    scram-sha-256    clientcert=1
    ```

    根据需要替换数据库名称、用户和 Gitea 实例的 IP 地址。

5. 重新启动 PostgreSQL 以应用上述配置。

6. 在运行 Gitea 实例的服务器上，将以下凭据放置在运行 Gitea 的用户的主目录下（例如 `git`）：

    - `~/.postgresql/postgresql.crt`: 数据库客户端证书
    - `~/.postgresql/postgresql.key`: 数据库客户端私钥
    - `~/.postgresql/root.crt`: 用于验证服务器证书的CA证书链

    注意：上述文件名在 PostgreSQL 中是硬编码的，无法更改。

7. 根据需要调整凭据、所有权和权限：

    ```
    chown git:git ~/.postgresql/postgresql.crt ~/.postgresql/postgresql.key ~/.postgresql/root.crt
    chown 0600 ~/.postgresql/postgresql.crt ~/.postgresql/postgresql.key ~/.postgresql/root.crt
    ```

8. 测试与数据库的连接：

    ```
    psql "postgres://gitea@example.db/giteadb?sslmode=verify-full"
    ```

    您将被提示输入数据库用户的密码，然后连接到数据库。

### MySQL

虽然 Gitea 使用的MySQL驱动程序也支持双向 TLS，但目前 Gitea 仅支持单向 TLS。有关详细信息，请参见工单＃10828。

在单向TLS中，数据库客户端在连接握手期间验证服务器发送的证书，而服务器则假定连接的客户端是合法的，因为不进行客户端证书验证。

1. 在数据库实例上放置以下凭据：

    - `/path/to/mysql.crt`: 数据库实例证书
    - `/path/to/mysql.key`: 数据库实例密钥
    - `/path/to/ca.crt`: CA证书链。在单向TLS中不使用此文件，但用于验证双向TLS中的客户端证书。

2. 将以下选项添加到 `my.cnf`：

    ```ini
    [mysqld]
    ssl-ca = /path/to/ca.crt
    ssl-cert = /path/to/mysql.crt
    ssl-key = /path/to/mysql.key
    tls-version = TLSv1.2,TLSv1.3
    ```

3. 调整凭据的所有权和权限：

    ```
    chown mysql:mysql /path/to/ca.crt /path/to/mysql.crt /path/to/mysql.key
    chmod 0600 /path/to/ca.crt /path/to/mysql.crt /path/to/mysql.key
    ```

4. 重新启动MySQL以应用设置。

5. Gitea 的数据库用户可能已经创建过，但只会对运行 Gitea 的服务器的 IP 地址进行身份验证。要对其域名进行身份验证，请重新创建用户，并设置其需要通过 TLS 连接到数据库：

    ```sql
    DROP USER 'gitea'@'192.0.2.10';
    CREATE USER 'gitea'@'example.gitea' IDENTIFIED BY 'gitea' REQUIRE SSL;
    GRANT ALL PRIVILEGES ON giteadb.* TO 'gitea'@'example.gitea';
    FLUSH PRIVILEGES;
    ```

    根据需要替换数据库用户名、密码和 Gitea 实例域名。

6. 确保用于验证数据库服务器证书的CA证书链位于数据库和 Gitea 服务器的系统证书存储中。请参考系统文档中有关将 CA 证书添加到证书存储的说明。

7. 在运行Gitea的服务器上，测试与数据库的连接：

    ```
    mysql -u gitea -h example.db -p --ssl
    ```

    至此应该成功连接到数据库了。
