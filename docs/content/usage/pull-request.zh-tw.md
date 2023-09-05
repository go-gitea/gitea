---
date: "2018-06-01T19:00:00+02:00"
title: "合併請求"
slug: "pull-request"
sidebar_position: 13
toc: false
draft: false
aliases:
  - /zh-tw/pull-request
menu:
  sidebar:
    parent: "usage"
    name: "合併請求"
    sidebar_position: 13
    identifier: "pull-request"
---

# 合併請求

## 「還在進行中（WIP）」的合併請求

將合併請求標記為還在進行中（Work In Progress, WIP）可避免意外地被合併。
要將合併請求標記為還在進行中請在標題中使用 `WIP:` 或 `[WIP]` 前綴（不分大小寫）。這些值可在您的 `app.ini` 中設定：

```ini
[repository.pull-request]
WORK_IN_PROGRESS_PREFIXES=WIP:,[WIP]
```

網頁提示會使用第一個值作為範例。

## 合併請求範本

您可以在[問題與合併請求範本](usage/issue-pull-request-templates.md)找到更多關於合併請求範本的資訊。
