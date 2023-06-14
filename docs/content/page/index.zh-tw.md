---
date: "2016-11-08T16:00:00+02:00"
title: "文件"
slug: "documentation"
url: "/zh-tw/"
weight: 10
toc: false
draft: false
---

# 關於 Gitea

Gitea 是一個可自行託管的 Git 服務。你可以拿 GitHub、Bitbucket 或 Gitlab 來比較看看。
Gitea 是從 [Gogs](http://gogs.io) Fork 出來的，請閱讀部落格文章 [Gitea 公告](https://blog.gitea.io/2016/12/welcome-to-gitea/)以了解我們 Fork 的理由。

## 目標

本專案的首要目標是建立一個容易安裝，執行快速，安装和使用體驗良好的自建 Git 服務。我們採用 GO 為後端語言，Go 可以產生各平台使用的執行檔。它支援 Linux、macOS 和 Windows 外，處理器架構包含 amd64、i386、ARM 和 PowerPC 等。

## 功能

- 使用者面板
  - 內容切換（組織或目前使用者）
  - 動態時間軸
    - 提交
    - 問題
    - 合併請求
    - 儲存庫的建立
  - 可搜尋的儲存庫清單
  - 組織清單
  - 鏡像儲存庫清單
- 問題面板
  - 內容切換（組織或目前使用者）
  - 篩選器
    - 開放中
    - 已關閉
    - 您的儲存庫
    - 被指派的問題
    - 您的問題
    - 儲存庫
  - 排序
    - 最舊
    - 最近更新
    - 留言數量
- 合併請求面板
  - 和問題面板相同
- 儲存庫類型
  - 鏡像
  - 一般
  - 已遷移
- 通知（email 和網頁）
  - 已讀
  - 未讀
  - 釘選
- 探索頁面
  - 使用者
  - 儲存庫
  - 組織
  - 搜尋
- 自訂範本
- 複寫 public 檔案（logo, css 等）
- CSRF 與 XSS 保護
- 支援 HTTPS
- 設定允許上傳的檔案大小和類型
- 日誌
- 組態
  - 資料庫
    - MySQL
    - PostgreSQL
    - SQLite3
    - MSSQL
    - TiDB（MySQL 協議）
  - 設定檔
    - [app.ini](https://github.com/go-gitea/gitea/blob/main/custom/conf/app.example.ini)
  - 管理員面板
    - 系統摘要
    - 維護操作
      - 刪除未啟用帳戶
      - 刪除快取的儲存庫存檔
      - 刪除遺失 Git 檔案的儲存庫
      - 對儲存庫進行垃圾回收
      - 重寫 SSH 金鑰
      - 重新同步 hooks
      - 重新建立遺失的儲存庫
    - 伺服器狀態
      - 執行時間
      - 記憶體
      - 目前的 Goroutines 數量
      - 還有更多……
    - 使用者管理
      - 搜尋
      - 排序
      - 最後登入時間
      - 認證來源
      - 儲存庫上限
      - 停用帳戶
      - 管理員權限
      - 建立 Git Hook 的權限
      - 建立組織的權限
      - 匯入儲存庫的權限
    - 組織管理
      - 成員
      - 團隊
      - 大頭貼
      - Hook
    - 儲存庫管理
      - 查看所有儲存庫資訊和管理儲存庫
    - 認證來源
      - OAuth
      - PAM
      - LDAP
      - SMTP
    - 組態檢視器
      - 所有設定檔中的值
    - 系統提示
      - 當有未預期的事情發生時
    - 應用監控面板
      - 目前的處理程序
      - Cron 任務
        - 更新鏡像
        - 儲存庫健康檢查
        - 檢查儲存庫的統計資料
        - 刪除舊的儲存庫存檔
  - 環境變數
  - 命令列選項
- 支援多國語言 ([21 種語言](https://github.com/go-gitea/gitea/tree/master/options/locale))
- 支援 [Mermaid](https://mermaidjs.github.io/) 圖表
- 郵件服務
  - 通知
  - 確認註冊
  - 重設密碼
- 支援反向代理（Reverse Proxy）
  - 包含子路徑
- 使用者

  - 個人資料
    - 姓名
    - 帳號
    - 電子信箱
    - 網站
    - 加入日期
    - 追蹤者和追蹤中
    - 組織
    - 儲存庫
    - 動態
    - 已加星號的儲存庫
  - 設定
    - 和個人資料相同並包含下列功能
    - 隱藏電子信箱
    - 大頭貼
      - Gravatar
      - Libravatar
      - 自訂
    - 密碼
    - 多個電子信箱
    - SSH 金鑰
    - 已連結的應用程式
    - 兩步驟驗證
    - 已連結 OAuth2 來源
    - 刪除帳戶

- 儲存庫
  - 以 SSH/HTTP/HTTPS Clone
  - Git LFS
  - 關注、星號、Fork
  - 檢視關注、已加星號、Fork 的使用者
  - 程式碼
    - 瀏覽分支
    - 從網頁上傳和建立檔案
    - Clone url
    - 下載
      - ZIP
      - TAR.GZ
    - 網頁程式碼編輯器
      - Markdown 編輯器
      - 文本編輯器
        - 語法高亮
      - 預覽差異
      - 預覽
      - 選擇提交目標分支
    - 檢視檔案歷史
    - 刪除檔案
    - 檢視 raw
  - 問題
    - 問題範本
    - 里程碑
    - 標籤
    - 指派問題
    - 時間追蹤
    - 表情反應
    - 篩選器
      - 開放中
      - 已關閉
      - 被指派的人
      - 您的問題
      - 提及您
    - 排序
      - 最舊
      - 最近更新
      - 留言數量
    - 搜尋
    - 留言
    - 附件
  - 合併請求
    - 功能和問題相同
  - 提交
    - 提交線圖
    - 不同分支的提交
    - 搜尋
    - 在所有分支中搜尋
    - 檢視差異（diff）
    - 檢視 SHA
    - 檢視作者（author）
    - 瀏覽提交中的檔案
  - 版本發佈
    - 附件
    - 標題
    - 內容
    - 刪除
    - 標記為 pre-release
    - 選擇分支
  - Wiki
    - 匯入
    - Markdown 編輯器
  - 設定
    - 選項
      - 名稱
      - 描述
      - 私有/公開
      - 網站
      - Wiki
        - 開啟/關閉
        - 內部/外部
      - 問題
        - 開啟/關閉
        - 內部/外部
        - 外部問題追蹤器支援 URL 重寫（URL Rewrite）以獲得更好的整合性
      - 開啟/關閉合併請求
      - 轉移儲存庫所有權
      - 刪除 wiki
      - 刪除儲存庫
    - 協作者
      - 讀取/寫入/管理員
    - 分支
      - 預設分支
      - 分支保護
    - Webhook
    - Git hook
    - 部署金鑰

## 系統需求

- Raspberry Pi 3 的效能足夠讓 Gitea 承擔小型工作負載。
- 雙核心 CPU 和 1GB 記憶體通常足以應付小型團隊/專案。
- 在類 UNIX 系統上， 應該以專用的非 root 系統帳號來執行 Gitea。
  - 備註：Gitea 管理著 `~/.ssh/authorized_keys` 檔案。以一般身份使用者執行 Gitea 可能會破壞該使用者的登入能力。

- [Git](https://git-scm.com/) 的最低需求為 2.0 或更新版本。
  - 當 git 版本 >= 2.1.2 時，可啟用 Git [large file storage](https://git-lfs.github.com/)。
  - 當 git 版本 >= 2.18 時，將自動啟用 Git 提交線圖渲染。

## 瀏覽器支援

- 最近 2 個版本的 Chrome, Firefox, Safari, Edge
- Firefox ESR

## 元件

- Web 框架： [Chi](http://github.com/go-chi/chi)
- ORM： [XORM](https://xorm.io)
- UI 元件：
  - [jQuery](https://jquery.com)
  - [Fomantic UI](https://fomantic-ui.com)
  - [Vue3](https://vuejs.org)
  - [CodeMirror](https://codemirror.net)
  - [EasyMDE](https://github.com/Ionaru/easy-markdown-editor)
  - [Monaco Editor](https://microsoft.github.io/monaco-editor)
  - ... (package.json)
- 資料庫驅動程式：
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## 集成支持

請訪問 [Awesome Gitea](https://gitea.com/gitea/awesome-gitea/) 獲得更多的第三方集成支持
