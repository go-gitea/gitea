---
date: "2023-05-23T09:00:00+08:00"
title: "克隆过滤器 (部分克隆)"
slug: "clone-filters"
sidebar_position: 25
draft: false
toc: false
aliases:
  - /zh-cn/clone-filters
menu:
  sidebar:
    parent: "usage"
    name: "克隆过滤器"
    sidebar_position: 25
    identifier: "clone-filters"
---

# 克隆过滤器 (部分克隆)

Git 引入了 `--filter` 选项用于 `git clone` 命令，该选项可以过滤掉大文件和对象（如 blob），从而创建一个仓库的部分克隆。克隆过滤器对于大型仓库和/或按流量计费的连接特别有用，因为完全克隆（不使用 `--filter`）可能会很昂贵（需要下载所有历史数据）。

这需要 Git 2.22 或更高版本，无论是在 Gitea 服务器上还是在客户端上都需要如此。为了使克隆过滤器正常工作，请确保客户端上的 Git 版本至少与服务器上的版本相同（或更高）。以管理员身份登录到 Gitea，然后转到管理后台 -> 应用配置，查看服务器的 Git 版本。

默认情况下，克隆过滤器是启用的，除非在 `[git]` 下将 `DISABLE_PARTIAL_CLONE` 设置为 `true`。

请参阅 [GitHub 博客文章：了解部分克隆](https://github.blog/2020-12-21-get-up-to-speed-with-partial-clone-and-shallow-clone/) 以获取克隆过滤器的常见用法（无 Blob 和无树的克隆），以及 [GitLab 部分克隆文档](https://docs.gitlab.com/ee/topics/git/partial_clone.html) 以获取更高级的用法（例如按文件大小过滤和取消过滤以将部分克隆转换为完全克隆）。
