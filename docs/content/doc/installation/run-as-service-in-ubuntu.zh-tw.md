---
date: "2017-07-21T12:00:00+02:00"
title: "在 Linux 中以服務執行"
slug: "linux-service"
weight: 40
toc: false
draft: false
aliases:
  - /zh-tw/linux-service
menu:
  sidebar:
    parent: "installation"
    name: "Linux 服務"
    weight: 40
    identifier: "linux-service"
---

### 以 Linux 服務執行 Gitea

您可使用 systemd 或 supervisor 以服務的方式執行 Gitea。下列步驟已在 Ubuntu 16.04 中測試，但它們應該適用於所有的 Linux 發行版（只需要一些小小的調整）。

#### 使用 systemd

複製範例 [gitea.service](https://github.com/go-gitea/gitea/blob/main/contrib/systemd/gitea.service) 到 `/etc/systemd/system/gitea.service` 後用您喜愛的文字編輯器開啟檔案。

取消註解任何需要在此系統上啟動的服務像是 MySQL。

修改 user, home directory 和其它必要的啟動參數。若預設埠已被占用請修改埠號或移除「-p」旗標。

在系統啟動時啟用並執行 Gitea：

```
sudo systemctl enable gitea
sudo systemctl start gitea
```

若您使用 systemd 220 或更新版本，您能以一行指令啟動並立即執行 Gitea：

```
sudo systemctl enable gitea --now
```

#### 使用 supervisor

在終端機使用下列指令安裝 supervisor：

```
sudo apt install supervisor
```

為 supervisor 建立 log 資料夾：

```
# assuming Gitea is installed in /home/git/gitea/
mkdir /home/git/gitea/log/supervisor
```

附加範例 [supervisord config](https://github.com/go-gitea/gitea/blob/main/contrib/supervisor/gitea) 的設定值到 `/etc/supervisor/supervisord.conf`。

用您喜愛的文字編輯器修改使用者（git）和家目錄（/home/git）設定以符合部署環境。若預設埠已被占用請修改埠號或移除「-p」旗標。

最後設定在系統啟動時啟用並執行 supervisor：

```
sudo systemctl enable supervisor
sudo systemctl start supervisor
```

若您使用 systemd 220 或更新版本，您能以一行指令啟動並立即執行 supervisor：

```
sudo systemctl enable supervisor --now
```
