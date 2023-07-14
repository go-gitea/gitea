---
date: "2023-01-20T00:00:00+00:00"
title: "Chef Package Registry"
slug: "chef"
weight: 5
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Chef"
    weight: 5
    identifier: "chef"
---

# Chef Package Registry

Publish [Chef](https://chef.io/) cookbooks for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Chef package registry, you have to use [`knife`](https://docs.chef.io/workstation/knife/).

## Authentication

The Chef package registry does not use an username:password authentication but signed requests with a private:public key pair.
Visit the package owner settings page to create the necessary key pair.
Only the public key is stored inside Gitea. if you loose access to the private key you must re-generate the key pair.
[Configure `knife`](https://docs.chef.io/workstation/knife_setup/) to use the downloaded private key with your Gitea username as `client_name`.

## Configure the package registry

To [configure `knife`](https://docs.chef.io/workstation/knife_setup/) to use the Gitea package registry add the url to the `~/.chef/config.rb` file.

```
knife[:supermarket_site] = 'https://gitea.example.com/api/packages/{owner}/chef'
```

| Parameter | Description |
| --------- | ----------- |
| `owner`   | The owner of the package. |

## Publish a package

To publish a Chef package execute the following command:

```shell
knife supermarket share {package_name}
```

| Parameter      | Description |
| -------------- | ----------- |
| `package_name` | The package name. |

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a package from the package registry, execute the following command:

```shell
knife supermarket install {package_name}
```

Optional you can specify the package version:

```shell
knife supermarket install {package_name} {package_version}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `package_name`    | The package name. |
| `package_version` | The package version. |

## Delete a package

If you want to remove a package from the registry, execute the following command:

```shell
knife supermarket unshare {package_name}
```

Optional you can specify the package version:

```shell
knife supermarket unshare {package_name}/versions/{package_version}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `package_name`    | The package name. |
| `package_version` | The package version. |
