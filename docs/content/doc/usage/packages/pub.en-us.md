---
date: "2022-07-31T00:00:00+00:00"
title: "Pub Packages Repository"
slug: "pub"
weight: 90
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Pub"
    weight: 90
    identifier: "pub"
---

# Pub Packages Repository

Publish [Pub](https://dart.dev/guides/packages) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Pub package registry, you need to use the tools [dart](https://dart.dev/tools/dart-tool) and/or [flutter](https://docs.flutter.dev/reference/flutter-cli).

The following examples use dart.

## Configuring the package registry

To register the package registry and provide credentials, execute:

```shell
dart pub token add https://gitea.example.com/api/packages/{owner}/pub
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |

You need to provide your [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}).

## Publish a package

To publish a package, edit the `pubspec.yaml` and add the following line:

```yaml
publish_to: https://gitea.example.com/api/packages/{owner}/pub
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |

Now you can publish the package by running the following command:

```shell
dart pub publish
```

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a Pub package from the package registry, execute the following command:

```shell
dart pub add {package_name} --hosted-url=https://gitea.example.com/api/packages/{owner}/pub/
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |

For example:

```shell
# use latest version
dart pub add mypackage --hosted-url=https://gitea.example.com/api/packages/testuser/pub/
# specify version
dart pub add mypackage:1.0.8 --hosted-url=https://gitea.example.com/api/packages/testuser/pub/
```
