---
date: "2021-07-20T00:00:00+00:00"
title: "通用软件包注册表"
slug: "generic"
sidebar_position: 40
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "通用"
    sidebar_position: 40
    identifier: "generic"
---

# 通用软件包注册表

发布通用文件，如发布二进制文件或其他输出，供您的用户或组织使用。

## 身份验证软件包注册表

要身份验证软件包注册表，您需要提供[自定义 HTTP 头或使用 HTTP 基本身份验证](development/api-usage.md#通过-api-认证)。

## 发布软件包

要发布通用软件包，请执行 HTTP `PUT` 操作，并将软件包内容放入请求主体中。
您无法向软件包中多次发布具有相同名称的文件。您必须首先删除现有的软件包版本。

```
PUT https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}/{file_name}
```

| 参数              | 描述                                                                                                                        |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `owner`           | 软件包的所有者。                                                                                                            |
| `package_name`    | 软件包名称。它只能包含小写字母 (`a-z`)、大写字母 (`A-Z`)、数字 (`0-9`)、点号 (`.`)、连字符 (`-`)、加号 (`+`) 或下划线 (`_`) |
| `package_version` | 软件包版本，一个非空字符串，不包含前导或尾随空格                                                                            |
| `file_name`       | 文件名。它只能包含小写字母 (`a-z`)、大写字母 (`A-Z`)、数字 (`0-9`)、点号 (`.`)、连字符 (`-`)、加号 (`+`) 或下划线 (`_`)     |

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.bin \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0/file.bin
```

如果您使用 2FA 或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)替代密码。

服务器将使用以下 HTTP 状态代码进行响应。

| HTTP 状态码       | 意义                               |
| ----------------- | ---------------------------------- |
| `201 Created`     | 软件包已发布                       |
| `400 Bad Request` | 软件包名称和/或版本和/或文件名无效 |
| `409 Conflict`    | 具有相同名称的文件已存在于软件包中 |

## 下载软件包

要下载通用软件包，请执行 HTTP `GET` 操作。

```
GET https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}/{file_name}
```

| 参数              | 描述           |
| ----------------- | -------------- |
| `owner`           | 软件包的所有者 |
| `package_name`    | 软件包名称     |
| `package_version` | 软件包版本     |
| `file_name`       | 文件名         |

文件内容将在响应主体中返回。响应的内容类型为 `application/octet-stream`。

服务器将使用以下 HTTP 状态代码进行响应。

```shell
curl --user your_username:your_token_or_password \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0/file.bin
```

服务器会以以下 HTTP 状态码进行响应：

| HTTP 状态码     | 含义                 |
| --------------- | -------------------- |
| `200 OK`        | 成功                 |
| `404 Not Found` | 找不到软件包或者文件 |

## 删除软件包

要删除通用软件包，请执行 HTTP DELETE 操作。这将同时删除该版本的所有文件。

```
DELETE https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}
```

| 参数              | 描述           |
| ----------------- | -------------- |
| `owner`           | 软件包的所有者 |
| `package_name`    | 软件包名称     |
| `package_version` | 软件包版本     |

服务器将使用以下 HTTP 状态代码进行响应。

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0
```

The server responds with the following HTTP Status codes.

| HTTP 状态码      | 意义         |
| ---------------- | ------------ |
| `204 No Content` | 成功         |
| `404 Not Found`  | 找不到软件包 |

## 删除软件包文件

要删除通用软件包的文件，请执行 HTTP `DELETE` 操作。如果没有文件留下，这将同时删除软件包版本。

```
DELETE https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}/{filename}
```

| 参数              | 描述           |
| ----------------- | -------------- |
| `owner`           | 软件包的所有者 |
| `package_name`    | 软件包名称     |
| `package_version` | 软件包版本     |
| `filename`        | 文件名         |

使用 HTTP 基本身份验证的示例请求：

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0/file.bin
```

服务器将使用以下 HTTP 状态代码进行响应：

| HTTP 状态码      | 含义               |
| ---------------- | ------------------ |
| `204 No Content` | 成功               |
| `404 Not Found`  | 找不到软件包或文件 |
