---
date: "2020-03-19T19:27:00+02:00"
title: "Install on Kubernetes"
slug: "install-on-kubernetes"
sidebar_position: 80
toc: false
draft: false
aliases:
  - /en-us/install-on-kubernetes
menu:
  sidebar:
    parent: "installation"
    name: "Kubernetes"
    sidebar_position: 80
    identifier: "install-on-kubernetes"
---

# Installation with Helm (on Kubernetes)

Gitea provides a Helm Chart to allow for installation on kubernetes.

A non-customized install can be done with:

```
helm repo add gitea-charts https://dl.gitea.com/charts/
helm install gitea gitea-charts/gitea
```

If you would like to customize your install, which includes kubernetes ingress, please refer to the complete [Gitea helm chart configuration details](https://gitea.com/gitea/helm-chart/)

## Health check endpoint

Gitea comes with a health check endpoint `/api/healthz`, you can configure it in kubernetes like this:

```yaml
  livenessProbe:
    httpGet:
      path: /api/healthz
      port: http
    initialDelaySeconds: 200
    timeoutSeconds: 5
    periodSeconds: 10
    successThreshold: 1
    failureThreshold: 10
```

a successful health check response will respond with http code `200`, here's example:

```
HTTP/1.1 200 OK

{
  "status": "pass",
  "description": "Gitea: Git with a cup of tea",
  "checks": {
    "cache:ping": [
      {
        "status": "pass",
        "time": "2022-02-19T09:16:08Z"
      }
    ],
    "database:ping": [
      {
        "status": "pass",
        "time": "2022-02-19T09:16:08Z"
      }
    ]
  }
}
```

for more information, please reference to kubernetes documentation [Define a liveness HTTP request](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-liveness-http-request)
