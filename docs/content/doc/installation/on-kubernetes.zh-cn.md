---
date: "2020-03-19T19:27:00+02:00"
title: "在 Kubernetes 中安装 Gitea"
slug: "install-on-kubernetes"
weight: 80
toc: false
draft: false
aliases:
  - /zh-cn/install-on-kubernetes
menu:
  sidebar:
    parent: "installation"
    name: "在 Kubernetes 中安装 Gitea"
    weight: 80
    identifier: "install-on-kubernetes"
---

# 使用 Helm 在 Kubernetes 云原生环境中安装 Gitea

Gitea 已经提供了便于在 Kubernetes 云原生环境中安装所需的 Helm Chart

默认安装指令为：

```bash
helm repo add gitea https://dl.gitea.com/charts
helm repo update
helm install gitea gitea/gitea
```

如果采用默认安装指令，Helm 会部署单实例的 Gitea, PostgreSQL, Memcached。若您想实现自定义安装（包括配置 Gitea 集群、NGINX Ingress、MySQL、MariaDB、持久存储等），请前往阅读：[Gitea Helm Chart](https://gitea.com/gitea/helm-chart/)

您也可以通过 `helm show` 命令导出 `README.md` 和配置文件 `values.yaml` 进行学习和编辑，例如：

```bash
helm show values gitea/gitea > values.yaml
helm show readme gitea/gitea > README.md

# 使用自定义的配置文件 values.yaml
helm install gitea -f values.yaml gitea/gitea
```

## 运行状况检查接口

Gitea 附带了一个运行状况检查接口 `/api/healthz`，你可以像这样在 Kubernetes 中配置它：

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

成功的运行状况检查响应代码为 HTTP `200`，下面是示例：

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

有关更多信息，请参考 Kubernetes 文档 [配置存活、就绪和启动探测器](https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
