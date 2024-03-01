---
date: "2021-09-02T16:00:00+08:00"
title: "Upgrade from an old Gitea"
slug: "upgrade-from-gitea"
sidebar_position: 100
toc: false
draft: false
aliases:
  - /en-us/upgrade-from-gitea
menu:
  sidebar:
    parent: "installation"
    name: "Upgrade From Old Gitea"
    sidebar_position: 100
    identifier: "upgrade-from-gitea"
---

# Upgrade from an old Gitea

Follow below steps to ensure a smooth upgrade to a new Gitea version.

## Check the Changelog for breaking changes

To make Gitea better, some breaking changes are unavoidable, especially for big milestone releases.
Before upgrading, please read the [Changelog on Gitea blog](https://blog.gitea.com/)
and check whether the breaking changes affect your Gitea instance.

## Verify there are no deprecated configuration options

New versions of Gitea often come with changed configuration syntax or options which are usually displayed for
at least one release cycle inside at the top of the Site Administration panel. If these warnings are not
resolved, Gitea may refuse to start in the following version.

## Backup for downgrade

Gitea keeps compatibility for patch versions whose first two fields are the same (`a.b.x` -> `a.b.y`),
these patch versions can be upgraded and downgraded with the same database structure.
Otherwise (`a.b.?` -> `a.c.?`), a newer Gitea version will upgrade the old database
to a new structure that may differ from the old version.

For example:

| From | To | Result |
| --- | --- | --- |
| 1.4.0 | 1.4.1 | ✅ |
| 1.4.1 | 1.4.0 | ⚠️ Not recommended, take your own risk! Although it may work if the database structure doesn't change, it's highly recommended to use a backup to downgrade. |
| 1.4.x | 1.5.y | ✅ Database gets upgraded. You can upgrade from 1.4.x to the latest 1.5.y directly. |
| 1.5.y | 1.4.x | ❌ Database already got upgraded and can not be used for an old Gitea, use a backup to downgrade. |

**Since you can not run an old Gitea with an upgraded database,
a backup should always be made before a database upgrade.**

If you use Gitea in production, it's always highly recommended to make a backup before upgrade,
even if the upgrade is between patch versions.

Backup steps:

* Stop Gitea instance
* Backup database
* Backup Gitea config
* Backup Gitea data files in `APP_DATA_PATH`
* Backup Gitea external storage (eg: S3/MinIO or other storages if used)

If you are using cloud services or filesystems with snapshot feature,
a snapshot for the Gitea data volume and related object storage is more convenient.

After all of steps have been prepared, download the new version, stop the application, perform a backup and
then start the new application. On each startup, Gitea verifies that the database is up to date and will automatically
perform any necessary migrations. Depending on the size of the database, this can take some additional time on the
first launch during which the application will be unavailable.

## Upgrade with Docker

* `docker pull` the latest Gitea release.
* Stop the running instance, backup data.
* Use `docker` or `docker-compose` to start the newer Gitea Docker container.

## Upgrade from package

* Stop the running instance, backup data.
* Use your package manager to upgrade Gitea to the latest version.
* Start the Gitea instance.

## Upgrade from binary

* Download the latest Gitea binary to a temporary directory.
* Stop the running instance, backup data.
* Replace the installed Gitea binary with the downloaded one.
* Start the Gitea instance.

A script automating these steps for a deployment on Linux can be found at [`contrib/upgrade.sh` in Gitea's source tree](https://github.com/go-gitea/gitea/blob/main/contrib/upgrade.sh).

## Take care about customized templates

Gitea's template structure and variables may change between releases, if you are using customized templates,
do pay attention if your templates are compatible with the Gitea you are using.

If the customized templates don't match Gitea version, you may experience:
`50x` server error, page components missing or malfunctioning, strange page layout, ...
Remove or update the incompatible templates and Gitea web will work again.
