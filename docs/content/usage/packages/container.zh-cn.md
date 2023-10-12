---
date: "2021-07-20T00:00:00+00:00"
title: "容器注册表"
slug: "container"
sidebar_position: 30
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "容器"
    sidebar_position: 30
    identifier: "container"
---

# 容器注册表

为您的用户或组织发布符合  [Open Container Initiative(OCI)](https://opencontainers.org/) 规范的镜像。
该容器注册表遵循 OCI 规范，并支持所有兼容的镜像类型，如 [Docker](https://www.docker.com/) 和 [Helm Charts](https://helm.sh/)。

## 目录

要使用容器注册表，您可以使用适用于特定镜像类型的工具。
以下示例使用 `docker` 客户端。

## 登录容器注册表

要推送镜像或者如果镜像位于私有注册表中，您需要进行身份验证：

```shell
docker login gitea.example.com
```

如果您使用的是 2FA 或 OAuth，请使用[个人访问令牌](development/api-usage.md#通过-api-认证)替代密码进行身份验证。

## 镜像命名约定

镜像必须遵循以下命名约定：

`{registry}/{owner}/{image}`

例如，以下是所有者为 `testuser` 的有效镜像名称示例：

`gitea.example.com/testuser/myimage`

`gitea.example.com/testuser/my-image`

`gitea.example.com/testuser/my/image`

**注意:** 该注册表仅支持大小写不敏感的标签名称。因此，`image:tag` 和 `image:Tag` 将被视为相同的镜像和标签。

## 推送镜像

通过执行以下命令来推送镜像：

```shell
docker push gitea.example.com/{owner}/{image}:{tag}
```

| 参数    | 描述         |
| ------- | ------------ |
| `owner` | 镜像的所有者 |
| `image` | 镜像的名称   |
| `tag`   | 镜像的标签   |

例如：

```shell
docker push gitea.example.com/testuser/myimage:latest
```

## 拉取镜像

通过执行以下命令来拉取镜像：

```shell
docker pull gitea.example.com/{owner}/{image}:{tag}
```

| Parameter | Description  |
| --------- | ------------ |
| `owner`   | 镜像的所有者 |
| `image`   | 镜像的名称   |
| `tag`     | 镜像的标签   |

例如：

```shell
docker pull gitea.example.com/testuser/myimage:latest
```
