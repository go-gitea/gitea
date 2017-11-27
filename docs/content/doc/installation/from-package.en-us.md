---
date: "2016-12-01T16:00:00+02:00"
title: "Installation from package"
slug: "install-from-package"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From package"
    weight: 20
    identifier: "install-from-package"
---

# Installation from package

## Linux

We have not published any real package yet, we will update this page directly when we start to publish packages for any Linux distribution, in the meantime you should follow our [installation from binary]({{< relref "from-binary.en-us.md" >}}) guide.

## Windows

We have not published any package for Windows yet, we will update this page directly when we start to publish packages in the form of `MSI` installers or via [Chocolatey](https://chocolatey.org/), in the meantime you should follow our [installation from binary]({{< relref "from-binary.en-us.md" >}}) guide.

## macOS

Currently we only support the installation via `brew` for macOS. If you are not using [Homebrew](http://brew.sh/) you should follow our [installation from binary]({{< relref "from-binary.en-us.md" >}}) guide. To install Gitea via `brew` you just need to execute the following commands:

```
brew tap go-gitea/gitea
brew install gitea
```

## FreeBSD

A FreeBSD port `www/gitea` is available.  You can install a pre-built binary package:

```
pkg install gitea
```

For the most up to date version, or to build the port with custom options, [install it from the port](https://www.freebsd.org/doc/handbook/ports-using.html):

```
su -
cd /usr/ports/www/gitea
make install clean
```

The port uses the standard FreeBSD file system layout: config files are in `/usr/local/etc/gitea`, bundled templates, options, plugins and themes are in `/usr/local/share/gitea`, and a start script is in `/usr/local/etc/rc.d/gitea`.

To enable Gitea to run as a service, run `sysrc gitea_enable=YES` and start it with `service gitea start`. 

## Anything missing?

Are we missing anything on this page? Then feel free to reach out to us on our [Discord server](https://discord.gg/NsatcWJ), there you will get answers to any question pretty fast.
