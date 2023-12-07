---
date: "2021-07-20T00:00:00+00:00"
title: "NuGet 软件包注册表"
slug: "nuget"
sidebar_position: 80
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "NuGet"
    sidebar_position: 80
    identifier: "nuget"
---

# NuGet 软件包注册表

发布适用于您的用户或组织的 [NuGet](https://www.nuget.org/) 软件包。软件包注册表支持 V2 和 V3 API 协议，并且您还可以使用 [NuGet 符号软件包](https://docs.microsoft.com/en-us/nuget/create-packages/symbol-packages-snupkg)。

## 要求

要使用 NuGet 软件包注册表，您可以使用命令行界面工具，以及各种集成开发环境（IDE）中的 NuGet 功能，如 Visual Studio。有关 NuGet 客户端的更多信息，请参[阅官方文档](https://docs.microsoft.com/en-us/nuget/install-nuget-client-tools)。
以下示例使用 `dotnet nuget` 工具。

## 配置软件包注册表

要注册软件包注册表，您需要配置一个新的NuGet源：

```shell
dotnet nuget add source --name {source_name} --username {username} --password {password} https://gitea.example.com/api/packages/{owner}/nuget/index.json
```

| 参数          | 描述                                                                                                                                   |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `source_name` | 所需源名称                                                                                                                             |
| `username`    | 您的Gitea用户名                                                                                                                        |
| `password`    | 您的Gitea密码。如果您使用2FA或OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)代替密码。 |
| `owner`       | 软件包的所有者                                                                                                                         |

例如：

```shell
dotnet nuget add source --name gitea --username testuser --password password123 https://gitea.example.com/api/packages/testuser/nuget/index.json
```

您可以在不提供凭据的情况下添加源，并在发布软件包时使用--api-key参数。在这种情况下，您需要提供[个人访问令牌](development/api-usage.md#通过-api-认证)。

## 发布软件包

通过运行以下命令发布软件包：

```shell
dotnet nuget push --source {source_name} {package_file}
```

| 参数           | 描述                         |
| -------------- | ---------------------------- |
| `source_name`  | 所需源名称                   |
| `package_file` | 软件包 `.nupkg` 文件的路径。 |

例如：

```shell
dotnet nuget push --source gitea test_package.1.0.0.nupkg
```

如果已经存在相同名称和版本的软件包，您无法发布该软件包。您必须先删除现有的软件包。

### 符号软件包

NuGet 软件包注册表支持构建用于符号服务器的符号软件包。客户端可以请求嵌入在符号软件包（`.snupkg`）中的 PDB 文件。
为此，请将 NuGet 软件包注册表注册为符号源：

```
https://gitea.example.com/api/packages/{owner}/nuget/symbols
```

| 参数    | 描述                 |
| ------- | -------------------- |
| `owner` | 软件包注册表的所有者 |

例如：

```
https://gitea.example.com/api/packages/testuser/nuget/symbols
```

## 安装软件包

要从软件包注册表安装 NuGet 软件包，请执行以下命令：

```shell
dotnet add package --source {source_name} --version {package_version} {package_name}
```

| 参数              | 描述         |
| ----------------- | ------------ |
| `source_name`     | 所需源名称   |
| `package_name`    | 软件包名称   |
| `package_version` | 软件包版本。 |

例如：

```shell
dotnet add package --source gitea --version 1.0.0 test_package
```

## 支持的命令

```
dotnet add
dotnet nuget push
dotnet nuget delete
```
