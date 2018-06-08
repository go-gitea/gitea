---
date: "2016-11-08T16:00:00+02:00"
title: "文件"
slug: "documentation"
url: "/zh-tw/"
weight: 10
toc: true
draft: false
---

# 關於 Gitea

Gitea 是一個可自行託管的 Git 服務。你可以拿 GitHub、Bitbucket 或 Gitlab 來比較看看。初期是從 [Gogs](http://gogs.io) 發展而來，不過我們已經 Fork 並且命名為 Gitea。如果您想更深入了解 Fork 原因，請直接參考[這裡](https://blog.gitea.io/2016/12/welcome-to-gitea/)。

## 目標

Gitea 的首要目標是建立一個容易安裝，運行快速，安装和使用體驗良好的自建 Git 服務。我們採用 GO 為後端語言，Go 可以產生各平台使用的執行檔。除了支援 Linux、macOS 和 Windows 外，甚至還包含 ARM 和 PowerPC。

## 功能

- 支援個人活動時間表
- 支援 SSH 和 HTTP/HTTPS 協定
- 支援 SMTP/LDAP/Reverse 代理認證
- 支援反向代理子路徑
- 支援帳號/組織/儲存庫管理
- 支援新增/刪除儲存庫合作帳號
- 支援儲存庫/組織 webhooks (包含 Slack)
- 支援儲存庫 Git hooks/部署金鑰
- 支援儲存庫問題列表 (issues), 合併請求 (pull requests) 及 wiki
- 支援遷移及複製儲存庫及 wiki
- 支援線上編輯儲存庫檔案及 wiki
- 支援自訂來源 Gravatar 及 Federated avatar
- 支援郵件服務
- 支援後台管理
- 支援 MySQL, PostgreSQL, SQLite3, MSSQL 和 [TiDB](https://github.com/pingcap/tidb) (實驗性)
- 支援多國語言 ([21 國語言](https://github.com/go-gitea/gitea/tree/master/options/locale))

## 系統需求

- 最低的系統需求就是一片便宜的樹莓派 (Raspberry Pi)。
- 如果用於團隊，建議使用 2 核 CPU 和 1GB 記憶體。

## 瀏覽器支援

- 請參考 [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) 所支援的瀏覽器列表。
- 官方支援最小 UI 尺寸為 **1024*768**， UI 在更小尺寸也看起來不錯，但是我們並不保證。

## 元件

* Web 框架： [Macaron](http://go-macaron.com/)
* ORM： [XORM](https://github.com/go-xorm/xorm)
* UI 元件：
  * [Semantic UI](http://semantic-ui.com/)
  * [GitHub Octicons](https://octicons.github.com/)
  * [Font Awesome](http://fontawesome.io/)
  * [DropzoneJS](http://www.dropzonejs.com/)
  * [Highlight](https://highlightjs.org/)
  * [Clipboard](https://zenorocha.github.io/clipboard.js/)
  * [Emojify](https://github.com/Ranks/emojify.js)
  * [CodeMirror](https://codemirror.net/)
  * [jQuery Date Time Picker](https://github.com/xdan/datetimepicker)
  * [jQuery MiniColors](https://github.com/claviska/jquery-minicolors)
* 資料庫：
  * [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  * [github.com/lib/pq](https://github.com/lib/pq)
  * [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  * [github.com/pingcap/tidb](https://github.com/pingcap/tidb)
  * [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)
