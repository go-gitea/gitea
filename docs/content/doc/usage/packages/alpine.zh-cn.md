---
date: "2023-03-25T00:00:00+00:00"
title: "Alpine 软件包注册表"
slug: "alpine"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Alpine"
    weight: 4
    identifier: "alpine"
---

# Alpine 软件包注册表

在您的用户或组织中发布 [Alpine](https://pkgs.alpinelinux.org/) 软件包。

**目录**

{{< toc >}}

## 要求

要使用 Alpine 注册表，您需要使用像 curl 这样的 HTTP 客户端来上传包，并使用像 apk 这样的包管理器来消费包。

以下示例使用 `apk`。

## 配置软件包注册表

要注册 Alpine 注册表，请将 URL 添加到已知的 apk 源列表中 (`/etc/apk/repositories`):

```
https://gitea.example.com/api/packages/{owner}/alpine/<branch>/<repository>
```

| 占位符       | 描述           |
| ------------ | -------------- |
| `owner`      | 软件包所有者   |
| `branch`     | 要使用的分支名 |
| `repository` | 要使用的仓库名 |

如果注册表是私有的，请在 URL 中提供凭据。您可以使用密码或[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}}):

```
https://{username}:{your_password_or_token}@gitea.example.com/api/packages/{owner}/alpine/<branch>/<repository>
```

Alpine 注册表文件使用 RSA 密钥进行签名，apk 必须知道该密钥。下载公钥并将其存储在 `/etc/apk/keys/` 目录中：

```shell
curl -JO https://gitea.example.com/api/packages/{owner}/alpine/key
```

之后，更新本地软件包索引：

```shell
apk update
```

## 发布软件包

要发布一个 Alpine 包（`*.apk`），请执行带有包内容的 HTTP `PUT` 操作，将其放在请求体中。

```
PUT https://gitea.example.com/api/packages/{owner}/alpine/{branch}/{repository}
```

| 参数         | 描述                                                                                                |
| ------------ | --------------------------------------------------------------------------------------------------- |
| `owner`      | 包的所有者。                                                                                        |
| `branch`     | 分支可以与操作系统的发行版本匹配，例如：v3.17。                                                     |
| `repository` | 仓库可以用于[分组包](https://wiki.alpinelinux.org/wiki/Repositories) 或者只是 `main` 或类似的名称。 |

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.apk \
     https://gitea.example.com/api/packages/testuser/alpine/v3.17/main
```

如果您使用的是双重身份验证或 OAuth，请使用[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#authentication" >}})代替密码。
您不能将具有相同名称的文件两次发布到一个包中。您必须首先删除现有的包文件。

服务器将以以下的 HTTP 状态码响应：

| HTTP 状态码       | 含义                                       |
| ----------------- | ------------------------------------------ |
| `201 Created`     | 软件包已发布。                             |
| `400 Bad Request` | 软件包的名称、版本、分支、仓库或架构无效。 |
| `409 Conflict`    | 具有相同参数组合的包文件已存在于软件包中。 |

## 删除软件包

要删除 Alpine 包，执行 HTTP 的 DELETE 操作。如果没有文件，这将同时删除包版本。

```
DELETE https://gitea.example.com/api/packages/{owner}/alpine/{branch}/{repository}/{architecture}/{filename}
```

| 参数           | 描述           |
| -------------- | -------------- |
| `owner`        | 软件包的所有者 |
| `branch`       | 要使用的分支名 |
| `repository`   | 要使用的仓库名 |
| `architecture` | 软件包的架构   |
| `filename`     | 要删除的文件名 |

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/alpine/v3.17/main/test-package-1.0.0.apk
```

服务器将以以下的 HTTP 状态码响应：

| HTTP 状态码      | 含义               |
| ---------------- | ------------------ |
| `204 No Content` | 成功               |
| `404 Not Found`  | 未找到软件包或文件 |

## 安装软件包

要从 Alpine 注册表安装软件包，请执行以下命令：

```shell
# use latest version
apk add {package_name}
# use specific version
apk add {package_name}={package_version}
```
