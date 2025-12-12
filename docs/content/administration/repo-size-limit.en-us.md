---
date: "2023-05-24T13:00:00+00:00"
title: "Per repository size limit"
slug: "repo-size-limit"
weight: 12
toc: false
draft: false
aliases:
  - /en-us/repo-size-limit
menu:
  sidebar:
    parent: "administration"
    name: "Per repository size limit"
    weight: 12
    identifier: "repo-size-limit"
---

# Gitea per repository size limit setup

To use Gitea's experimental built-in per repository size limit support, Administrator must update the `app.ini` file:

```ini
;; Enable applying a global size limit defined by REPO_SIZE_LIMIT. Each repository can have a value that overrides the global limit
;; "false" means no limit will be enforced, even if specified on a repository
ENABLE_SIZE_LIMIT = true

;; Specify a global repository size limit in bytes to apply for each repository. 0 - No limit
;; If repository has it's own limit set in UI it will override the global setting
;; Standard units of measurements for size can be used like B, KB, KiB, ... , EB, EiB, ...
REPO_SIZE_LIMIT = 500 MB

This setting is persistent.

The size limitation is triggered when repository `disk size` + `new commit size` > `defined repository size limit`

If size limitation is triggered the feature would prevent commits that increase repository size on disk
of gitea server and allow those that decrease it

# Gitea per repository size limit setup in UI

1. For Gitea admin it is possible during runtime to enable/disable limit size feature, change the global size limit on the fly.
**This setting is not persistent across restarts**

`Admin panel/Site settings` -> `Repository management`

Persistance can be achieved if the limit is maintained by editing `app.ini` file

2. The individually set per repository limit in `Settings` of the
repository would take precedence over global limit when the size limit
feature is enabled. Only admin can modify those limits

**Note**: Size checking for large repositories is time consuming operation so time of push under size limit might increase up to a minute depending on your server hardware
