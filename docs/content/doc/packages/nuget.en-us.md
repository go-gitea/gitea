---
date: "2021-07-20T00:00:00+00:00"
title: "NuGet Packages Repository"
slug: "packages/nuget"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "NuGet"
    weight: 50
    identifier: "nuget"
---

# NuGet Packages Repository

Publish [NuGet](https://www.nuget.org/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the NuGet package registry, you can use command-line interface tools as well as NuGet features in various IDEs like Visual Studio.
More informations about NuGet clients can be found in [the official documentation](https://docs.microsoft.com/en-us/nuget/install-nuget-client-tools).
The following examples use the `dotnet nuget` tool.

## Configuring the package registry

To register the package registry you need to configure a new NuGet feed source:

```shell
dotnet nuget add source --name {source_name} --username {username} --password {password} https://gitea.example.com/api/v1/packages/{owner}/nuget/index.json
```

| Parameter     | Description |
| ------------- | ----------- |
| `source_name` | The desired source name. |
| `username`    | Your Gitea username. |
| `password`    | Your Gitea password or a personal access token. |
| `owner`       | The owner of the package. |

For example:

```shell
dotnet nuget add source --name gitea --username testuser --password password123 https://gitea.example.com/api/v1/packages/testuser/nuget/index.json
```

## Publish a package

Publish a package by running the following command:

```shell
dotnet nuget push --source {source_name} {package_file}
```

| Parameter      | Description |
| -------------- | ----------- |
| `source_name`  | The desired source name. |
| `package_file` | Path to the package `.nupkg` file. |

For example:

```shell
dotnet nuget push --source gitea test_package.1.0.0.nupkg
```

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a NuGet package from the package registry, execute the following command:

```shell
dotnet add package --source {source_name} --version {package_version} {package_name}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `source_name`     | The desired source name. |
| `package_name`    | The package name. |
| `package_version` | The package version. |

For example:

```shell
dotnet add package --source gitea --version 1.0.0 test_package
```