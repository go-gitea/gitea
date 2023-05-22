---
date: "2020-03-19T19:27:00+02:00"
title: "在 Kubernetes 安裝"
slug: "install-on-kubernetes"
weight: 80
toc: false
draft: false
aliases:
  - /zh-tw/install-on-kubernetes
menu:
  sidebar:
    parent: "installation"
    name: "Kubernetes"
    weight: 80
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

## 運行狀況檢查終端節點

Gitea 附帶了一個運行狀況檢查端點 `/api/healthz`，你可以像這樣在 kubernetes 中配置它:

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

成功的運行狀況檢查回應將使用 HTTP 代碼 `200` 進行回應，下面是示例:

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

有關更多信息，請參考kubernetes文檔[定義一個存活態 HTTP請求接口]（https://kubernetes.io/zh/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/）
