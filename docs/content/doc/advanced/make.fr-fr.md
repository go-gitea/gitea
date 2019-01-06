---
date: "2017-08-23T09:00:00+02:00"
title: "Make"
slug: "make"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Make"
    weight: 30
    identifier: "make"
---

# Make

Gitea fait largement usage de Make pour automatiser les tâches et avoir un développement plus rapide. Ce guide explique comment installer Make.

### Linux

Vous pouvez installer Make avec votre gestionnaire de paquetages 

Depuis Ubuntu/Debian:

```bash
sudo apt-get install build-essential
```

Depuis Fedora/RHEL/CentOS:

```bash
sudo yum install make
```

### Windows

Si vous utilisez Windows, vous pouvez télécharger une des versions suivantes de Make:

- [Simple binaire](http://www.equation.com/servlet/equation.cmd?fa=make). Copiez le quelque part et mettez à jour `PATH`.
  - [32-bits version](ftp://ftp.equation.com/make/32/make.exe)
  - [64-bits version](ftp://ftp.equation.com/make/64/make.exe)
- [MinGW](http://www.mingw.org/) includes a build. The binary is called `mingw32-make.exe` instead of `make.exe`. Add the `bin` folder to your `PATH`.
- [Chocolatey package](https://chocolatey.org/packages/make). Run `choco install make`
