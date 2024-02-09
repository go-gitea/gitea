---
date: "2023-05-23T09:00:00+08:00"
title: "搜索引擎索引"
slug: "search-engines-indexation"
sidebar_position: 60
toc: false
draft: false
aliases:
  - /zh-cn/search-engines-indexation
menu:
  sidebar:
    parent: "administration"
    name: "搜索引擎索引"
    sidebar_position: 60
    identifier: "search-engines-indexation"
---

# Gitea 安装的搜索引擎索引

默认情况下，您的 Gitea 安装将被搜索引擎索引。
如果您不希望您的仓库对搜索引擎可见，请进一步阅读。

## 使用 robots.txt 阻止搜索引擎索引

为了使 Gitea 为顶级安装提供自定义的`robots.txt`（默认为空的 404），请在 [`custom`文件夹或`CustomPath`]（administration/customizing-gitea.md）中创建一个名为 `public/robots.txt` 的文件。

有关如何配置 `robots.txt` 的示例，请参考 [https://moz.com/learn/seo/robotstxt](https://moz.com/learn/seo/robotstxt)。

```txt
User-agent: *
Disallow: /
```

如果您将Gitea安装在子目录中，则需要在顶级目录中创建或编辑 `robots.txt`。

```txt
User-agent: *
Disallow: /gitea/
```
