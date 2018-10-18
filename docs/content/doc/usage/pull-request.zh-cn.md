---
date: "2018-06-01T19:00:00+02:00"
title: "使用：Pull Request"
slug: "pull-request"
weight: 13
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Pull Request"
    weight: 13
    identifier: "pull-request"
---

# Pull Request

## 在 pull requests 使用“Work In Progress”标记

您可以通过在一个进行中的 pull request 的标题上添加前缀 `WIP:` 或者 `[WIP]`（此处大小写敏感）来防止它被意外合并，具体的前缀设置可以在配置文件 `app.ini` 中找到：

```
[repository.pull-request]
WORK_IN_PROGRESS_PREFIXES=WIP:,[WIP]
```

列表的第一个值将用于 helpers 程序。

## Pull Request 模板

有关 pull request 模板的更多信息请您移步 : [Issue and Pull Request templates](../issue-pull-request-templates)
