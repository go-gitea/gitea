---
date: "2019-04-15T17:29:00+08:00"
title: "遷移介面"
slug: "migrations-interfaces"
weight: 55
toc: false
draft: false
aliases:
  - /zh-tw/migrations-interfaces
menu:
  sidebar:
    parent: "development"
    name: "遷移介面"
    weight: 55
    identifier: "migrations-interfaces"
---

# 遷移功能

完整的遷移從 Gitea 1.9.0 開始提供。它定義了兩個介面用來從其它 Git 託管平臺遷移儲存庫資料到 Gitea，未來或許會提供遷移到其它 git 託管平臺。
目前已實作了從 Github, Gitlab 和其它 Gitea 遷移資料。

Gitea 定義了一些基本物件於套件 [modules/migration](https://github.com/go-gitea/gitea/tree/master/modules/migration)。
分別是 `Repository`, `Milestone`, `Release`, `ReleaseAsset`, `Label`, `Issue`, `Comment`, `PullRequest`, `Reaction`, `Review`, `ReviewComment`。

## Downloader 介面

從新的 Git 託管平臺遷移，有兩個新的步驟。

- 您必須實作一個 `Downloader`，它用來取得儲存庫資訊。
- 您必須實作一個 `DownloaderFactory`，它用來偵測 URL 是否符合並建立上述的 `Downloader`。
  - 您需要在 `init()` 透過 `RegisterDownloaderFactory` 來註冊 `DownloaderFactory`。

您可以在 [downloader.go](https://github.com/go-gitea/gitea/blob/main/modules/migration/downloader.go) 中找到這些介面。

## Uploader 介面

目前只有 `GiteaLocalUploader` 被實作出來，所以我們只能通過 `Uploader` 儲存已下載的資料到本地的 Gitea 實例。
目前尚未支援其它 Uploader。

您可以在 [uploader.go](https://github.com/go-gitea/gitea/blob/main/modules/migration/uploader.go) 中找到這些介面。
