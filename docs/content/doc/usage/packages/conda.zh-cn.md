---
date: "2022-12-28T00:00:00+00:00"
title: "Conda 软件包注册表"
slug: "conda"
weight: 25
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Conda"
    weight: 25
    identifier: "conda"
---

# Conda 软件包注册表

为您的用户或组织发布 [Conda](https://docs.conda.io/en/latest/) 软件包。

**目录**

{{< toc >}}

## 要求

要使用 Conda 软件包注册表，您需要使用 [conda](https://docs.conda.io/projects/conda/en/stable/user-guide/install/index.html) 命令行工具。

## 配置软件包注册表

要注册软件包注册表并提供凭据，请编辑您的 `.condarc` 文件：

```yaml
channel_alias: https://gitea.example.com/api/packages/{owner}/conda
channels:
  - https://gitea.example.com/api/packages/{owner}/conda
default_channels:
  - https://gitea.example.com/api/packages/{owner}/conda
```

| 占位符  | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

有关各个设置的解释，请参阅[官方文档](https://conda.io/projects/conda/en/latest/user-guide/configuration/use-condarc.html)。

如果需要提供凭据，可以将它们作为通道 URL 的一部分嵌入（`https://user:password@gitea.example.com/...`）。

## 发布软件包

要发布一个软件包，请执行一个HTTP `PUT`操作，请求正文中包含软件包内容。

```
PUT https://gitea.example.com/api/packages/{owner}/conda/{channel}/{filename}
```

| 占位符     | 描述                                                                                                |
| ---------- | --------------------------------------------------------------------------------------------------- |
| `owner`    | 软件包的所有者                                                                                      |
| `channel`  | 软件包的[通道](https://conda.io/projects/conda/en/latest/user-guide/concepts/channels.html)（可选） |
| `filename` | 文件名                                                                                              |

使用HTTP基本身份验证的示例请求：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/package-1.0.conda \
     https://gitea.example.com/api/packages/testuser/conda/package-1.0.conda
```

如果已经存在同名和版本的软件包，则无法发布软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表中安装软件包，请执行以下命令之一：

```shell
conda install {package_name}
conda install {package_name}={package_version}
conda install -c {channel} {package_name}
```

| 参数              | 描述                 |
| ----------------- | -------------------- |
| `package_name`    | 软件包的名称         |
| `package_version` | 软件包的版本         |
| `channel`         | 软件包的通道（可选） |
