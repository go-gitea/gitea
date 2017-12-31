---
date: "2017-01-14T11:00:00-02:00"
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

Gitea makes heavy use of Make to automate tasks and have a faster development. This guide cover how to install Make.

### On Linux

You can install with your package manager. 

On Ubuntu/Debian:

```bash
sudo apt-get install build-essential
```

On Fedora/RHEL/CentOS:

```bash
sudo yum install make
```

### On Windows

If you are using Windows, you can download and use one of these distributions of Make:

- [Single binary build](http://www.equation.com/servlet/equation.cmd?fa=make). Copy somewhere and add to `PATH`.
  - [32-bits version](ftp://ftp.equation.com/make/32/make.exe)
  - [64-bits version](ftp://ftp.equation.com/make/64/make.exe)
- [MinGW](http://www.mingw.org/) includes a build. The binary is called `mingw32-make.exe` instead of `make.exe`. Add the `bin` folder to your `PATH`.
- [Chocolatey package](https://chocolatey.org/packages/make). Run `choco install make`
