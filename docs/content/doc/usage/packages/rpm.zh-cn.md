---
date: "2023-03-08T00:00:00+00:00"
title: "RPM 软件包注册表"
slug: "packages/rpm"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "RPM"
    weight: 105
    identifier: "rpm"
---

# RPM 软件包注册表

为您的用户或组织发布 [RPM](https://rpm.org/) 软件包。

**目录**

{{< toc >}}

## 要求

要使用RPM注册表，您需要使用像 `yum` 或 `dnf` 这样的软件包管理器来消费软件包。

以下示例使用 `dnf`。

## 配置软件包注册表

要注册RPM注册表，请将 URL 添加到已知 `apt` 源列表中：

```shell
dnf config-manager --add-repo https://gitea.example.com/api/packages/{owner}/rpm.repo
```

| 占位符  | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

如果注册表是私有的，请在URL中提供凭据。您可以使用密码或[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})：

```shell
dnf config-manager --add-repo https://{username}:{your_password_or_token}@gitea.example.com/api/packages/{owner}/rpm.repo
```

您还必须将凭据添加到 `/etc/yum.repos.d` 中的 `rpm.repo` 文件中的URL中。

## 发布软件包

要发布RPM软件包（`*.rpm`），请执行带有软件包内容的 HTTP `PUT` 操作。

```
PUT https://gitea.example.com/api/packages/{owner}/rpm/upload
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

使用HTTP基本身份验证的示例请求：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.rpm \
     https://gitea.example.com/api/packages/testuser/rpm/upload
```

如果您使用 2FA 或 OAuth，请使用[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})替代密码。您无法将具有相同名称的文件两次发布到软件包中。您必须先删除现有的软件包版本。

服务器将以以下HTTP状态码响应。

| HTTP 状态码       | 含义                                             |
| ----------------- | ------------------------------------------------ |
| `201 Created`     | 软件包已发布                                     |
| `400 Bad Request` | 软件包无效                                       |
| `409 Conflict`    | 具有相同参数组合的软件包文件已经存在于该软件包中 |

## 删除软件包

要删除 RPM 软件包，请执行 HTTP `DELETE` 操作。如果没有文件剩余，这也将删除软件包版本。

```
DELETE https://gitea.example.com/api/packages/{owner}/rpm/{package_name}/{package_version}/{architecture}
```

| 参数              | 描述           |
| ----------------- | -------------- |
| `owner`           | 软件包的所有者 |
| `package_name`    | 软件包名称     |
| `package_version` | 软件包版本     |
| `architecture`    | 软件包架构     |

使用HTTP基本身份验证的示例请求：

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/rpm/test-package/1.0.0/x86_64
```

服务器将以以下HTTP状态码响应：

| HTTP 状态码      | 含义               |
| ---------------- | ------------------ |
| `204 No Content` | 成功               |
| `404 Not Found`  | 未找到软件包或文件 |

## 安装软件包

要从RPM注册表安装软件包，请执行以下命令：

```shell
# use latest version
dnf install {package_name}
# use specific version
dnf install {package_name}-{package_version}.{architecture}
```
