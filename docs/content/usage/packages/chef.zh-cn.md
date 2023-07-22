---
date: "2023-01-20T00:00:00+00:00"
title: "Chef 软件包注册表"
slug: "chef"
sidebar_position: 5
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Chef"
    sidebar_position: 5
    identifier: "chef"
---

# Chef Package Registry

为您的用户或组织发布 [Chef](https://chef.io/) cookbooks。

## 要求

要使用 Chef 软件包注册表，您需要使用 [`knife`](https://docs.chef.io/workstation/knife/).

## 认证

Chef 软件包注册表不使用用户名和密码进行身份验证，而是使用私钥和公钥对请求进行签名。
请访问软件包所有者设置页面以创建必要的密钥对。
只有公钥存储在Gitea中。如果您丢失了私钥的访问权限，您必须重新生成密钥对。
[配置 `knife`](https://docs.chef.io/workstation/knife_setup/)，使用下载的私钥，并将 Gitea 用户名设置为 `client_name`。

## 配置软件包注册表

要将 [`knife` 配置](https://docs.chef.io/workstation/knife_setup/)为使用 Gitea 软件包注册表，请将 URL 添加到 `~/.chef/config.rb` 文件中。

```
knife[:supermarket_site] = 'https://gitea.example.com/api/packages/{owner}/chef'
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

## 发布软件包

若要发布 Chef 软件包，请执行以下命令：

```shell
knife supermarket share {package_name}
```

| 参数           | 描述       |
| -------------- | ---------- |
| `package_name` | 软件包名称 |

如果已经存在同名和版本的软件包，则无法发布新的软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表中安装软件包，请执行以下命令：

```shell
knife supermarket install {package_name}
```

您可以指定软件包的版本，这是可选的：

```shell
knife supermarket install {package_name} {package_version}
```

| 参数              | 描述       |
| ----------------- | ---------- |
| `package_name`    | 软件包名称 |
| `package_version` | 软件包版本 |

## 删除软件包

如果您想要从注册表中删除软件包，请执行以下命令：

```shell
knife supermarket unshare {package_name}
```

可选地，您可以指定软件包的版本：

```shell
knife supermarket unshare {package_name}/versions/{package_version}
```

| 参数              | 描述       |
| ----------------- | ---------- |
| `package_name`    | 软件包名称 |
| `package_version` | 软件包版本 |
