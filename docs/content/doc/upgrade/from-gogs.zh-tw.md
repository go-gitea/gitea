---
date: "2016-12-01T16:00:00+02:00"
title: "從 Gogs 升級"
slug: "upgrade-from-gogs"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "upgrade"
    name: "從 Gogs 升級"
    weight: 10
    identifier: "upgrade-from-gogs"
---

# 從 Gogs 升級

假如您正在運行 Gogs 0.9.146 以下版本，你可以很平順的升級到 Gitea，請參考底下升級步驟：

* 停止 Gogs 服務。
* 複製 Gogs 設定檔 `custom/conf/app.ini` 到 Gitea 相對應位置。
* 複製 Gogs `conf/` 目錄到 Gitea `options/` 目錄。
* 假如您還有更多自訂的檔案在 `custom/` 目錄，像是多國語系檔案或模板，你應該手動將設定轉移到 Gitea 上，因為這些檔案在 Gitea 上有些不同。
* 複製 `data/` 目錄到 Gitea 相對應目錄，此目錄包含 issue 附件檔及頭像。
* 啟動 Gitea 服務
* 進入 Gitea 管理介面，執行 `重新產生 '.ssh/authorized_keys' 檔案` (警告: 非 Gitea 金鑰將被刪除) 和 `重新產生全部倉庫 update hook` (當自訂設定檔已經被修改，則需要此步驟)。
