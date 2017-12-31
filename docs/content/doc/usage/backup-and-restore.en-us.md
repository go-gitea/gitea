---
date: "2017-01-01T16:00:00+02:00"
title: "Usage: Backup and Restore"
slug: "backup-and-restore"
weight: 11
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Backup and Restore"
    weight: 11
    identifier: "backup-and-restore"
---

# Backup and Restore

Gitea currently has a `dump` command that will save your installation to a zip file. There will be a `restore` command implemented at some point in the future. You will be able to use this to back up your installation, as well as make migrating servers easier.

## Backup Command (`dump`)

First, switch to the user running gitea: `su git` (or whatever user you are using). Run `./gitea dump` in the gitea installation directory. You should see some output similar to this:

```
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

Inside the `gitea-dump-1482906742.zip` file, you will find the following:

* `custom/conf/app.ini` - This is your server config.
* `gitea-db.sql` - SQL dump of your database.
* `gitea-repo.zip` - This zip will be a complete copy of your repo folder.
   See Config -> repository -> `ROOT` for the location.
* `log/` - this will contain various logs. You don't need these if you are doing
   a migration.

Intermediate backup files are created in a temporary directory specified either with the `--tempdir` command-line parameter or the `TMPDIR` environment variable.

## Restore Command (`restore`)

WIP: Does not exist yet.
