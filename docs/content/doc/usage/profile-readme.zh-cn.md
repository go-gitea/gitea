---
date: "2023-05-23T09:00:00+08:00"
title: "个人资料 README"
slug: "profile-readme"
weight: 12
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "个人资料 README"
    weight: 12
    identifier: "profile-readme"
---

# 个人资料 README

要在您的 Gitea 个人资料页面显示一个 Markdown 文件，只需创建一个名为 ".profile" 的仓库，并编辑其中的 README.md 文件。Gitea 将自动获取该文件并在您的仓库上方显示。

注意：您可以将此仓库设为私有。这样可以隐藏您的源文件，使其对公众不可见，并允许您将某些文件设为私有。但是，README.md 文件将是您个人资料上唯一存在的文件。如果您希望完全私有化 .profile 仓库，则需删除或重命名 README.md 文件。
