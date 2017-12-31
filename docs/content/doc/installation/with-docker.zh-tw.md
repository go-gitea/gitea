---
date: "2016-12-01T16:00:00+02:00"
title: "Docker 安裝"
slug: "install-with-docker"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Docker 安裝"
    weight: 10
    identifier: "install-with-docker"
---

# 用 Docker 安裝

我們在 Docker Hub 提供了自動更新的映像檔，它會保持最新穩定版。根據您的部屬環境來使用最新版本或用其他服務來更新 Docker 映像檔。首先您需要下載映像檔：

```
docker pull gitea/gitea:latest
```

為了儲存您的所有 Git 儲存庫資料，您應該建立一個目錄，用來存放資料的地方。

```
sudo mkdir -p /var/lib/gitea
```

現在就可以直接啟動 Docker 容器，這是一個非常簡單的過程，您必須定義啟動連接埠，並且提供上面所建立的資料儲存路徑:

```
docker run -d --name=gitea -p 10022:22 -p 10080:3000 -v /var/lib/gitea:/data gitea/gitea:latest
```

然後 Gitea 容器已經開始運行，您可以透過個人喜愛的瀏覽器來訪問 http://hostname:10080，假如您想要開始 Clone 儲存庫，可以直接執行 `git clone ssh://git@hostname:10022/username/repo.git` 指令。

## 需要協助？

如果本頁中無法解決您的問題，請直接到 [Discord server](https://discord.gg/NsatcWJ)，在那邊可以快速得到協助。
