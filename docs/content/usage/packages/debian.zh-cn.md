---
date: "2023-01-07T00:00:00+00:00"
title: "Debian 软件包注册表"
slug: "debian"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Debian"
    sidebar_position: 35
    identifier: "debian"
---

# Debian 软件包注册表

为您的用户或组织发布 [Debian](https://www.debian.org/distrib/packages) 软件包。

## 要求

要使用 Debian 注册表，您需要使用类似于 `curl` 的 HTTP 客户端进行上传，并使用类似于 `apt` 的软件包管理器消费软件包。

以下示例使用 `apt`。

## 配置软件包注册表

要注册 Debian 注册表，请将 URL 添加到已知 `apt` 源列表中：

```shell
echo "deb https://gitea.example.com/api/packages/{owner}/debian {distribution} {component}" | sudo tee -a /etc/apt/sources.list.d/gitea.list
```

| 占位符         | 描述           |
| -------------- | -------------- |
| `owner`        | 软件包的所有者 |
| `distribution` | 要使用的发行版 |
| `component`    | 要使用的组件   |

如果注册表是私有的，请在 URL 中提供凭据。您可以使用密码或[个人访问令牌](development/api-usage.md#通过-api-认证)：

```shell
echo "deb https://{username}:{your_password_or_token}@gitea.example.com/api/packages/{owner}/debian {distribution} {component}" | sudo tee -a /etc/apt/sources.list.d/gitea.list
```

Debian 注册表文件使用 PGP 密钥进行签名，`apt` 必须知道该密钥：

```shell
sudo curl https://gitea.example.com/api/packages/{owner}/debian/repository.key -o /etc/apt/trusted.gpg.d/gitea-{owner}.asc
```

然后更新本地软件包索引：

```shell
apt update
```

## 发布软件包

要发布一个 Debian 软件包（`*.deb`），执行 HTTP `PUT` 操作，并将软件包内容放入请求主体中。

```
PUT https://gitea.example.com/api/packages/{owner}/debian/pool/{distribution}/{component}/upload
```

| 参数           | 描述                                                  |
| -------------- | ----------------------------------------------------- |
| `owner`        | 软件包的所有者                                        |
| `distribution` | 发行版，可能与操作系统的发行版名称匹配，例如 `bionic` |
| `component`    | 组件，可用于分组软件包，或仅为 `main` 或类似的组件。  |

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.deb \
     https://gitea.example.com/api/packages/testuser/debian/pool/bionic/main/upload
```

如果您使用 2FA 或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)替代密码。
您无法向软件包中多次发布具有相同名称的文件。您必须首先删除现有的软件包版本。

服务器将使用以下 HTTP 状态代码进行响应。

| HTTP 状态码       | 意义                                     |
| ----------------- | ---------------------------------------- |
| `201 Created`     | 软件包已发布                             |
| `400 Bad Request` | 软件包名称、版本、发行版、组件或架构无效 |
| `409 Conflict`    | 具有相同参数组合的软件包文件已经存在     |

## 删除软件包

要删除 Debian 软件包，请执行 HTTP `DELETE` 操作。如果没有文件留下，这将同时删除软件包版本。

```
DELETE https://gitea.example.com/api/packages/{owner}/debian/pool/{distribution}/{component}/{package_name}/{package_version}/{architecture}
```

| 参数              | 描述           |
| ----------------- | -------------- |
| `owner`           | 软件包的所有者 |
| `package_name`    | 软件包名称     |
| `package_version` | 软件包版本     |
| `distribution`    | 软件包发行版   |
| `component`       | 软件包组件     |
| `architecture`    | 软件包架构     |

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/debian/pools/bionic/main/test-package/1.0.0/amd64
```

服务器将使用以下 HTTP 状态代码进行响应。

| HTTP 状态码      | 含义               |
| ---------------- | ------------------ |
| `204 No Content` | 成功               |
| `404 Not Found`  | 找不到软件包或文件 |

## 安装软件包

要从 Debian 注册表安装软件包，请执行以下命令:

```shell
# use latest version
apt install {package_name}
# use specific version
apt install {package_name}={package_version}
```
