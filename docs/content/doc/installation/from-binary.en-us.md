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

All downloads come with SQLite, MySQL and PostgreSQL support, and are built with embedded assets. Keep in mind that this can be different for older releases. The installation based on our binaries is quite simple, just choose the file matching your platform from the [downloads page](https://dl.gitea.io/gitea), copy the URL and replace the URL within the commands below:

```
wget -O gitea https://dl.gitea.io/gitea/1.3.2/gitea-1.3.2-linux-amd64
chmod +x gitea
```

## Test

After following the steps above you will have a `gitea` binary within your working directory, first you can test it if it works like expected and afterwards you can copy it to the destination where you want to store it. When you launch Gitea manually from your CLI you can always kill it by hitting `Ctrl + C`.

```
./gitea web
```

## Troubleshooting

### Old glibc versions

Older Linux distributions (such as Debian 7 and CentOS 6) may not be able to load the Gitea binary, usually resulting an error like ```./gitea: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.14' not found (required by ./gitea)```. This is due to the integrated SQLite support in the binaries we provide. In the future, we will provide binaries without the requirement for glibc. As a workaround, you can upgrade your distribution or [install from source]({{< relref "from-source.en-us.md" >}}).

### Running gitea on another port

If getting an error like `702 runWeb()] [E] Failed to start server: listen tcp 0.0.0.0:3000: bind: address already in use` gitea needs to be started on another free port. This is possible using `./gitea web -p $PORT`.

## Anything missing?

Are we missing anything on this page? Then feel free to reach out to us on our [Discord server](https://discord.gg/NsatcWJ), there you will get answers to any question pretty fast.
