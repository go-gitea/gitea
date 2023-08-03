---
date: "2023-05-25T17:29:00+08:00"
title: "集成"
slug: "integrations"
sidebar_position: 65
toc: false
draft: false
aliases:
  - /zh-cn/integrations
menu:
  sidebar:
    parent: "development"
    name: "集成"
    sidebar_position: 65
    identifier: "integrations"
---

# 集成

Gitea拥有一个出色的第三方集成社区，以及在其他各种项目中的一流支持。

我们正在[awesome-gitea](https://gitea.com/gitea/awesome-gitea)上整理一个列表来跟踪这些集成！

如果你正在寻找[CI/CD](https://gitea.com/gitea/awesome-gitea#user-content-devops)，
一个[SDK](https://gitea.com/gitea/awesome-gitea#user-content-sdk)，
甚至一些额外的[主题](https://gitea.com/gitea/awesome-gitea#user-content-themes)，
你可以在[awesome-gitea](https://gitea.com/gitea/awesome-gitea)中找到它们的列表！

## 预填新文件名和内容

如果你想打开一个具有给定名称和内容的新文件，
你可以使用查询参数来实现：

```txt
GET /{{org}}/{{repo}}/_new/{{filepath}}
    ?filename={{filename}}
    &value={{content}}
```

例如：

```txt
GET https://git.example.com/johndoe/bliss/_new/articles/
    ?filename=hello-world.md
    &value=Hello%2C%20World!
```
