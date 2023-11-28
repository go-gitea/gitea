---
date: "2023-05-23T09:00:00+08:00"
title: "Email 设置"
slug: "email-setup"
sidebar_position: 12
toc: false
draft: false
aliases:
  - /zh-cn/email-setup
menu:
  sidebar:
    parent: "administration"
    name: "Email 设置"
    sidebar_position: 12
    identifier: "email-setup"
---

# Email 设置

Gitea 具有邮件功能，用于发送事务性邮件（例如注册确认邮件）。它可以配置为使用 Sendmail（或兼容的 MTA，例如 Postfix 和 msmtp）或直接使用 SMTP 服务器。

## 使用 Sendmail

使用 `sendmail` 命令作为邮件传输代理（mailer）。

注意：对于在官方Gitea Docker镜像中使用，请使用SMTP版本进行配置（请参考下一节）。

注意：对于面向互联网的网站，请查阅您的 MTA 文档以了解通过TLS发送邮件的说明。同时设置 SPF、DMARC 和 DKIM DNS 记录，以使发送的邮件被各个电子邮件提供商接受为合法邮件。

```ini
[mailer]
ENABLED       = true
FROM          = gitea@mydomain.com
PROTOCOL   = sendmail
SENDMAIL_PATH = /usr/sbin/sendmail
SENDMAIL_ARGS = "--" ; 大多数 "sendmail" 程序都接受选项，使用 "--" 将防止电子邮件地址被解释为选项。
```

## 使用 SMTP

直接使用 SMTP 服务器作为中继。如果您不想在实例上设置 MTA，但在电子邮件提供商那里有一个帐户，这个选项非常有用。

```ini
[mailer]
ENABLED        = true
FROM           = gitea@mydomain.com
PROTOCOL    = smtps
SMTP_ADDR      = mail.mydomain.com
SMTP_PORT      = 587
USER           = gitea@mydomain.com
PASSWD         = `password`
```

重启 Gitea 以使配置更改生效。

要发送测试邮件以验证设置，请转到 Gitea > 站点管理 > 配置 > SMTP 邮件配置。

有关所有选项的完整列表，请查看[配置速查表](administration/config-cheat-sheet.md)。

请注意：只有在使用 TLS 或 `HOST=localhost` 加密 SMTP 服务器通信时才支持身份验证。TLS 加密可以通过以下方式进行：

- 通过端口 587 的 STARTTLS（也称为 Opportunistic TLS）。初始连接是明文的，但如果服务器支持，则可以升级为 TLS。
- 通过默认端口 465 的 SMTPS 连接。连接到服务器从一开始就使用 TLS。
- 使用 `IS_TLS_ENABLED=true` 进行强制的 SMTPS 连接。（这两种方式都被称为 Implicit TLS）
这是由于 Go 内部库对 STRIPTLS 攻击的保护机制。

请注意，自2018年起，[RFC8314](https://tools.ietf.org/html/rfc8314#section-3) 推荐使用 Implicit TLS。

### Gmail

以下配置应该适用于 Gmail 的 SMTP 服务器：

```ini
[mailer]
ENABLED        = true
HOST           = smtp.gmail.com:465 ; 对于 Gitea >= 1.18.0，删除此行
SMTP_ADDR      = smtp.gmail.com
SMTP_PORT      = 465
FROM           = example.user@gmail.com
USER           = example.user
PASSWD         = `***`
PROTOCOL    = smtps
```

请注意，您需要创建并使用一个 [应用密码](https://support.google.com/accounts/answer/185833?hl=en) 并在您的 Google 帐户上启用 2FA。您将无法直接使用您的 Google 帐户密码。
