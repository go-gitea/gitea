---
date: "2017-06-19T12:00:00+02:00"
title: "Installation from binary"
slug: "install-from-binary"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From binary"
    weight: 20
    identifier: "install-from-binary"
---

# Installation from binary

All downloads come with SQLite, MySQL and PostgreSQL support, and are built with
embedded assets. This can be different for older releases. Choose the file matching
the destination platform from the [downloads page](https://dl.gitea.io/gitea), copy
the URL and replace the URL within the commands below:

```
wget -O gitea https://dl.gitea.io/gitea/1.3.2/gitea-1.3.2-linux-amd64
chmod +x gitea
```

## Test

After getting a binary, it can be tested with `./gitea web` or moved to a permanent
location. When launched manually, Gitea can be killed using `Ctrl+C`.

```
./gitea web
```

## Troubleshooting

### Old glibc versions

Older Linux distributions (such as Debian 7 and CentOS 6) may not be able to load the
Gitea binary, usually producing an error such as ```./gitea: /lib/x86_64-linux-gnu/libc.so.6:
version `GLIBC\_2.14' not found (required by ./gitea)```. This is due to the integrated
SQLite support in the binaries provided by dl.gitea.io. In this situation, it is usually
possible to [install from source]({{< relref "from-source.en-us.md" >}}) without sqlite
support.

### Running gitea on another port

For errors like `702 runWeb()] [E] Failed to start server: listen tcp 0.0.0.0:3000:
bind: address already in use` gitea needs to be started on another free port. This
is possible using `./gitea web -p $PORT`. It's possible another instance of gitea
is already running.
