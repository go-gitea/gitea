---
date: "2022-11-20T00:00:00+00:00"
title: "Cargo 软件包注册表"
slug: "cargo"
sidebar_position: 5
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Cargo"
    sidebar_position: 5
    identifier: "cargo"
---

# Cargo 软件包注册表

为您的用户或组织发布 [Cargo](https://doc.rust-lang.org/stable/cargo/) 软件包。

## 要求

若要使用 Cargo 软件包注册表, 您需要安装 [Rust 和 Cargo](https://www.rust-lang.org/tools/install).

Cargo 将可用软件包的信息存储在一个存储在 git 仓库中的软件包索引中。
这个仓库是与注册表交互所必需的。
下面的部分将介绍如何创建它。

## 索引仓库

Cargo 将可用软件包的信息存储在一个存储在 git 仓库中的软件包索引中。
在 Gitea 中，这个仓库有一个特殊的名称叫做 `_cargo-index`。
在上传软件包之后，它的元数据会自动写入索引中。
不应手动修改这个注册表的内容。

用户或组织软件包设置页面允许创建这个索引仓库以及配置文件。
如果需要，此操作将重写配置文件。
例如，如果 Gitea 实例的域名已更改，这将非常有用。

如果存储在 Gitea 中的软件包与索引注册表中的信息不同步，设置页面允许重建这个索引注册表。
这个操作将遍历注册表中的所有软件包，并将它们的信息写入索引中。
如果有很多软件包，这个过程可能需要一些时间。

## 配置软件包注册表

要注册这个软件包注册表，必须更新 Cargo 的配置。
将以下文本添加到位于当前用户主目录中的配置文件中（例如 `~/.cargo/config.toml`）：

```
[registry]
default = "gitea"

[registries.gitea]
index = "sparse+https://gitea.example.com/api/packages/{owner}/cargo/" # Sparse index
# index = "https://gitea.example.com/{owner}/_cargo-index.git" # Git

[net]
git-fetch-with-cli = true
```

| 参数    | 描述             |
| ------- | ---------------- |
| `owner` | 软件包的所有者。 |

如果这个注册表是私有的或者您想要发布新的软件包，您必须配置您的凭据。
将凭据部分添加到位于当前用户主目录中的凭据文件中（例如 `~/.cargo/credentials.toml`）：

```
[registries.gitea]
token = "Bearer {token}"
```

| 参数    | 描述                                                                                  |
| ------- | ------------------------------------------------------------------------------------- |
| `token` | 您的[个人访问令牌](development/api-usage.md#通过-api-认证) |

## 发布软件包

在项目中运行以下命令来发布软件包：

```shell
cargo publish
```

如果已经存在同名和版本的软件包，您将无法发布新的软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表安装软件包，请执行以下命令：

```shell
cargo add {package_name}
```

| 参数           | 描述         |
| -------------- | ------------ |
| `package_name` | 软件包名称。 |

## 支持的命令

```
cargo publish
cargo add
cargo install
cargo yank
cargo unyank
cargo search
```
