---
date: "2016-12-21T15:00:00-02:00"
title: "註冊為 Windows 服務"
slug: "windows-service"
sidebar_position: 50
toc: false
draft: false
aliases:
  - /zh-tw/windows-service
menu:
  sidebar:
    parent: "installation"
    name: "Windows 服務"
    sidebar_position: 50
    identifier: "windows-service"
---

# 事前準備

確認您的 C:\gitea\custom\conf\app.ini 中包含：

```
RUN_USER = COMPUTERNAME$
```

設定 Gitea 以本地使用者身份執行。

請將在命令提示字元（cmd）執行 `echo %COMPUTERNAME%` 的結果輸入 `COMPUTERNAME`。若回應為 `USER-PC`，請輸入 `RUN_USER = USER-PC$`

## 使用絕對路徑

如果您使用 sqlite3，修改 `PATH` 為完整路徑：

```
[database]
PATH     = c:/gitea/data/gitea.db
```

# 註冊為 Windows 服務

要註冊為 Windows 服務，請先以系統管理員身份開啟命令提示字元，接著執行下列指令：

```
sc.exe create gitea start= auto binPath= "\"C:\gitea\gitea.exe\" web --config \"C:\gitea\custom\conf\app.ini\""
```

別忘記將 `C:\gitea` 取代為您的 Gitea 安裝路徑。

開啟 Windows 的「服務」，並且搜尋服務名稱「gitea」，按右鍵選擇「啟動」。在瀏覽器打開 `http://localhost:3000` 就可以成功看到畫面 (如果修改過連接埠，請自行修正，3000 是預設值)。

## 刪除服務

要刪除 Gitea 服務，請用系統管理員身份開啟命令提示字元，接著執行下列指令：

```
sc.exe delete gitea
```
