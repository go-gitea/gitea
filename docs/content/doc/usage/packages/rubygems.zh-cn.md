---
date: "2021-07-20T00:00:00+00:00"
title: "RubyGems 软件包注册表"
slug: "rubygems"
weight: 110
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "RubyGems"
    weight: 110
    identifier: "rubygems"
---

# RubyGems 软件包注册表

为您的用户或组织发布 [RubyGems](https://guides.rubygems.org/) 软件包。

**目录**

{{< toc >}}

## 要求

要使用RubyGems软件包注册表，您需要使用 [gem](https://guides.rubygems.org/command-reference/) 命令行工具来消费和发布软件包。

## 配置软件包注册表

要注册软件包注册表，请编辑 `~/.gem/credentials` 文件并添加：

```ini
---
https://gitea.example.com/api/packages/{owner}/rubygems: Bearer {token}
```

| 参数    | 描述                                                                                  |
| ------- | ------------------------------------------------------------------------------------- |
| `owner` | 软件包的所有者                                                                        |
| `token` | 您的[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}}) |

例如：

```
---
https://gitea.example.com/api/packages/testuser/rubygems: Bearer 3bd626f84b01cd26b873931eace1e430a5773cc4
```

## 发布软件包

通过运行以下命令来发布软件包：

```shell
gem push --host {host} {package_file}
```

| 参数           | 描述                     |
| -------------- | ------------------------ |
| `host`         | 软件包注册表的URL        |
| `package_file` | 软件包 `.gem` 文件的路径 |

例如：

```shell
gem push --host https://gitea.example.com/api/packages/testuser/rubygems test_package-1.0.0.gem
```

如果已经存在相同名称和版本的软件包，您将无法发布软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表安装软件包，您可以使用 [Bundler](https://bundler.io) 或 `gem`。

### Bundler

在您的 `Gemfile` 中添加一个新的 `source` 块：

```
source "https://gitea.example.com/api/packages/{owner}/rubygems" do
  gem "{package_name}"
end
```

| 参数           | 描述           |
| -------------- | -------------- |
| `owner`        | 软件包的所有者 |
| `package_name` | 软件包名称     |

例如：

```
source "https://gitea.example.com/api/packages/testuser/rubygems" do
  gem "test_package"
end
```

之后运行以下命令：

```shell
bundle install
```

### gem

执行以下命令：

```shell
gem install --host https://gitea.example.com/api/packages/{owner}/rubygems {package_name}
```

| 参数           | 描述           |
| -------------- | -------------- |
| `owner`        | 软件包的所有者 |
| `package_name` | 软件包名称     |

例如：

```shell
gem install --host https://gitea.example.com/api/packages/testuser/rubygems test_package
```

## 支持的命令

```
gem install
bundle install
gem push
```
