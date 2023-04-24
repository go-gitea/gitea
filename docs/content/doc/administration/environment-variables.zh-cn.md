---
date: "2017-04-08T11:34:00+02:00"
title: "环境变量清单"
slug: "environment-variables"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "administration"
    name: "环境变量清单"
    weight: 10
    identifier: "environment-variables"
---

# 环境变量清单

这里是用来控制 Gitea 行为表现的的环境变量清单，您需要在执行如下 Gitea 启动命令前设置它们来确保配置生效：

```
GITEA_CUSTOM=/home/gitea/custom ./gitea web
```

## Go 的配置

因为 Gitea 使用 Go 语言编写，因此它使用了一些相关的 Go 的配置参数：

* `GOOS`
* `GOARCH`
* [`GOPATH`](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable)

您可以在[官方文档](https://golang.org/cmd/go/#hdr-Environment_variables)中查阅这些配置参数的详细信息。

## Gitea 的文件目录

* `GITEA_WORK_DIR`：工作目录的绝对路径
* `GITEA_CUSTOM`：默认情况下 Gitea 使用默认目录 `GITEA_WORK_DIR`/custom，您可以使用这个参数来配置 *custom* 目录
* `GOGS_WORK_DIR`： 已废弃，请使用 `GITEA_WORK_DIR` 替代
* `GOGS_CUSTOM`： 已废弃，请使用 `GITEA_CUSTOM` 替代

## 操作系统配置

* `USER`：Gitea 运行时使用的系统用户，它将作为一些 repository 的访问地址的一部分
* `USERNAME`： 如果没有配置 `USER`， Gitea 将使用 `USERNAME`
* `HOME`： 用户的 home 目录，在 Windows 中会使用 `USERPROFILE` 环境变量

### 仅限于 Windows 的配置

* `USERPROFILE`： 用户的主目录，如果未配置则会使用 `HOMEDRIVE` + `HOMEPATH`
* `HOMEDRIVE`: 用于访问 home 目录的主驱动器路径（C盘）
* `HOMEPATH`：在指定主驱动器下的 home 目录相对路径

## Miscellaneous

* `SKIP_MINWINSVC`：如果设置为 1，在 Windows 上不会以 service 的形式运行。
