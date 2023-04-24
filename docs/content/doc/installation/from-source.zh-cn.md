---
date: "2016-12-01T16:00:00+02:00"
title: "从源代码安装"
slug: "install-from-source"
weight: 30
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "从源代码安装"
    weight: 30
    identifier: "install-from-source"
---

# 从源代码安装

首先你需要安装Golang，关于Golang的安装，参见[官方文档](https://golang.google.cn/doc/install)。

其次你需要[安装Node.js](https://nodejs.org/zh-cn/download/)，Node.js 和 npm 将用于构建 Gitea 前端。

**目录**

{{< toc >}}

## 下载

你需要获取Gitea的源码，最方便的方式是使用 `git` 命令。执行以下命令：

```
git clone https://github.com/go-gitea/gitea
cd gitea
```

然后你可以选择编译和安装的版本，当前你有多个选择。如果你想编译 `main` 版本，你可以直接跳到 [编译](#编译) 部分，这是我们的开发分支，虽然也很稳定但不建议您在正式产品中使用。

如果你想编译最新稳定分支，你可以执行以下命令签出源码：

```bash
git branch -a
git checkout v{{< version >}}
```

最后，你也可以直接使用标签版本如 `v{{< version >}}`。你可以执行以下命令列出可用的版本并选择某个版本签出：

```bash
git tag -l
git checkout v{{< version >}}
```

## 编译

要从源代码进行编译，以下依赖程序必须事先安装好：

- `go` {{< min-go-version >}} 或以上版本, 详见[这里](https://golang.google.cn/doc/install)
- `node` {{< min-node-version >}} 或以上版本，并且安装 `npm`, 详见[这里](https://nodejs.org/zh-cn/download/)
- `make`, 详见[这里](/zh-cn/hacking-on-gitea/)

各种可用的 [make 任务](https://github.com/go-gitea/gitea/blob/main/Makefile)
可以用来使编译过程更方便。

按照您的编译需求，以下 tags 可以使用：

- `bindata`: 这个编译选项将会把运行Gitea所需的所有外部资源都打包到可执行文件中，这样部署将非常简单因为除了可执行程序将不再需要任何其他文件。
- `sqlite sqlite_unlock_notify`: 这个编译选项将启用SQLite3数据库的支持，建议只在少数人使用时使用这个模式。
- `pam`: 这个编译选项将会启用 PAM (Linux Pluggable Authentication Modules) 认证，如果你使用这一认证模式的话需要开启这个选项。

使用 bindata 可以打包资源文件到二进制可以使开发和测试更容易，你可以根据自己的需求决定是否打包资源文件。
要包含资源文件，请使用 `bindata` tag：

```bash
TAGS="bindata" make build
```

默认的发布版本中的编译选项是： `TAGS="bindata sqlite sqlite_unlock_notify"`。以下为推荐的编译方式：

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

## 测试

在执行了以上步骤之后，你将会获得 `gitea` 的二进制文件，在你复制到部署的机器之前可以先测试一下。在命令行执行完后，你可以 `Ctrl + C` 关掉程序。

```bash
./gitea web
```

## 交叉编译

Go 编译器支持交叉编译到不同的目标架构。有关 Go 支持的目标架构列表，请参见 [Optional environment variables](https://go.dev/doc/install/source#environment)。

交叉构建适用于 Linux ARM64 的 Gitea：

```bash
GOOS=linux GOARCH=arm64 make build
```

交叉构建适用于 Linux ARM64 的 Gitea，并且带上 Gitea 发行版采用的编译选项：

```bash
CC=aarch64-unknown-linux-gnu-gcc GOOS=linux GOARCH=arm64 TAGS="bindata sqlite sqlite_unlock_notify" make build
```

## 需要帮助?

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "seek-help.zh-cn.md" >}})
