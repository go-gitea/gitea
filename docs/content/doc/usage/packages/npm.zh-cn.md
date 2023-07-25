---
date: "2021-07-20T00:00:00+00:00"
title: "npm 软件包注册表"
slug: "npm"
weight: 70
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "npm"
    weight: 70
    identifier: "npm"
---

# npm Package Registry

为您的用户或组织发布 [npm](https://www.npmjs.com/) 包。

**目录**

{{< toc >}}

## 要求

要使用 npm 包注册表，您需要安装 [Node.js](https://nodejs.org/en/download/)  以及与之配套的软件包管理器，例如 [Yarn](https://classic.yarnpkg.com/en/docs/install) 或 [npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm/) 本身。

该注册表支持[作用域](https://docs.npmjs.com/misc/scope/)和非作用域软件包。

以下示例使用具有作用域 `@test` 的 `npm` 工具。

## 配置软件包注册表

要注册软件包注册表，您需要配置一个新的软件包源。

```shell
npm config set {scope}:registry https://gitea.example.com/api/packages/{owner}/npm/
npm config set -- '//gitea.example.com/api/packages/{owner}/npm/:_authToken' "{token}"
```

| 参数    | 描述                                                                                    |
| ------- | --------------------------------------------------------------------------------------- |
| `scope` | 软件包的作用域                                                                          |
| `owner` | 软件包的所有者                                                                          |
| `token` | 您的[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})。 |

例如：

```shell
npm config set @test:registry https://gitea.example.com/api/packages/testuser/npm/
npm config set -- '//gitea.example.com/api/packages/testuser/npm/:_authToken' "personal_access_token"
```

或者，不使用作用域：

```shell
npm config set registry https://gitea.example.com/api/packages/testuser/npm/
npm config set -- '//gitea.example.com/api/packages/testuser/npm/:_authToken' "personal_access_token"
```

## 发布软件包

在项目中运行以下命令发布软件包：

```shell
npm publish
```

如果已经存在相同名称和版本的软件包，您无法发布该软件包。您必须先删除现有的软件包。

## 删除软件包

通过运行以下命令删除软件包：

```shell
npm unpublish {package_name}[@{package_version}]
```

| 参数              | 描述       |
| ----------------- | ---------- |
| `package_name`    | 软件包名称 |
| `package_version` | 软件包版本 |

例如

```shell
npm unpublish @test/test_package
npm unpublish @test/test_package@1.0.0
```

## 安装软件包

要从软件包注册表中安装软件包，请执行以下命令：

```shell
npm install {package_name}
```

| 参数           | 描述       |
| -------------- | ---------- |
| `package_name` | 软件包名称 |

例如：

```shell
npm install @test/test_package
```

## 给软件包打标签

该注册表支持[版本标签](https://docs.npmjs.com/adding-dist-tags-to-packages/)，可以通过 `npm dist-tag` 管理：

```shell
npm dist-tag add {package_name}@{version} {tag}
```

| 参数           | 描述       |
| -------------- | ---------- |
| `package_name` | 软件包名称 |
| `version`      | 软件包版本 |
| `tag`          | 软件包标签 |

例如：

```shell
npm dist-tag add test_package@1.0.2 release
```

标签名称不能是有效的版本。所有可解析为版本的标签名称都将被拒绝。

## 搜索软件包

该注册表支持[搜索](https://docs.npmjs.com/cli/v7/commands/npm-search/)，但不支持像 `author:gitea` 这样的特殊搜索限定符。

## 支持的命令

```
npm install
npm ci
npm publish
npm unpublish
npm dist-tag
npm view
npm search
```
