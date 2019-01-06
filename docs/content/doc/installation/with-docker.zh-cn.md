---
date: "2016-12-01T16:00:00+02:00"
title: "从Docker安装"
slug: "install-with-docker"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "从Docker安装"
    weight: 10
    identifier: "install-with-docker"
---

# 从Docker安装

阅读本章之前我们已经假设您对docker已经有了解并能够正常使用docker。

我们在 Docker Hub 的 Gitea 组织中提供了自动更新的 Docker 镜像，它会保持最新的稳定版。你也可以用其它 Docker 服务来更新。首先你需要pull镜像：

```
docker pull gitea/gitea:latest
```

如果要将git和其它数据持久化，你需要创建一个目录来作为数据存储的地方：

```
sudo mkdir -p /var/lib/gitea
```

然后就可以运行 docker 容器了，这很简单。 当然你需要定义端口数数据目录：

```
docker run -d --name=gitea -p 10022:22 -p 10080:3000 -v /var/lib/gitea:/data gitea/gitea:latest
```

然后 容器已经运行成功，在浏览器中访问 http://hostname:10080 就可以看到界面了。你可以尝试在上面创建项目，clone操作 `git clone ssh://git@hostname:10022/username/repo.git`.

注意：目前端口改为非3000时，需要修改配置文件 `LOCAL_ROOT_URL  = http://localhost:3000/`。

## 需要帮助?

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "seek-help.zh-cn.md" >}})
