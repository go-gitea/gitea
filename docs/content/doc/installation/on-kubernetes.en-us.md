---
date: "2020-03-19T19:27:00+02:00"
title: "Install on Kubernetes"
slug: "install-on-kubernetes"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Kubernetes"
    weight: 50
    identifier: "install-on-kubernetes"
---

# Installation with Helm (on Kubernetes)

Gitea provides a Helm Chart to allow for installation on kubernetes.

A non-customized install can be done with:

```
helm repo add gitea-charts https://dl.gitea.io/charts/
helm install gitea gitea-charts/gitea
```

If you would like to customize your install, which includes kubernetes ingress, please refer to the complete [Gitea helm chart configuration details](https://gitea.com/gitea/helm-chart/)
