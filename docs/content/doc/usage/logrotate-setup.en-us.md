---
date: "2020-01-19"
title: "Usage: Logrotate setup"
slug: "logrotate-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Logrotate setup"
    weight: 12
    identifier: "logrotate-setup"
---

## `logrotate` Setup

The `logrotate` utility automates log rotation. This is especially useful for larger Gitea instance with many users and repositories, where Gitea logs can grow quickly.

To use `logrotate`, install the package first. Make sure to set `LOG_ROTATE` and `DAILY_ROTATE` on your `app.ini` to `false`, as `logrotate` will do the job instead of built-in rotation. See also [Config Cheat Sheet](https://docs.gitea.io/en-us/config-cheat-sheet/#file-log-mode-log-file-log-file-or-mode-file).

Then copy [sample configuration](https://github.com/go-gitea/gitea/blob/master/contrib/logrotate/gitea.conf) to `/etc/logrotate.conf.d` and edit it to fit your Gitea instance.

As root, test the configuration by:

```
logrotate /etc/logrotate.conf --debug
```

Fix any errors if found.

Please note that `logrotate` doesn't have restart command as its next rotation job includes any new configurations.
