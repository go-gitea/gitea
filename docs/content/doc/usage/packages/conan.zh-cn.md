---
date: "2021-07-20T00:00:00+00:00"
title: "Conan 软件包注册表"
slug: "conan"
weight: 20
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Conan"
    weight: 20
    identifier: "conan"
---

# Conan 软件包注册表

为您的用户或组织发布 [Conan](https://conan.io/) 软件包。

**目录**

{{< toc >}}

## 要求

要使用 [conan](https://conan.io/downloads.html) 软件包注册表，您需要使用 conan 命令行工具来消费和发布软件包。

## 配置软件包注册表

要注册软件包注册表，您需要配置一个新的 Conan remote：

```shell
conan remote add {remote} https://gitea.example.com/api/packages/{owner}/conan
conan user --remote {remote} --password {password} {username}
```

| 参数       | 描述                                                                                                                                        |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `remote`   | 远程名称。                                                                                                                                  |
| `username` | 您的 Gitea 用户名。                                                                                                                         |
| `password` | 您的 Gitea 密码。如果您使用 2FA 或 OAuth，请使用[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})替代密码。 |
| `owner`    | 软件包的所有者。                                                                                                                            |

例如:

```shell
conan remote add gitea https://gitea.example.com/api/packages/testuser/conan
conan user --remote gitea --password password123 testuser
```

## 发布软件包

通过运行以下命令发布 Conan 软件包：

```shell
conan upload --remote={remote} {recipe}
```

| 参数     | 描述            |
| -------- | --------------- |
| `remote` | 远程名称        |
| `recipe` | 要上传的 recipe |

For example:

```shell
conan upload --remote=gitea ConanPackage/1.2@gitea/final
```

Gitea Conan 软件包注册表支持完整的[版本修订](https://docs.conan.io/en/latest/versioning/revisions.html)。

## 安装软件包

要从软件包注册表中安装Conan软件包，请执行以下命令：

```shell
conan install --remote={remote} {recipe}
```

| 参数     | 描述            |
| -------- | --------------- |
| `remote` | 远程名称        |
| `recipe` | 要下载的 recipe |

例如：

```shell
conan install --remote=gitea ConanPackage/1.2@gitea/final
```

## 支持的命令

```
conan install
conan get
conan info
conan search
conan upload
conan user
conan download
conan remove
```
