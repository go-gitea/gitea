---
date: "2016-12-01T16:00:00+02:00"
title: "加入 Gitea 开源"
slug: "hacking-on-gitea"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "加入 Gitea 开源"
    weight: 10
    identifier: "hacking-on-gitea"
---

# Hacking on Gitea

首先你需要一些运行环境，这和 [从源代码安装]({{< relref "from-source.zh-cn.md" >}}) 相同，如果你还没有设置好，可以先阅读那个章节。

如果你想为 Gitea 贡献代码，你需要 Fork 这个项目并且以 `master` 为开发分支。Gitea 使用 Govendor
来管理依赖，因此所有依赖项都被工具自动 copy 在 vendor 子目录下。用下面的命令来下载源码：

```
go get -d code.gitea.io/gitea
```

然后你可以在 Github 上 fork [Gitea 项目](https://github.com/go-gitea/gitea)，之后可以通过下面的命令进入源码目录：

```
cd $GOPATH/src/code.gitea.io/gitea
```

要创建 pull requests 你还需要在源码中新增一个 remote 指向你 Fork 的地址，直接推送到 origin 的话会告诉你没有写权限：

```
git remote rename origin upstream
git remote add origin git@github.com:<USERNAME>/gitea.git
git fetch --all --prune
```

然后你就可以开始开发了。你可以看一下 `Makefile` 的内容。`make test` 可以运行测试程序， `make build` 将生成一个 `gitea` 可运行文件在根目录。如果你的提交比较复杂，尽量多写一些单元测试代码。

好了，到这里你已经设置好了所有的开发 Gitea 所需的环境。欢迎成为 Gitea 的 Contributor。
