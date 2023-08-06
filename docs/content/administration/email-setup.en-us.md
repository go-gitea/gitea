---
date: "2019-10-15T10:10:00+05:00"
title: "Email setup"
slug: "email-setup"
sidebar_position: 12
toc: false
draft: false
aliases:
  - /en-us/email-setup
menu:
  sidebar:
    parent: "administration"
    name: "Email setup"
    sidebar_position: 12
    identifier: "email-setup"
---

# Email setup

Gitea has mailer functionality for sending transactional emails (such as registration confirmation). It can be configured to either use Sendmail (or compatible MTAs like Postfix and msmtp) or directly use SMTP server.

## Using Sendmail

Use `sendmail` command as mailer.

Note: For use in the official Gitea Docker image, please configure with the SMTP version (see the following section).

Note: For Internet-facing sites consult documentation of your MTA for instructions to send emails over TLS. Also set up SPF, DMARC, and DKIM DNS records to make emails sent be accepted as legitimate by various email providers.

```ini
[mailer]
ENABLED       = true
FROM          = gitea@mydomain.com
PROTOCOL      = sendmail
SENDMAIL_PATH = /usr/sbin/sendmail
SENDMAIL_ARGS = "--" ; most "sendmail" programs take options, "--" will prevent an email address being interpreted as an option.
```

## Using SMTP

Directly use SMTP server as relay. This option is useful if you don't want to set up MTA on your instance but you have an account at email provider.

```ini
[mailer]
ENABLED        = true
FROM           = gitea@mydomain.com
PROTOCOL       = smtps
SMTP_ADDR      = mail.mydomain.com
SMTP_PORT      = 587
USER           = gitea@mydomain.com
PASSWD         = `password`
```

Restart Gitea for the configuration changes to take effect.

To send a test email to validate the settings, go to Gitea > Site Administration > Configuration > SMTP Mailer Configuration.

For the full list of options check the [Config Cheat Sheet](administration/config-cheat-sheet.md)

Please note: authentication is only supported when the SMTP server communication is encrypted with TLS or `HOST=localhost`. TLS encryption can be through:

- STARTTLS (also known as Opportunistic TLS) via port 587. Initial connection is done over cleartext, but then be upgraded over TLS if the server supports it.
- SMTPS connection (SMTP over TLS) via the default port 465. Connection to the server use TLS from the beginning.
- Forced SMTPS connection with `IS_TLS_ENABLED=true`. (These are both known as Implicit TLS.)
This is due to protections imposed by the Go internal libraries against STRIPTLS attacks.

Note that Implicit TLS is recommended by [RFC8314](https://tools.ietf.org/html/rfc8314#section-3) since 2018.

### Gmail

The following configuration should work with GMail's SMTP server:

```ini
[mailer]
ENABLED        = true
HOST           = smtp.gmail.com:465 ; Remove this line for Gitea >= 1.18.0
SMTP_ADDR      = smtp.gmail.com
SMTP_PORT      = 465
FROM           = example.user@gmail.com
USER           = example.user
PASSWD         = `***`
PROTOCOL       = smtps ; Gitea >= 1.19.0
; PROTOCOL       = smtp  ; Gitea < 1.19.0
; IS_TLS_ENABLED = true  ; Gitea < 1.19.0
```

Note that you'll need to create and use an [App password](https://support.google.com/accounts/answer/185833?hl=en) by enabling 2FA on your Google
account. You won't be able to use your Google account password directly.
