---
date: "2016-12-01T16:00:00+02:00"
title: "Installation from package"
slug: "install-from-package"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From package"
    weight: 20
    identifier: "install-from-package"
---

# Installation from package

**Table of Contents**

{{< toc >}}

## Alpine Linux

Alpine Linux has [Gitea](https://pkgs.alpinelinux.org/packages?name=gitea&branch=edge) in its community repository which follows the latest stable version.

```sh
apk add gitea
```

## Arch Linux

The rolling release distribution has [Gitea](https://www.archlinux.org/packages/community/x86_64/gitea/) in their official community repository and package updates are provided with new Gitea releases.

```sh
pacman -S gitea
```

## Arch Linux ARM

Arch Linux ARM provides packages for [aarch64](https://archlinuxarm.org/packages/aarch64/gitea), [armv7h](https://archlinuxarm.org/packages/armv7h/gitea) and [armv6h](https://archlinuxarm.org/packages/armv6h/gitea).

```sh
pacman -S gitea
```

## Canonical Snap

There is a [Gitea Snap](https://snapcraft.io/gitea) package which follows the latest stable version.

### This guide was tested with Ubuntu 20.04 and MariaDB:

* Install needed packages:

```sh
sudo snap install gitea
sudo apt install nginx mariadb-server
```
* Improve the security of your MariaDB Installation

``sh
sudo mysql_secure_installation``
More information can be found on this website: https://mariadb.com/kb/en/mysql_secure_installation/

* Create your database and database user:

```sql
sudo mysql -u root -p
        CREATE DATABASE gitea;
        GRANT ALL PRIVILEGES ON gitea.* TO 'gitea'@'localhost' IDENTIFIED BY 'the same password for gitea config';
        FLUSH PRIVILEGES;
        QUIT;
```
Detailed information about Database preperation can be found by following this link: https://docs.gitea.io/en-us/database-prep/

* Create nginx config to pass traffic to port 3000:
 
```sh
sudo nano /etc/nginx/conf.d/gitea.conf
```
 
* The Nginx config should look something like the following code block.
To use `HTTPS`, you should additionally pass a certificate and redirect all traffic to port 443.
The following link covers some information about a secure setup:
https://docs.gitea.io/en-us/https-setup/

This link may help to create a secure nginx configuration: https://ssl-config.mozilla.org/#server=nginx&version=2.4.41&config=intermediate&openssl=1.1.1d&guideline=5.6

For junior users this blogpost may help to get you started with securing your installation with nginx as reverse proxy and certbot for certificate issuing:
https://www.nginx.com/blog/using-free-ssltls-certificates-from-lets-encrypt-with-nginx/


```nginx
server {
    listen 80;
    server_name gitea.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```
More information about reverse proxies can be found here:
https://docs.gitea.io/en-us/reverse-proxies/
* Restart nginx
```sh
sudo systemctl restart nginx
```

* Create correct DNS entry on your DNS Server if not done already
* Configure your gitea settings
	The config file for the Snap can be found here if you want to make changes later: /var/snap/gitea/common/conf/app.ini

## SUSE and openSUSE

OpenSUSE build service provides packages for [openSUSE and SLE](https://software.opensuse.org/download/package?package=gitea&project=devel%3Atools%3Ascm) 
in the Development Software Configuration Management Repository

## Windows

There is a [Gitea](https://chocolatey.org/packages/gitea) package for Windows by [Chocolatey](https://chocolatey.org/).

```sh
choco install gitea
```

Or follow the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide.

## macOS

Currently, the only supported method of installation on MacOS is [Homebrew](http://brew.sh/).
Following the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide may work,
but is not supported. To install Gitea via `brew`:

```
brew tap gitea/tap https://gitea.com/gitea/homebrew-gitea
brew install gitea
```

## FreeBSD

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

## Third-party

Various other third-party packages of Gitea exist.
To see a curated list, head over to [awesome-gitea](https://gitea.com/gitea/awesome-gitea/src/branch/master/README.md#user-content-packages).

Do you know of an existing package that isn't on the list? Send in a PR to get it added!
