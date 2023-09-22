---
date: "2016-12-01T16:00:00+02:00"
title: "Upgrade from Gogs"
slug: "upgrade-from-gogs"
sidebar_position: 101
toc: false
draft: false
aliases:
  - /en-us/upgrade-from-gogs
menu:
  sidebar:
    parent: "installation"
    name: "Upgrade From Gogs"
    sidebar_position: 101
    identifier: "upgrade-from-gogs"
---

# Upgrade from Gogs

Gogs, version 0.9.146 and older, can be easily migrated to Gitea.

There are some basic steps to follow. On a Linux system run as the Gogs user:

* Create a Gogs backup with `gogs backup`. This creates `gogs-backup-[timestamp].zip` file
  containing all important Gogs data. You would need it if you wanted to move to the `gogs` back later.
* Download the file matching the destination platform from the [downloads page](https://dl.gitea.com/gitea/).
 It should be `1.0.x` version. Migrating from `gogs` to any other version is impossible.
* Put the binary at the desired install location.
* Copy `gogs/custom/conf/app.ini` to `gitea/custom/conf/app.ini`.
* Copy custom `templates, public` from `gogs/custom/` to `gitea/custom/`.
* For any other custom folders, such as `gitignore, label, license, locale, readme` in
  `gogs/custom/conf`, copy them to `gitea/custom/options`.
* Copy `gogs/data/` to `gitea/data/`. It contains issue attachments and avatars.
* Verify by starting Gitea with `gitea web`.
* Enter Gitea admin panel on the UI, run `Rewrite '.ssh/authorized_keys' file`.
* Launch every major version of the binary ( `1.1.4` → `1.2.3` → `1.3.4` → `1.4.2` →  etc ) to migrate database.
* If custom or config path was changed, run `Rewrite all update hook of repositories`.

## Change gogs specific information

* Rename `gogs-repositories/` to `gitea-repositories/`
* Rename `gogs-data/` to `gitea-data/`
* In `gitea/custom/conf/app.ini` change:

  FROM:

  ```ini
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

  ```ini
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

## Upgrading to most recent `gitea` version

After successful migration from `gogs` to `gitea 1.0.x`, it is possible to upgrade `gitea` to a modern version
in a two steps process.

Upgrade to [`gitea 1.6.4`](https://dl.gitea.com/gitea/1.6.4/) first. Download the file matching
the destination platform from the [downloads page](https://dl.gitea.com/gitea/1.6.4/) and replace the binary.
Run Gitea at least once and check that everything works as expected.

Then repeat the procedure, but this time using the [latest release](https://dl.gitea.com/gitea/@version@/).

## Upgrading from a more recent version of Gogs

Upgrading from a more recent version of Gogs (up to `0.11.x`) may also be possible, but will require a bit more work.
See [#4286](https://github.com/go-gitea/gitea/issues/4286), which includes various Gogs `0.11.x` versions.

Upgrading from Gogs `0.12.x` and above will be increasingly more difficult as the projects diverge further apart in configuration and schema.

## Troubleshooting

* If errors are encountered relating to custom templates in the `gitea/custom/templates`
  folder, try moving the templates causing the errors away one by one. They may not be
  compatible with Gitea or an update.

## Add Gitea to startup on Unix

Update the appropriate file from [gitea/contrib](https://github.com/go-gitea/gitea/tree/main/contrib)
with the right environment variables.

For distros with systemd:

* Copy the updated script to `/etc/systemd/system/gitea.service`
* Add the service to the startup with: `sudo systemctl enable gitea`
* Disable old gogs startup script: `sudo systemctl disable gogs`

For distros with SysVinit:

* Copy the updated script to `/etc/init.d/gitea`
* Add the service to the startup with: `sudo rc-update add gitea`
* Disable old gogs startup script: `sudo rc-update del gogs`
