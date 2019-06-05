---
date: "2016-12-01T16:00:00+02:00"
title: "选择包安装"
slug: "install-from-package"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "选择包安装"
    weight: 20
    identifier: "install-from-package"
---

{{% h1 %}}使用包安装{{% /h1 %}}

{{% h2 %}}Linux{{% /h2 %}}

目前还没有对应的Linux安装包发布，如果我们发布了，我们将更新本页面。当前你可以查看 [从二进制安装]({{< relref "from-binary.zh-cn.md" >}})。

{{% h2 %}}Windows{{% /h2 %}}

目前还没有对应的Windows安装包发布，如果我们发布了，我们将更新本页面。我们计划使用 `MSI` 安装器或者 [Chocolatey](https://chocolatey.org/)来制作安装包。当前你可以查看 [从二进制安装]({{< relref "from-binary.zh-cn.md" >}})。

{{% h2 %}}macOS{{% /h2 %}}

macOS 平台下当前我们仅支持通过 `brew` 来安装。如果您没有安装 [Homebrew](http://brew.sh/)，你冶可以查看 [从二进制安装]({{< relref "from-binary.zh-cn.md" >}})。在你安装了 `brew` 之后， 你可以执行以下命令：

```
brew tap go-gitea/gitea
brew install gitea
```

{{% h2 %}}需要帮助?{{% /h2 %}}

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "seek-help.zh-cn.md" >}})
