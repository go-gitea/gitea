---
date: "2021-07-20T00:00:00+00:00"
title: "Composer 软件包注册表"
slug: "composer"
sidebar_position: 10
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Composer"
    sidebar_position: 10
    identifier: "composer"
---

# Composer 软件包注册表

为您的用户或组织发布 [Composer](https://getcomposer.org/) 软件包。

## 要求

要使用 Composer 软件包注册表，您可以使用 [Composer](https://getcomposer.org/download/) 消费，并使用类似 `curl` 的 HTTP 上传客户端发布软件包。

## 发布软件包

要发布 Composer 软件包，请执行 HTTP `PUT` 操作，将软件包内容放入请求体中。
软件包内容必须是包含 `composer.json` 文件的压缩 PHP 项目。
如果已经存在同名和版本的软件包，则无法发布新的软件包。您必须先删除现有的软件包。

```
PUT https://gitea.example.com/api/packages/{owner}/composer
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

如果 `composer.json` 文件不包含 `version` 属性，您必须将其作为查询参数提供：

```
PUT https://gitea.example.com/api/packages/{owner}/composer?version={x.y.z}
```

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/project.zip \
     https://gitea.example.com/api/packages/testuser/composer
```

或者将软件包版本指定为查询参数：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/project.zip \
     https://gitea.example.com/api/packages/testuser/composer?version=1.0.3
```

如果您使用 2FA 或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)替代密码。

服务器将以以下 HTTP 状态码响应。

| HTTP 状态码       | 含义                                                        |
| ----------------- | ----------------------------------------------------------- |
| `201 Created`     | 软件包已发布                                                |
| `400 Bad Request` | 软件包名称和/或版本无效，或具有相同名称和版本的软件包已存在 |

## 配置软件包注册表

要注册软件包注册表，您需要将其添加到 Composer 的 `config.json` 文件中（通常可以在 `<user-home-dir>/.composer/config.json` 中找到）：

```json
{
  "repositories": [{
      "type": "composer",
      "url": "https://gitea.example.com/api/packages/{owner}/composer"
   }
  ]
}
```

要使用凭据访问软件包注册表，您必须在 `auth.json` 文件中指定它们，如下所示：

```json
{
  "http-basic": {
    "gitea.example.com": {
      "username": "{username}",
      "password": "{password}"
    }
  }
}
```

| 参数       | 描述                        |
| ---------- | --------------------------- |
| `owner`    | 软件包的所有者              |
| `username` | 您的 Gitea 用户名           |
| `password` | 您的Gitea密码或个人访问令牌 |

## 安装软件包

要从软件包注册表中安装软件包，请执行以下命令：

```shell
composer require {package_name}
```

您可以指定软件包的版本，这是可选的：

```shell
composer require {package_name}:{package_version}
```

| 参数              | 描述       |
| ----------------- | ---------- |
| `package_name`    | 软件包名称 |
| `package_version` | 软件包版本 |
