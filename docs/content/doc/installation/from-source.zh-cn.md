---
date: "2016-12-01T16:00:00+02:00"
title: "从源代码安装"
slug: "install-from-source"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "从源代码安装"
    weight: 30
    identifier: "install-from-source"
---

# 从源代码安装

首先你需要安装Golang，关于Golang的安装，参见官方文档 [install instructions](https://golang.org/doc/install)。

## 下载

你需要获取Gitea的源码，最方便的方式是使用 go 命令。执行以下命令：

```
go get -d -u code.gitea.io/gitea
cd $GOPATH/src/code.gitea.io/gitea
```

然后你可以选择编译和安装的版本，当前你有多个选择。如果你想编译 `master` 版本，你可以直接跳到 [编译](#build) 部分，这是我们的开发分支，虽然也很稳定但不建议您在正式产品中使用。

如果你想编译最新稳定分支，你可以执行以下命令签出源码：

```
git branch -a
git checkout v1.0
```

最后，你也可以直接使用标签版本如 `v1.0.0`。你可以执行以下命令列出可用的版本并选择某个版本签出：

```
git tag -l
git checkout v1.0.0
```

## 编译

我们已经将所有的依赖项拷贝到本工程，我们提供了一些 [编译选项](https://github.com/go-gitea/gitea/blob/master/Makefile) 来让编译更简单。你可以按照你的需求来设置编译开关，可用编译选项如下：

* `bindata`: 这个编译选项将会把运行Gitea所需的所有外部资源都打包到可执行文件中，这样部署将非常简单因为除了可执行程序将不再需要任何其他文件。
* `sqlite sqlite_unlock_notify`: 这个编译选项将启用SQLite3数据库的支持，建议只在少数人使用时使用这个模式。
* `pam`: 这个编译选项将会启用 PAM (Linux Pluggable Authentication Modules) 认证，如果你使用这一认证模式的话需要开启这个选项。

我们支持两种方式进行编译，Make 工具 和 Go 工具。不过我们推荐使用 Make工具，因为他将会给出更多的编译选项。

**Note**: We recommend the Go version 1.6 or higher because we are using vendoring and we don't set the required env variable for 1.5 anywhere.

* Make 工具

这个编译方式要求你先安装Make工具，关于Make工具的安装你可以参考Make相关资料。同样如果要使用bindata选项，你可能需要先执行make generate：

```
TAGS="bindata" make generate build
```

* Go 工具

使用 Go 工具编译需要你至少安装了Go 1.5以上版本并且将 govendor 的支持打开。执行命令如下：

```
go build
```

## 测试

在执行了以上步骤之后，你将会获得 `gitea` 的二进制文件，在你复制到部署的机器之前可以先测试一下。在命令行执行完后，你可以 `Ctrl + C` 关掉程序。

```
./gitea web
```

## 需要帮助?

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "seek-help.zh-cn.md" >}})
