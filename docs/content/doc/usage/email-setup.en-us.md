---
date: "2019-10-15T10:10:00+05:00"
title: "Usage: Email setup"
slug: "email-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Email setup"
    weight: 12
    identifier: "email-setup"
---

# Email setup

To use Gitea's built-in Email support, update the `app.ini` config file [mailer] section:

## Sendmail version
Use the operating system’s sendmail command instead of SMTP. This is common on Linux servers.  
Note: For use in the official Gitea Docker image, please configure with the SMTP version.
```ini
[mailer]
ENABLED       = true
FROM          = gitea@mydomain.com
MAILER_TYPE   = sendmail
SENDMAIL_PATH = /usr/sbin/sendmail
```

## SMTP version
```ini
[mailer]
ENABLED        = true
FROM           = gitea@mydomain.com
MAILER_TYPE    = smtp
HOST           = mail.mydomain.com:587
IS_TLS_ENABLED = true
USER           = gitea@mydomain.com
PASSWD         = `password`
```

- Restart Gitea for the configuration changes to take effect.

- To send a test email to validate the settings, go to Gitea > Site Administration > Configuration > SMTP Mailer Configuration.

For the full list of options check the [Config Cheat Sheet]({{< relref "doc/advanced/config-cheat-sheet.en-us.md" >}})

- Please note: authentication is only supported when the SMTP server communication is encrypted with TLS or `HOST=localhost`. TLS encryption can be through:
  - Via the server supporting TLS through STARTTLS - usually provided on port 587. (Also known as Opportunistic TLS.)
  - SMTPS connection (SMTP over transport layer security) via the default port 465.
  - Forced SMTPS connection with `IS_TLS_ENABLED=true`. (These are both known as Implicit TLS.)
- This is due to protections imposed by the Go internal libraries against STRIPTLS attacks.

### Gmail

The following configuration should work with GMail's SMTP server:

```ini
[mailer]
ENABLED        = true
HOST           = smtp.gmail.com:465
FROM           = example@gmail.com
USER           = example@gmail.com
PASSWD         = ***
MAILER_TYPE    = smtp
IS_TLS_ENABLED = true
HELO_HOSTNAME  = example.com
```
