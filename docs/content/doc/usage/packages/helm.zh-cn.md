---
date: "2022-04-14T00:00:00+00:00"
title: "Helm Chart 注册表"
slug: "helm"
weight: 50
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Helm"
    weight: 50
    identifier: "helm"
---

# Helm Chart 注册表

为您的用户或组织发布 [Helm](https://helm.sh/) charts。

**目录**

{{< toc >}}

## 要求

要使用 Helm Chart 注册表，可以使用诸如 `curl` 或 [`helm cm-push`](https://github.com/chartmuseum/helm-push/) 插件之类的简单HTTP客户端。

## 发布软件包

通过运行以下命令来发布软件包：

```shell
curl --user {username}:{password} -X POST --upload-file ./{chart_file}.tgz https://gitea.example.com/api/packages/{owner}/helm/api/charts
```

或者使用 `helm cm-push` 插件：

```shell
helm repo add  --username {username} --password {password} {repo} https://gitea.example.com/api/packages/{owner}/helm
helm cm-push ./{chart_file}.tgz {repo}
```

| 参数         | 描述                                                                                                                                                   |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `username`   | 您的Gitea用户名                                                                                                                                        |
| `password`   | 您的Gitea密码。如果您使用的是2FA或OAuth，请使用[个人访问令牌]({{< relref "doc/development/api-usage.zh-cn.md#通过-api-认证" >}})替代密码进行身份验证。 |
| `repo`       | 仓库名称                                                                                                                                               |
| `chart_file` | Helm Chart 归档文件                                                                                                                                    |
| `owner`      | 软件包的所有者                                                                                                                                         |

## 安装软件包

要从注册表中安装Helm Chart，请执行以下命令：

```shell
helm repo add  --username {username} --password {password} {repo} https://gitea.example.com/api/packages/{owner}/helm
helm repo update
helm install {name} {repo}/{chart}
```

| 参数       | 描述                        |
| ---------- | --------------------------- |
| `username` | 您的Gitea用户名             |
| `password` | 您的Gitea密码或个人访问令牌 |
| `repo`     | 存储库的名称                |
| `owner`    | 软件包的所有者              |
| `name`     | 本地名称                    |
| `chart`    | Helm Chart的名称            |
