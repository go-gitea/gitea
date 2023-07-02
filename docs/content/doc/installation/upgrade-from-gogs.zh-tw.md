---
date: "2016-12-01T16:00:00+02:00"
title: "從 Gogs 升級"
slug: "upgrade-from-gogs"
weight: 101
toc: false
draft: false
aliases:
  - /zh-tw/upgrade-from-gogs
menu:
  sidebar:
    parent: "installation"
    name: "從 Gogs 升級"
    weight: 101
    identifier: "upgrade-from-gogs"
---

# 從 Gogs 升級

**目錄**

{{< toc >}}

若您正在執行 Gogs 0.9.146 以下版本，您可以很簡單地遷移到 Gitea。

請參考下列步驟。在 Linux 系統上請以 Gogs 的使用者身份執行：

- 使用 `gogs backup` 建立 Gogs 的備份。這會建立檔案 `gogs-backup-[timestamp].zip` 包含所有重要的 Gogs 資料。
  如果稍後您要恢復到 `gogs` 時會用到它。
- 從[下載頁](https://dl.gitea.com/gitea/)下載對應您平臺的檔案。請下載 `1.0.x` 版，從 `gogs` 遷移到其它版本是不可行的。
- 將二進位檔放到適當的安裝位置。
- 複製 `gogs/custom/conf/app.ini` 到 `gitea/custom/conf/app.ini`。
- 從 `gogs/custom/` 複製自訂 `templates, public` 到 `gitea/custom/`。
- `gogs/custom/conf` 中的其它自訂資料夾如： `gitignore, label, license, locale, readme`，
  請複製到 `gitea/custom/options`。
- 複製 `gogs/data/` 到 `gitea/data/`。它包含了問題附件和大頭貼。
- 以指令 `gitea web` 啟動 Gitea 驗證上列設定是否正確。
- 從網頁 UI 進入 Gitea 管理員面板, 執行 `Rewrite '.ssh/authorized_keys' file`。
- 執行每個主要版本的二進位檔 ( `1.1.4` → `1.2.3` → `1.3.4` → `1.4.2` → 等等 ) 以遷移資料庫。
- 如果變更了自訂檔、設定檔路徑，請執行 `Rewrite all update hook of repositories`。

## 修改指定的 gogs 資訊

- 重新命名 `gogs-repositories/` 為 `gitea-repositories/`
- 重新命名 `gogs-data/` 為 `gitea-data/`
- 在 `gitea/custom/conf/app.ini` 中修改：

  修改前：

  ```ini
  [database]
  PATH = /home/:USER/gogs/data/:DATABASE.db
  [attachment]
  PATH = /home/:USER/gogs-data/attachments
  [picture]
  AVATAR_UPLOAD_PATH = /home/:USER/gogs-data/avatars
  [log]
  ROOT_PATH = /home/:USER/gogs/log
  ```

  修改後：

  ```ini
  [database]
  PATH = /home/:USER/gitea/data/:DATABASE.db
  [attachment]
  PATH = /home/:USER/gitea-data/attachments
  [picture]
  AVATAR_UPLOAD_PATH = /home/:USER/gitea-data/avatars
  [log]
  ROOT_PATH = /home/:USER/gitea/log
  ```

- 執行 `gitea web` 啟動 Gitea 檢查是否正確執行

## 升級到最新版的 `gitea`

成功從 `gogs` 升級到 `gitea 1.0.x` 後再用 2 個步驟即可升級到最新版的 `gitea`。

請先升級到 [`gitea 1.6.4`](https://dl.gitea.com/gitea/1.6.4/)，先從[下載頁](https://dl.gitea.com/gitea/1.6.4/)下載
您平臺的二進位檔取代既有的。至少執行一次 Gitea 並確認一切符合預期。

接著重複上述步驟，但這次請使用[最新發行版本](https://dl.gitea.com/gitea/{{< version >}}/)。

## 從更新版本的 Gogs 升級

您也可以從更新版本的 Gogs 升級，但需要更多步驟。
請參考 [#4286](https://github.com/go-gitea/gitea/issues/4286)。

## 疑難排解

- 如果錯誤和 `gitea/custom/templates` 中 的自訂樣板有關，請試著逐一移除它們。
  它們可能和 Gitea 或更新不相容。

## 在 Unix 啟動時執行 Gitea

從 [gitea/contrib](https://github.com/go-gitea/gitea/tree/master/contrib) 更新必要的檔案以取得正確的環境變數。

使用 systemd 的發行版：

- 複製新的腳本到 `/etc/systemd/system/gitea.service`
- 啟動系統時執行服務： `sudo systemctl enable gitea`
- 停用舊的 gogs 腳本： `sudo systemctl disable gogs`

使用 SysVinit 的發行版：

- 複製新的腳本到 `/etc/init.d/gitea`
- 啟動系統時執行服務： `sudo rc-update add gitea`
- 停用舊的 gogs 腳本： `sudo rc-update del gogs`
