---
date: "2022-04-14T00:00:00+00:00"
title: "Helm Chart Registry"
slug: "usage/packages/helm"
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

# Helm Chart Registry

Publish [Helm](https://helm.sh/) charts for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Helm Chart registry use a simple HTTP client like `curl` or the [`helm cm-push`](https://github.com/chartmuseum/helm-push/) plugin.

## Publish a package

Publish a package by running the following command:

```shell
curl --user {username}:{password} -X POST --upload-file ./{chart_file}.tgz https://gitea.example.com/api/packages/{owner}/helm/api/charts
```

or with the `helm cm-push` plugin:

```shell
helm repo add  --username {username} --password {password} {repo} https://gitea.example.com/api/packages/{owner}/helm
helm cm-push ./{chart_file}.tgz {repo}
```

| Parameter    | Description |
| ------------ | ----------- |
| `username`   | Your Gitea username. |
| `password`   | Your Gitea password. If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password. |
| `repo`       | The name for the repository. |
| `chart_file` | The Helm Chart archive. |
| `owner`      | The owner of the package. |

## Install a package

To install a Helm char from the registry, execute the following command:

```shell
helm repo add  --username {username} --password {password} {repo} https://gitea.example.com/api/packages/{owner}/helm
helm repo update
helm install {name} {repo}/{chart}
```

| Parameter  | Description |
| ---------- | ----------- |
| `username` | Your Gitea username. |
| `password` | Your Gitea password or a personal access token. |
| `repo`     | The name for the repository. |
| `owner`    | The owner of the package. |
| `name`     | The local name. |
| `chart`    | The name Helm Chart. |
