---
date: "2021-07-20T00:00:00+00:00"
title: "PyPI 软件包注册表"
slug: "pypi"
weight: 100
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "PyPI"
    weight: 100
    identifier: "pypi"
---

# PyPI 软件包注册表

为您的用户或组织发布 [PyPI](https://pypi.org/) 软件包。

**目录**

{{< toc >}}

## 要求

要使用 PyPI 软件包注册表，您需要使用 [pip](https://pypi.org/project/pip/) 工具来消费和使用 [twine](https://pypi.org/project/twine/) 工具来发布软件包。

## 配置软件包注册表

要注册软件包注册表，您需要编辑本地的 `~/.pypirc` 文件。添加以下内容：

```ini
[distutils]
index-servers = gitea

[gitea]
repository = https://gitea.example.com/api/packages/{owner}/pypi
username = {username}
password = {password}
```

| 占位符     | 描述                                                                                                                                      |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `owner`    | 软件包的所有者                                                                                                                            |
| `username` | 您的 Gitea 用户名                                                                                                                         |
| `password` | 您的 Gitea 密码。如果您使用 2FA 或 OAuth，请使用[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})替代密码 |

## 发布软件包

通过运行以下命令来发布软件包：

```shell
python3 -m twine upload --repository gitea /path/to/files/*
```

软件包文件的扩展名为 `.tar.gz` 和 `.whl`。

如果已存在具有相同名称和版本的软件包，则无法发布软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表安装 PyPI 软件包，请执行以下命令：

```shell
pip install --index-url https://{username}:{password}@gitea.example.com/api/packages/{owner}/pypi/simple --no-deps {package_name}
```

| 参数           | 描述                          |
| -------------- | ----------------------------- |
| `username`     | 您的 Gitea 用户名             |
| `password`     | 您的 Gitea 密码或个人访问令牌 |
| `owner`        | 软件包的所有者                |
| `package_name` | 软件包名称                    |

例如：

```shell
pip install --index-url https://testuser:password123@gitea.example.com/api/packages/testuser/pypi/simple --no-deps test_package
```

您可以使用 `--extra-index-url` 替代 `--index-url`，但这样会使您容易受到依赖混淆攻击，因为 `pip` 会先检查官方 PyPi 仓库中的软件包，然后再检查指定的自定义仓库。请阅读 `pip` 文档以获取更多信息。

## 支持的命令

```
pip install
twine upload
```
