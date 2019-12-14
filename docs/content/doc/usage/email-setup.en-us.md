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

- To use Gitea's built-in Email support, update the `app.ini` config file [mailer] section:

```ini
[mailer]
ENABLED = true
HOST    = mail.mydomain.com:587
FROM    = gitea@mydomain.com
USER    = gitea@mydomain.com
PASSWD  = `password`
```

- Restart Gitea for the configuration changes to take effect.

- To send a test email to validate the settings, go to Gitea > Site Administration > Configuration > SMTP Mailer Configuration.

For the full list of options check the [Config Cheat Sheet]({{< relref "doc/advanced/config-cheat-sheet.en-us.md" >}})