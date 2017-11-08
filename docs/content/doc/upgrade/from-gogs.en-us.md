---
date: "2016-12-01T16:00:00+02:00"
title: "Upgrade from Gogs"
slug: "upgrade-from-gogs"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "upgrade"
    name: "From Gogs"
    weight: 10
    identifier: "upgrade-from-gogs"
---

# Upgrade from Gogs

Gogs versions up to 0.9.146 (db schema version 15) can be smoothly upgraded to Gitea.

There are some steps to do so below. On Unix run as your Gogs user:

* Create a Gogs backup with `gogs dump`. This creates `gogs-dump-[timestamp].zip` file containing all your Gogs data. 
* Download the file matching your platform from the [downloads page](https://dl.gitea.io/gitea).
* Put the binary at the desired install location.
* Copy `gogs/custom/conf/app.ini` to `gitea/custom/conf/app.ini`.
* If you have custom `templates, public` in `gogs/custom/` copy them to `gitea/custom/`.
* If you have any other custom folders like `gitignore, label, license, locale, readme` in `gogs/custom/conf` copy them to `gitea/custom/options`.
* Copy `gogs/data/` to `gitea/data/`. It contains issue attachments and avatars.
* Verify by starting Gitea with `gitea web`.
* Enter Gitea admin panel on the UI, run `Rewrite '.ssh/authorized_keys' file`, then run `Rewrite all update hook of repositories` (needed when custom config path is changed).

### Change gogs specific information:

* Rename `gogs-repositories/` to `gitea-repositories/`
* Rename `gogs-data/` to `gitea-data/`
* In your `gitea/custom/conf/app.ini` change:

FROM:
```
[database]
PATH = /home/:USER/gogs/data/:DATABASE.db
[attachment]
PATH = /home/:USER/gogs-data/attachments
[picture]
AVATAR_UPLOAD_PATH = /home/:USER/gogs-data/avatars
[log]
ROOT_PATH = /home/:USER/gogs/log
```

TO:
```
[database]
PATH = /home/:USER/gitea/data/:DATABASE.db
[attachment]
PATH = /home/:USER/gitea-data/attachments
[picture]
AVATAR_UPLOAD_PATH = /home/:USER/gitea-data/avatars
[log]
ROOT_PATH = /home/:USER/gitea/log
```

* Verify by starting Gitea with `gitea web`

### Troubleshooting

* If you encounter errors relating to custom templates in the `gitea/custom/templates` folder, try moving the templates causing the errors away one by one. They may not be compatible with Gitea.

### Add Gitea to startup on Unix

Update the appropriate file from [gitea/contrib](https://github.com/go-gitea/gitea/tree/master/contrib) with the right environment variables.

For distro's with systemd:

* Copy the updated script to `/etc/systemd/system/gitea.service`
* Add the service to the startup with: `sudo systemctl enable gitea`
* Disable old gogs startup script: `sudo systemctl disable gogs`

For distro's with SysVinit:

* Copy the updated script to `/etc/init.d/gitea`
* Add the service to the startup with: `sudo rc-update add gitea`
* Disable old gogs startup script: `sudo rc-update del gogs`
