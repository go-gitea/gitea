---
date: "2017-01-01T16:00:00+02:00"
title: "用法: 備份與還原"
slug: "backup-and-restore"
weight: 11
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "備份與還原"
    weight: 11
    identifier: "backup-and-restore"
---

# 備份與還原

Gitea 目前支援 `dump` 指令，用來將資料備份成 zip 檔案，後續會提供還原指令，讓你可以輕易的將備份資料及還原到另外一台機器。

## 備份指令 (`dump`)

首先，切換到執行 Gitea 的使用者: `su git` (請修改成您指定的使用者)，並在安裝目錄內執行 `./gitea dump` 指令，你可以看到 console 畫面如下:

```
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

備份出來的 `gitea-dump-1482906742.zip` 檔案，檔案內會包含底下內容:

* `custom/conf/app.ini` - 伺服器設定檔。
* `gitea-db.sql` - SQL 備份檔案。
* `gitea-repo.zip` - 此 zip 檔案為全部的 repo 目錄。
   請參考 Config -> repository -> `ROOT` 所設定的路徑。
* `log/` - 全部 logs 檔案，如果你要 migrate 到其他伺服器，此目錄不用保留。

你可以透過設定 `--tempdir` 指令參數來指定備份檔案目錄，或者是設定 `TMPDIR` 環境變數來達到此功能。

## 還原指令 (`restore`)

持續更新中: 此文件尚未完成.
