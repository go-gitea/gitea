---
date: "2023-01-01T00:00:00+00:00"
title: "CRAN 软件包注册表"
slug: "cran"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "CRAN"
    sidebar_position: 35
    identifier: "cran"
---

# CRAN 软件包注册表

将 [R](https://www.r-project.org/) 软件包发布到您的用户或组织的类似 [CRAN](https://cran.r-project.org/) 的注册表。

## 要求

要使用CRAN软件包注册表，您需要安装 [R](https://cran.r-project.org/)。

## 配置软件包注册表

要注册软件包注册表，您需要将其添加到 `Rprofile.site` 文件中，可以是系统级别、用户级别 `~/.Rprofile` 或项目级别：

```
options("repos" = c(getOption("repos"), c(gitea="https://gitea.example.com/api/packages/{owner}/cran")))
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

如果需要提供凭据，可以将它们嵌入到URL(`https://user:password@gitea.example.com/...`)中。

## 发布软件包

要发布 R 软件包，请执行带有软件包内容的 HTTP `PUT` 操作。

源代码软件包：

```
PUT https://gitea.example.com/api/packages/{owner}/cran/src
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

二进制软件包：

```
PUT https://gitea.example.com/api/packages/{owner}/cran/bin?platform={platform}&rversion={rversion}
```

| 参数       | 描述           |
| ---------- | -------------- |
| `owner`    | 软件包的所有者 |
| `platform` | 平台的名称     |
| `rversion` | 二进制的R版本  |

例如：

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/package.zip \
     https://gitea.example.com/api/packages/testuser/cran/bin?platform=windows&rversion=4.2
```

如果同名和版本的软件包已存在，则无法发布软件包。您必须首先删除现有的软件包。

## 安装软件包

要从软件包注册表中安装R软件包，请执行以下命令：

```shell
install.packages("{package_name}")
```

| 参数           | 描述              |
| -------------- | ----------------- |
| `package_name` | The package name. |

例如：

```shell
install.packages("testpackage")
```
