---
date: "2020-02-02"
title: "Set up Git"
slug: "git-set-up"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Set up Git"
    weight: 10
    identifier: "git-set-up"
---

At the heart of Gitea is [Git](https://git-scm.com), an open-source, distributed version control system (DVCS). Before you can use Gitea, you'll need to install Git to your Gitea instance.

## From binary package

### Windows

The official Git package for Windows can be found on [Git Downloads page](https://git-scm.com/download/win), maintained by [Git for Windows project](https://gitforwindows.org/).

### Mac

Git provides binary package for Mac which is available for download [here](https://git-scm.com/download/mac).

### Linux

Most distributions ship with their own Git package. Just fire up package manager of your distro to install Git.

For example, on Debian and Ubuntu systems:

```
sudo apt install git
```

On CentOS/RedHat:

```
sudo yum install git-all
```

On Fedora:

```
sudo dnf install git-all
```

For more information, see [Git Download for Linux and Unix](https://git-scm.com/download/linux).

The only caveat is the Git version from package managers is often older, because most distros only pick one Git version throughout entire lifetime of the distro. Rolling-release distros (such as Arch Linux and Gentoo) packages newest version of Git from upstream as soon as it is ready to be shipped instead.

## From source

The steps below are tested on Debian and Ubuntu systems, but you can adapt them to other systems.

First, get build tools:

```
sudo apt install build-essential
```

Then install build-time dependencies for Git:

```
sudo apt install libcurl4-openssl-dev libexpat1-dev zlib1g-dev libssl-dev gettext
```

Note: On Debian and Ubuntu, there are two dev packages for `libcurl`: one provides TLS support by OpenSSL (`libcurl4-openssl-dev`), and one provides TLS support by GnuTLS (`libcurl4-gnutls-dev`). Git should be compiled with either packages. Choose one that match your preferences.

Download and extract Git tarball (substitute version as needed):

```
wget https://mirrors.edge.kernel.org/pub/software/scm/git/git-2.25.0.tar.gz
tar xvf git-2.25.0.tar.gz
```

Configure Git for compilation:

```
cd git-2.25.0
./configure \
 --prefix=/opt/git \
 --with-libpcre2 \
 --with-openssl \
 --without-tcltk
```

Compile Git:

```
make
```

Once compiled successfully, install:

```
sudo make install
```

If you run [Gitea as systemd service](/en-us/linux-service/#using-systemd), you need to append Git installation directory to `$PATH` on `Environment=` directive, as following:

```ini
Environment=... PATH=/opt/git/bin:/bin:/sbin:/usr/bin:/usr/sbin LD_LIBRARY_PATH=/opt/git/lib
```
