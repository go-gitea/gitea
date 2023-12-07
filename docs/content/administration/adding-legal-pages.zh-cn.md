---
date: "2023-05-23T09:00:00+08:00"
title: "添加法律页面"
slug: adding-legal-pages
sidebar_position: 110
toc: false
draft: false
aliases:
  - /zh-cn/adding-legal-pages
menu:
  sidebar:
    parent: "administration"
    name: "添加法律页面"
    identifier: "adding-legal-pages"
    sidebar_position: 110
---

一些法域（例如欧盟）要求在网站上添加特定的法律页面（例如隐私政策）。按照以下步骤将它们添加到你的 Gitea 实例中。

## 获取页面

Gitea 源代码附带了示例页面，位于 `contrib/legal` 目录中。将它们复制到 `custom/public/` 目录下。例如，如果要添加隐私政策：

```
wget -O /path/to/custom/public/privacy.html https://raw.githubusercontent.com/go-gitea/gitea/main/contrib/legal/privacy.html.sample
```

现在，你需要编辑该页面以满足你的需求。特别是，你必须更改电子邮件地址、网址以及与 "Your Gitea Instance" 相关的引用，以匹配你的情况。

请务必不要放置会暗示 Gitea 项目对你的服务器负责的一般服务条款或隐私声明。

## 使其可见

创建或追加到 `/path/to/custom/templates/custom/extra_links_footer.tmpl` 文件中：

```go
<a class="item" href="{{AppSubUrl}}/assets/privacy.html">隐私政策</a>
```

重启 Gitea 以查看更改。
