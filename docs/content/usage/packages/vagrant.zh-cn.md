---
date: "2022-08-23T00:00:00+00:00"
title: "Vagrant 软件包注册表"
slug: "vagrant"
sidebar_position: 120
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Vagrant"
    sidebar_position: 120
    identifier: "vagrant"
---

# Vagrant 软件包注册表

为您的用户或组织发布 [Vagrant](https://www.vagrantup.com/) 软件包。

## 要求

要使用 Vagrant 软件包注册表，您需要安装 [Vagrant](https://www.vagrantup.com/downloads) 并使用类似于 `curl` 的工具进行 HTTP 请求。

## 发布软件包

通过执行 HTTP PUT 请求将 Vagrant box 发布到注册表：

```
PUT https://gitea.example.com/api/packages/{owner}/vagrant/{package_name}/{package_version}/{provider}.box
```

| 参数              | 描述                                                               |
| ----------------- | ------------------------------------------------------------------ |
| `owner`           | 软件包的所有者                                                     |
| `package_name`    | 软件包的名称                                                       |
| `package_version` | 软件包的版本，兼容 semver 格式                                     |
| `provider`        | [支持的提供程序名称](https://www.vagrantup.com/docs/providers)之一 |

上传 Hyper-V box 的示例：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/your/vagrant.box \
     https://gitea.example.com/api/packages/testuser/vagrant/test_system/1.0.0/hyperv.box
```

如果已经存在相同名称、版本和提供程序的软件包，则无法发布软件包。您必须首先删除现有的软件包。

## 安装软件包

要从软件包注册表安装软件包，请执行以下命令：

```shell
vagrant box add "https://gitea.example.com/api/packages/{owner}/vagrant/{package_name}"
```

| 参数           | 描述            |
| -------------- | --------------- |
| `owner`        | 软件包的所有者. |
| `package_name` | 软件包的名称    |

例如：

```shell
vagrant box add "https://gitea.example.com/api/packages/testuser/vagrant/test_system"
```

这将安装软件包的最新版本。要添加特定版本，请使用` --box-version` 参数。
如果注册表是私有的，您可以将您的[个人访问令牌](development/api-usage.md#通过-api-认证)传递给 `VAGRANT_CLOUD_TOKEN` 环境变量。

## 支持的命令

```
vagrant box add
```
