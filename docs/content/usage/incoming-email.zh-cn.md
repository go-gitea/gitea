---
date: "2023-05-23T09:00:00+08:00"
title: "邮件接收"
slug: "incoming-email"
sidebar_position: 13
draft: false
toc: false
aliases:
  - /zh-cn/incoming-email
menu:
  sidebar:
    parent: "usage"
    name: "邮件接收"
    sidebar_position: 13
    identifier: "incoming-email"
---

# 邮件接收

Gitea 支持通过接收邮件执行多种操作。本页面描述了如何进行设置。

## 要求

处理接收的电子邮件需要启用 IMAP 功能的电子邮件帐户。
推荐的策略是使用 [电子邮件子地址](https://en.wikipedia.org/wiki/Email_address#Sub-addressing)，但也可以使用 catch-all 邮箱。
接收电子邮件地址中包含一个用户/操作特定的令牌，告诉 Gitea 应执行哪个操作。
此令牌应该出现在 `To` 和 `Delivered-To` 头字段中。

Gitea 会尝试检测自动回复并跳过它们，电子邮件服务器也应该配置以减少接收到的干扰（垃圾邮件、通讯订阅等）。

## 配置

要激活处理接收的电子邮件消息功能，您需要在配置文件中配置 `email.incoming` 部分。

`REPLY_TO_ADDRESS` 包含电子邮件客户端将要回复的地址。
该地址需要包含 `%{token}` 占位符，该占位符将被替换为描述用户/操作的令牌。
此占位符在地址中只能出现一次，并且必须位于地址的用户部分（`@` 之前）。

使用电子邮件子地址的示例可能如下：`incoming+%{token}@example.com`

如果使用 catch-all 邮箱，则占位符可以出现在地址的用户部分的任何位置：`incoming+%{token}@example.com`、`incoming_%{token}@example.com`、`%{token}@example.com`

## 安全性

在选择用于接收传入电子邮件的域时要小心。
建议在子域名上接收传入电子邮件，例如 `incoming.example.com`，以防止与运行在 `example.com` 上的其他服务可能存在的安全问题。
