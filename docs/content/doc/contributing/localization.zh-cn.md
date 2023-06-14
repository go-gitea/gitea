---
date: "2016-12-01T16:00:00+02:00"
title: "本地化"
slug: "localization"
weight: 70
toc: false
draft: false
aliases:
  - /zh-cn/localization
menu:
  sidebar:
    parent: "contributing"
    name: "本地化"
    weight: 70
    identifier: "localization"
---

# 本地化

Gitea的本地化是通过我们的[Crowdin项目](https://crowdin.com/project/gitea)进行的。

对于对**英语翻译**的更改，可以发出pull-request，来更改[英语语言环境](https://github.com/go-gitea/gitea/blob/main/options/locale/locale_en-US.ini)中合适的关键字。

有关对**非英语**翻译的更改，请参阅上面的 Crowdin 项目。

## 支持的语言

上述 Crowdin 项目中列出的任何语言一旦翻译了 25% 或更多都将得到支持。

翻译被接受后，它将在下一次 Crowdin 同步后反映在主存储库中，这通常是在任何 PR 合并之后。

在撰写本文时，这意味着更改后的翻译可能要到 Gitea 的下一个版本才会出现。

如果使用开发版本，则在同步更改内容后，它应该会在更新后立即显示。
