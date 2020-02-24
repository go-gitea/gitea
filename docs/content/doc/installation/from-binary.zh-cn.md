---
date: "2016-12-01T16:00:00+02:00"
title: "从二进制安装"
slug: "install-from-binary"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "从二进制安装"
    weight: 20
    identifier: "install-from-binary"
---

# 从二进制安装

所有下载均包括 SQLite, MySQL 和 PostgreSQL 的支持，同时所有资源均已嵌入到可执行程序中，这一点和老版本有所不同。 基于二进制的安装非常简单，只要从 [下载页面](https://dl.gitea.io/gitea) 选择对应平台，拷贝下载URL，执行以下命令即可（以Linux为例）：

```
wget -O gitea https://dl.gitea.io/gitea/{{< version >}}/gitea-{{< version >}}-linux-amd64
chmod +x gitea
```

## 测试

在执行了以上步骤之后，你将会获得 `gitea` 的二进制文件，在你复制到部署的机器之前可以先测试一下。在命令行执行完后，你可以 `Ctrl + C` 关掉程序。

```
./gitea web
```

## 需要帮助?

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "seek-help.zh-cn.md" >}})
