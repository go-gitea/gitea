---
date: "2017-01-01T16:00:00+02:00"
title: "Backup and Restore"
slug: "backup-and-restore"
sidebar_position: 11
toc: false
draft: false
aliases:
  - /en-us/backup-and-restore
menu:
  sidebar:
    parent: "administration"
    name: "Backup and Restore"
    sidebar_position: 11
    identifier: "backup-and-restore"
---

# Backup and Restore

Gitea currently has a `dump` command that will save the installation to a ZIP file. This
file can be unpacked and used to restore an instance.

## Backup Consistency

To ensure the consistency of the Gitea instance, it must be shutdown during backup.

Gitea consists of a database, files and git repositories, all of which change when it is used. For instance, when a migration is in progress, a transaction is created in the database while the git repository is being copied over. If the backup happens in the middle of the migration, the git repository may be incomplete although the database claims otherwise because it was dumped afterwards. The only way to avoid such race conditions is by stopping the Gitea instance during the backups.

## Backup Command (`dump`)

Switch to the user running Gitea: `su git`. Run `./gitea dump -c /path/to/app.ini` in the Gitea installation
directory. There should be some output similar to the following:

```none
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

Inside the `gitea-dump-1482906742.zip` file, will be the following:

- `app.ini` - Optional copy of configuration file if originally stored outside the default `custom/` directory
- `custom/` - All config or customization files in `custom/`.
- `data/` - Data directory (APP_DATA_PATH), except sessions if you are using file session. This directory includes `attachments`, `avatars`, `lfs`, `indexers`, SQLite file if you are using SQLite.
- `repos/` - Complete copy of the repository directory.
- `gitea-db.sql` - SQL dump of database
- `log/` - Various logs. They are not needed for a recovery or migration.

Intermediate backup files are created in a temporary directory specified either with the
`--tempdir` command-line parameter or the `TMPDIR` environment variable.

## Backup the database

The SQL dump created by `gitea dump` uses XORM and Gitea admins may prefer to use the native the MySQL and PostgreSQL dump tools instead. There are still open issues when using XORM for dumping the database that may cause problems when attempting to restore it.

```sh
# mysql
mysqldump -u$USER -p$PASS --database $DATABASE > gitea-db.sql
# postgres
pg_dump -U $USER $DATABASE > gitea-db.sql
```

### Using Docker (`dump`)

There are a few caveats for using the `dump` command with Docker.

The command has to be executed with the `RUN_USER = <OS_USERNAME>` specified in `gitea/conf/app.ini`; and, for the zipping of the backup folder to occur without permission error the command `docker exec` must be executed inside of the `--tempdir`.

Example:

```none
docker exec -u <OS_USERNAME> -it -w <--tempdir> $(docker ps -qf 'name=^<NAME_OF_DOCKER_CONTAINER>$') bash -c '/usr/local/bin/gitea dump -c </path/to/app.ini>'
```

\*Note: `--tempdir` refers to the temporary directory of the docker environment used by Gitea; if you have not specified a custom `--tempdir`, then Gitea uses `/tmp` or the `TMPDIR` environment variable of the docker container. For `--tempdir` adjust your `docker exec` command options accordingly.

The result should be a file, stored in the `--tempdir` specified, along the lines of: `gitea-dump-1482906742.zip`

## Restore Command (`restore`)

There is currently no support for a recovery command. It is a manual process that mostly
involves moving files to their correct locations and restoring a database dump.

Example:

```sh
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
mv app.ini /etc/gitea/conf/app.ini
mv data/* /var/lib/gitea/data/
mv log/* /var/lib/gitea/log/
mv repos/* /var/lib/gitea/gitea-repositories/
chown -R gitea:gitea /etc/gitea/conf/app.ini /var/lib/gitea

# mysql
mysql --default-character-set=utf8mb4 -u$USER -p$PASS $DATABASE <gitea-db.sql
# sqlite3
sqlite3 $DATABASE_PATH <gitea-db.sql
# postgres
psql -U $USER -d $DATABASE < gitea-db.sql

service gitea restart
```

Repository Git Hooks should be regenerated if installation method is changed (eg. binary -> Docker), or if Gitea is installed to a different directory than the previous installation.

With Gitea running, and from the directory Gitea's binary is located, execute: `./gitea admin regenerate hooks`

This ensures that application and configuration file paths in repository Git Hooks are consistent and applicable to the current installation. If these paths are not updated, repository `push` actions will fail.

### Using Docker (`restore`)

There is also no support for a recovery command in a Docker-based gitea instance. The restore process contains the same steps as described in the previous section but with different paths.

Example:

```sh
# open bash session in container
docker exec --user git -it 2a83b293548e bash
# unzip your backup file within the container
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
# restore the gitea data
mv data/* /data/gitea
# restore the repositories itself
mv repos/* /data/git/gitea-repositories/
# adjust file permissions
chown -R git:git /data
# Regenerate Git Hooks
/usr/local/bin/gitea -c '/data/gitea/conf/app.ini' admin regenerate hooks
```

The default user in the gitea container is `git` (1000:1000). Please replace `2a83b293548e` with your gitea container id or name.

### Using Docker-rootless (`restore`)

The restore workflow in Docker-rootless containers differs only in the directories to be used:

```sh
# open bash session in container
docker exec --user git -it 2a83b293548e bash
# unzip your backup file within the container
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
# restore the app.ini
mv data/conf/app.ini /etc/gitea/app.ini
# restore the gitea data
mv data/* /var/lib/gitea
# restore the repositories itself
mv repos/* /var/lib/gitea/git/gitea-repositories
# adjust file permissions
chown -R git:git /etc/gitea/app.ini /var/lib/gitea
# Regenerate Git Hooks
/usr/local/bin/gitea -c '/etc/gitea/app.ini' admin regenerate hooks
```
