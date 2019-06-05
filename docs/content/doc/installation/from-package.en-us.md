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

{{% h1 %}}Installation from package{{% /h1 %}}

{{% h2 %}}Debian{{% /h2 %}}

Although there is a package of Gitea in Debian's [contrib](https://wiki.debian.org/SourcesList),
it is not supported directly by us.

Unfortunately, the package is not maintained anymore and broken because of missing sources.
Please follow the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide instead.

Should the packages get updated and fixed, we will provide up-to-date installation instructions here.

{{% h2 %}}Windows{{% /h2 %}}

There are no published packages for Windows. This page will change when packages are published,
in the form of `MSI` installers or via [Chocolatey](https://chocolatey.org/). In the meantime
the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide.

{{% h2 %}}macOS{{% /h2 %}}

Currently, the only supported method of installation on MacOS is [Homebrew](http://brew.sh/).
Following the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide may work,
but is not supported. To install Gitea via `brew`:

```
brew tap go-gitea/gitea
brew install gitea
```

{{% h2 %}}FreeBSD{{% /h2 %}}

A FreeBSD port `www/gitea` is available. To install the pre-built binary package:

```
pkg install gitea
```

For the most up to date version, or to build the port with custom options,
[install it from the port](https://www.freebsd.org/doc/handbook/ports-using.html):

```
su -
cd /usr/ports/www/gitea
make install clean
```

The port uses the standard FreeBSD file system layout: config files are in `/usr/local/etc/gitea`,
bundled templates, options, plugins and themes are in `/usr/local/share/gitea`, and a start script
is in `/usr/local/etc/rc.d/gitea`.

To enable Gitea to run as a service, run `sysrc gitea_enable=YES` and start it with `service gitea start`.

{{% h2 %}}Cloudron{{% /h2 %}}

Gitea is available as a 1-click install on [Cloudron](https://cloudron.io). For those unaware,
Cloudron makes it easy to run apps like Gitea on your server and keep them up-to-date and secure.

[![Install](https://cloudron.io/img/button.svg)](https://cloudron.io/button.html?app=io.gitea.cloudronapp)

The Gitea package is maintained [here](https://git.cloudron.io/cloudron/gitea-app).

There is a [demo instance](https://my-demo.cloudron.me) (username: cloudron password: cloudron) where
you can experiment with running Gitea.

