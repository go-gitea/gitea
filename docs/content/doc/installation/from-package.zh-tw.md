---
date: "2016-12-01T16:00:00+02:00"
title: "套件安裝"
slug: "install-from-package"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "套件安裝"
    weight: 20
    identifier: "install-from-package"
---

# 從套件安裝

## Linux

目前尚未發佈任何 Linux 套件，如果我們發佈了，會直接更新此網頁。在這之前請先參考[執行檔安裝]({{< relref "from-binary.zh-tw.md" >}})方式。

## Windows

目前尚未發佈任何 Windows 套件，如果我們發佈了，會直接更新此網頁。我們計畫使用 `MSI`，或 [Chocolatey](https://chocolatey.org/) 來製作套件。在這之前請先參考[執行檔安裝]({{< relref "from-binary.zh-tw.md" >}})方式。

## macOS

目前我們只支援透過 `brew` 來安裝套件。假如您尚未使用 [Homebrew](http://brew.sh/)，您就必須參考[執行檔安裝]({{< relref "from-binary.zh-tw.md" >}})方式。透過 `brew` 安裝 Gitea，您只需要執行底下指令:

```
brew tap go-gitea/gitea
brew install gitea
```

## FreeBSD

下載 FreeBSD port `www/gitea` 套件。你可以安裝 pre-built 執行檔:

```
pkg install gitea
```

對於最新版本或想要自行編譯特定選項，請使用 [port 安裝](https://www.freebsd.org/doc/handbook/ports-using.html):

```
su -
cd /usr/ports/www/gitea
make install clean
```

## 需要協助？

如果本頁中無法解決您的問題，請直接到 [Discord server](https://discord.gg/NsatcWJ)，在那邊可以快速得到協助。
