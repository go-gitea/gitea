---
date: "2020-03-19T19:27:00+02:00"
title: "在 Kubernetes 安裝"
slug: "install-on-kubernetes"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Kubernetes"
    weight: 50
    identifier: "install-on-kubernetes"
---

# 使用 Helm 安裝 (在 Kubernetes)

Gitea 提供 Helm Chart 用來安裝於 kubernetes。

非自訂安裝可使用下列指令：

```
helm repo add gitea-charts https://dl.gitea.io/charts/
helm install gitea gitea-charts/gitea
```

若您想自訂安裝（包括使用 kubernetes ingress），請前往完整的 [Gitea helm chart configuration details](https://gitea.com/gitea/helm-chart/)
