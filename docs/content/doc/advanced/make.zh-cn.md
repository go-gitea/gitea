---
date: "2017-01-14T11:00:00-02:00"
title: "Make 安装"
slug: "make"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Make 安装"
    weight: 30
    identifier: "make"
---

# 安装 Make

Gitea 大量使用了 Make 工具来自动执行任务并改进开发，本文将介绍如何安装 Make。

### 在 Linux 环境下

可以使用包管理工具来安装 Make。

Ubuntu/Debian 环境，执行以下命令：

```bash
sudo apt-get install make
```

Fedora/RHEL/CentOS，执行以下命令：

```bash
sudo yum install make
```

### 在 Windows 环境下

您可以参照以下三种方案在 Windows 环境安装 Make：

- 直接使用 [exe文件](http://www.equation.com/servlet/equation.cmd?fa=make)：将适合您系统的exe文件拷贝到某处并添加至环境变量 `PATH` 中。
  - [32 位版本](ftp://ftp.equation.com/make/32/make.exe)
  - [64 位版本](ftp://ftp.equation.com/make/64/make.exe)
- 使用 [MinGW](http://www.mingw.org/) 工具：
  - 此处使用二进制文件 `mingw32-make.exe` 替代前面提到的 `make.exe`文件。同样您需要将包含此exe文件的 `bin` 目录添加至环境变量 `PATH`中。
- 通过 [Chocolatey](https://chocolatey.org/packages/make) 安装： 执行 `choco install make` 命令即可。
