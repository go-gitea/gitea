---
date: "2022-07-31T00:00:00+00:00"
title: "Pub 软件包注册表"
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

# Pub 软件包注册表

为您的用户或组织发布 [Pub](https://dart.dev/guides/packages) 软件包。

**目录**

{{< toc >}}

## 要求

要使用Pub软件包注册表，您需要使用 [dart](https://dart.dev/tools/dart-tool) 和/或 [flutter](https://docs.flutter.dev/reference/flutter-cli). 工具。

以下示例使用 `dart`。

## 配置软件包注册表

要注册软件包注册表并提供凭据，请执行以下操作：

```shell
dart pub token add https://gitea.example.com/api/packages/{owner}/pub
```

| 占位符  | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

您需要提供您的[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})。

## 发布软件包

要发布软件包，请编辑 `pubspec.yaml` 文件，并添加以下行：

```yaml
publish_to: https://gitea.example.com/api/packages/{owner}/pub
```

| 占位符  | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

现在，您可以通过运行以下命令来发布软件包：

```shell
dart pub publish
```

如果已存在具有相同名称和版本的软件包，则无法发布软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表安装Pub软件包，请执行以下命令：

```shell
dart pub add {package_name} --hosted-url=https://gitea.example.com/api/packages/{owner}/pub/
```

| 参数           | 描述           |
| -------------- | -------------- |
| `owner`        | 软件包的所有者 |
| `package_name` | 软件包名称     |

例如：

```shell
# use latest version
dart pub add mypackage --hosted-url=https://gitea.example.com/api/packages/testuser/pub/
# specify version
dart pub add mypackage:1.0.8 --hosted-url=https://gitea.example.com/api/packages/testuser/pub/
```
