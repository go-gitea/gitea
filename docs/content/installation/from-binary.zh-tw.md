---
date: "2016-12-01T16:00:00+02:00"
title: "執行檔安裝"
slug: "install-from-binary"
sidebar_position: 15
toc: false
draft: false
aliases:
  - /zh-tw/install-from-binary
menu:
  sidebar:
    parent: "installation"
    name: "執行檔"
    sidebar_position: 15
    identifier: "install-from-binary"
---

# 從執行檔安裝

所有的執行檔皆支援 SQLite, MySQL and PostgreSQL，且所有檔案都已經包在執行檔內，這一點跟之前的版本有所不同。關於執行檔的安裝方式非常簡單，只要從[下載頁面](https://dl.gitea.com/gitea)選擇相對應平台，複製下載連結，使用底下指令就可以完成了:

```
wget -O gitea https://dl.gitea.com/gitea/@version@/gitea-@version@-linux-amd64
chmod +x gitea
```

## 測試

執行完上述步驟，您將會得到 `gita` 執行檔，在複製到遠端伺服器前，您可以先測試看看，在命令列執行完成後，可以透過 `Ctrl + C` 關閉程式。

```
./gitea web
```

## 需要協助？

如果本頁中無法解決您的問題，請直接到 [Discord server](https://discord.gg/Gitea)，在那邊可以快速得到協助。
