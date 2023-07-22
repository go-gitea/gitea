---
date: "2023-05-10T00:00:00+00:00"
title: "Go 软件包注册表"
slug: "go"
sidebar_position: 45
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Go"
    sidebar_position: 45
    identifier: "go"
---

# Go 软件包注册表

为您的用户或组织发布 Go 软件包。

## 发布软件包

要发布 Go 软件包，请执行 HTTP `PUT` 操作，并将软件包内容放入请求主体中。
如果已经存在相同名称和版本的软件包，您无法发布软件包。您必须首先删除现有的软件包。
该软件包必须遵循[文档中的结构](https://go.dev/ref/mod#zip-files)。

```
PUT https://gitea.example.com/api/packages/{owner}/go/upload
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

要身份验证到软件包注册表，您需要提供[自定义 HTTP 头或使用 HTTP 基本身份验证](development/api-usage.md#通过-api-认证)：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.zip \
     https://gitea.example.com/api/packages/testuser/go/upload
```

如果您使用的是 2FA 或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)替代密码进行身份验证。

服务器将使用以下 HTTP 状态代码进行响应。

| HTTP 状态码       | 含义                       |
| ----------------- | -------------------------- |
| `201 Created`     | 软件包已发布               |
| `400 Bad Request` | 软件包无效                 |
| `409 Conflict`    | 具有相同名称的软件包已存在 |

## 安装软件包

要安装Go软件包，请指示Go使用软件包注册表作为代理：

```shell
# 使用最新版本
GOPROXY=https://gitea.example.com/api/packages/{owner}/go go install {package_name}
# 或者
GOPROXY=https://gitea.example.com/api/packages/{owner}/go go install {package_name}@latest
# 使用特定版本
GOPROXY=https://gitea.example.com/api/packages/{owner}/go go install {package_name}@{package_version}
```

| 参数              | 描述           |
| ----------------- | -------------- |
| `owner`           | 软件包的所有者 |
| `package_name`    | 软件包名称     |
| `package_version` | 软件包版本     |

如果软件包的所有者是私有的，则需要[提供凭据](https://go.dev/ref/mod#private-module-proxy-auth)。

有关 `GOPROXY` 环境变量的更多信息以及如何防止数据泄漏的信息，请[参阅文档](https://go.dev/ref/mod#private-modules)。
