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

```sh
wget -O gitea https://dl.gitea.io/gitea/1.4.3/gitea-1.4.3-linux-amd64
chmod +x gitea
```

## Verify GPG signature
Gitea signs all binaries with a [GPG key](https://pgp.mit.edu/pks/lookup?op=vindex&fingerprint=on&search=0x2D9AE806EC1592E2) to prevent against unwanted modification of binaries. To validate the binary download the signature file which ends in `.asc` for the binary you downloaded and use the gpg command line tool.

```sh
gpg --keyserver pgp.mit.edu --recv 0x2D9AE806EC1592E2
gpg --verify gitea-1.5.0-linux-amd64.asc gitea-1.5.0-linux-amd64
```

## Test

After getting a binary, it can be tested with `./gitea web` or moved to a permanent
location. When launched manually, Gitea can be killed using `Ctrl+C`.

```
./gitea web
```

## Recommended server configuration

### Prepare environment

Check that git is installed on the server, if it is not install it first.
```sh
git --version
```

Create user to run gitea (ex. `git`)
```sh
adduser \
   --system \
   --shell /bin/bash \
   --gecos 'Git Version Control' \
   --group \
   --disabled-password \
   --home /home/git \
   git
```

### Create required directory structure

```sh
mkdir -p /var/lib/gitea/{custom,data,indexers,public,log}
chown git:git /var/lib/gitea/{data,indexers,log}
chmod 750 /var/lib/gitea/{data,indexers,log}
mkdir /etc/gitea
chown root:git /etc/gitea
chmod 770 /etc/gitea
```

**NOTE:** `/etc/gitea` is temporary set with write rights for user `git` so that Web installer could write configuration file. After installation is done it is recommended to set rights to read-only using:
```
chmod 750 /etc/gitea
chmod 644 /etc/gitea/app.ini
```

### Copy gitea binary to global location

```
cp gitea /usr/local/bin/gitea
```

### Create service file to start gitea automatically

See how to create [Linux service]({{< relref "run-as-service-in-ubuntu.en-us.md" >}})

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
