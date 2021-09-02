---
date: "2021-09-02T16:00:00+08:00"
title: "Upgrade from an old Gitea"
slug: "upgrade-from-gitea"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "upgrade"
    name: "From Gitea"
    weight: 10
    identifier: "upgrade-from-gitea"
---

# Upgrade from an old Gitea

**Table of Contents**

{{< toc >}}

Gitea provides automatically upgrade mechanism. Just get a new Gitea, stop the old one, run the new one.
Everytime a Gitea instance runs, it checks whether an upgrade action should be taken. 
If an upgrade action is required, Gitea will take some time to complete the upgrade and then serve.

## Backup for downgrade

Gitea keeps compatibility for versions whose first two fields are the same (`a.b.x` -> `a.b.y`), 
these versions can be upgraded and downgraded with the same database structure. 
Otherwise (`a.b.?` -> `a.c.?`), a new Gitea will upgrade an old database 
to a new structure which may not be used by an old Gitea.

For example:

| From | To |  |
| --- | --- | --- |
| 1.4.0 | 1.4.1 | ✅ |
| 1.4.1 | 1.4.0 | ✅ |
| 1.4.1 | 1.5.0 | ✅ Database gets upgraded |
| 1.5.0 | 1.4.1 | ❌ Database already got upgraded and can not be used for an old Gitea |

**Since you can not run an old Gitea with an upgraded database, 
if stability is the top priority and you want to make sure there is a chance to downgrade,
a backup should always be made before upgrade.** 

Backup steps:

* Stop Gitea instance
* Backup database (important)
* Backup Gitea config (optional)
* Backup Gitea data files in `APP_DATA_PATH` (optional)

`optional` means that these data seldom have compatibility problems between different versions unless specially mentioned. 

If you are using cloud services or filesystems with snapshot feature,
a snapshot for the Gitea data volume is more convenient.


## Upgrade with Docker

* `docker pull` the latest Gitea release.
* Stop the running instance, backup data.
* Use `docker` or `docker-compose` to start the Gitea Docker instance.

## Upgrade from package

* Stop the running instance, backup data.
* Use package manager to upgrade Gitea to the latest version.
* Start the Gitea instance.

## Upgrade from binary

* Download the latest Gitea binary to a temporary directory.
* Stop the running instance, backup data.
* Replace the installed Gitea binary with the downloaded one. 
* Start the Gitea instance.

## Take care about customized templates

Gitea's template structure and variables may change between releases, if you are using customized templates, 
do pay attention if your templates are compatible with the Gitea you are using. 

If the customized templates don't match Gitea version, you may experience: 
50x server error, page components missing or misfunction, strange page layout, etc. 
Remove the incompatible templates and Gitea web will work again.
