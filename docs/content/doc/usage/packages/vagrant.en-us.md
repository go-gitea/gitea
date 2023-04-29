---
date: "2022-08-23T00:00:00+00:00"
title: "Vagrant Packages Repository"
slug: "vagrant"
weight: 120
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Vagrant"
    weight: 120
    identifier: "vagrant"
---

# Vagrant Packages Repository

Publish [Vagrant](https://www.vagrantup.com/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Vagrant package registry, you need [Vagrant](https://www.vagrantup.com/downloads) and a tool to make HTTP requests like `curl`.

## Publish a package

Publish a Vagrant box by performing a HTTP PUT request to the registry:

```
PUT https://gitea.example.com/api/packages/{owner}/vagrant/{package_name}/{package_version}/{provider}.box
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |
| `package_version` | The package version, semver compatible. |
| `provider`        | One of the [supported provider names](https://www.vagrantup.com/docs/providers). |

Example for uploading a Hyper-V box:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/your/vagrant.box \
     https://gitea.example.com/api/packages/testuser/vagrant/test_system/1.0.0/hyperv.box
```

You cannot publish a box if a box of the same name, version and provider already exists. You must delete the existing package first.

## Install a package

To install a box from the package registry, execute the following command:

```shell
vagrant box add "https://gitea.example.com/api/packages/{owner}/vagrant/{package_name}"
```

| Parameter      | Description |
| -------------- | ----------- |
| `owner`        | The owner of the package. |
| `package_name` | The package name. |

For example:

```shell
vagrant box add "https://gitea.example.com/api/packages/testuser/vagrant/test_system"
```

This will install the latest version of the package. To add a specific version, use the `--box-version` parameter.
If the registry is private you can pass your [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) in the `VAGRANT_CLOUD_TOKEN` environment variable.

## Supported commands

```
vagrant box add
```
