---
date: "2023-01-10T00:00:00+00:00"
title: "Swift 软件包注册表"
slug: "swift"
sidebar_position: 95
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Swift"
    sidebar_position: 95
    identifier: "swift"
---

# Swift 软件包注册表

为您的用户或组织发布 [Swift](https://www.swift.org/) 软件包。

## 要求

要使用 Swift 软件包注册表，您需要使用 [swift](https://www.swift.org/getting-started/) 消费软件包，并使用 HTTP 客户端（如 `curl`）发布软件包。

## 配置软件包注册表

要注册软件包注册表并提供凭据，请执行以下命令：

```shell
swift package-registry set https://gitea.example.com/api/packages/{owner}/swift -login {username} -password {password}
```

| 占位符     | 描述                                                                                                                                           |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `owner`    | 软件包的所有者。                                                                                                                               |
| `username` | 您的 Gitea 用户名。                                                                                                                            |
| `password` | 您的 Gitea 密码。如果您使用两步验证或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)代替密码。 |

登录是可选的，只有在软件包注册表是私有的情况下才需要。

## 发布软件包

首先，您需要打包软件包的内容：

```shell
swift package archive-source
```

要发布软件包，请执行一个带有软件包内容的 HTTP `PUT` 请求，将内容放在请求正文中。

```shell --user your_username:your_password_or_token \
curl -X PUT --user {username}:{password} \
	 -H "Accept: application/vnd.swift.registry.v1+json" \
	 -F source-archive=@/path/to/package.zip \
	 -F metadata={metadata} \
	 https://gitea.example.com/api/packages/{owner}/swift/{scope}/{name}/{version}
```

| 占位符     | 描述                                                                                                                                           |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `username` | 您的 Gitea 用户名。                                                                                                                            |
| `password` | 您的 Gitea 密码。如果您使用两步验证或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)代替密码。 |
| `owner`    | 软件包的所有者。                                                                                                                               |
| `scope`    | 软件包的作用域。                                                                                                                               |
| `name`     | 软件包的名称。                                                                                                                                 |
| `version`  | 软件包的版本。                                                                                                                                 |
| `metadata` | （可选）软件包的元数据。以 JSON 编码的子集，格式参考 https://schema.org/SoftwareSourceCode                                                     |

如果已经存在相同名称和版本的软件包，则无法发布软件包。您必须首先删除现有的软件包。

## 安装软件包

要从软件包注册表安装 Swift 软件包，请将其添加到 `Package.swift` 文件的依赖项列表中：

```
dependencies: [
	.package(id: "{scope}.{name}", from:"{version}")
]
```

| 参数      | 描述           |
| --------- | -------------- |
| `scope`   | 软件包的作用域 |
| `name`    | 软件包的名称   |
| `version` | 软件包的版本   |

之后，执行以下命令来安装它：

```shell
swift package resolve
```
