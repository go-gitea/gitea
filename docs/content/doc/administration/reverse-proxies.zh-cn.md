---
date: "2018-05-22T11:00:00+00:00"
title: "使用：反向代理"
slug: "reverse-proxies"
weight: 16
toc: false
draft: false
menu:
  sidebar:
    parent: "administration"
    name: "反向代理"
    weight: 16
    identifier: "reverse-proxies"
---

# 反向代理

**目录**

{{< toc >}}

## 使用 Nginx 作为反向代理服务

如果您想使用 Nginx 作为 Gitea 的反向代理服务，您可以参照以下 `nginx.conf` 配置中 `server` 的 `http` 部分：

```
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 使用 Nginx 作为反向代理服务并将 Gitea 路由至一个子路径

如果您已经有一个域名并且想与 Gitea 共享该域名，您可以增加以下 `nginx.conf` 配置中 `server` 的 `http` 部分，为 Gitea 添加路由规则：

```
server {
    listen 80;
    server_name git.example.com;

    # 注意: /git/ 最后需要有一个路径符号
    location /git/ {
        # 注意: 反向代理后端 URL 的最后需要有一个路径符号
        proxy_pass http://localhost:3000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

然后您**必须**在 Gitea 的配置文件中正确的添加类似 `[server] ROOT_URL = http://git.example.com/git/` 的配置项。

## 使用 Apache HTTPD 作为反向代理服务

如果您想使用 Apache HTTPD 作为 Gitea 的反向代理服务，您可以为您的 Apache HTTPD 作如下配置（在 Ubuntu 中，配置文件通常在 `/etc/apache2/httpd.conf` 目录下）：

```
<VirtualHost *:80>
    ...
    ProxyPreserveHost On
    ProxyRequests off
    AllowEncodedSlashes NoDecode
    ProxyPass / http://localhost:3000/ nocanon
</VirtualHost>
```

注：必须启用以下 Apache HTTPD 组件：`proxy`， `proxy_http`

## 使用 Apache HTTPD 作为反向代理服务并将 Gitea 路由至一个子路径

如果您已经有一个域名并且想与 Gitea 共享该域名，您可以增加以下配置为 Gitea 添加路由规则（在 Ubuntu 中，配置文件通常在 `/etc/apache2/httpd.conf` 目录下）：

```
<VirtualHost *:80>
    ...
    <Proxy *>
         Order allow,deny
         Allow from all
    </Proxy>
    AllowEncodedSlashes NoDecode
    # 注意: 路径和 URL 后面都不要写路径符号 '/'
    ProxyPass /git http://localhost:3000 nocanon
</VirtualHost>
```

然后您**必须**在 Gitea 的配置文件中正确的添加类似 `[server] ROOT_URL = http://git.example.com/git/` 的配置项。

注：必须启用以下 Apache HTTPD 组件：`proxy`， `proxy_http`

## 使用 Caddy 作为反向代理服务

如果您想使用 Caddy 作为 Gitea 的反向代理服务，您可以在 `Caddyfile` 中添加如下配置：

```
git.example.com {
    proxy / http://localhost:3000
}
```

## 使用 Caddy 作为反向代理服务并将 Gitea 路由至一个子路径

如果您已经有一个域名并且想与 Gitea 共享该域名，您可以在您的 `Caddyfile` 文件中增加以下配置，为 Gitea 添加路由规则：

```
git.example.com {
    # 注意: 路径 /git/ 最后需要有路径符号
    proxy /git/ http://localhost:3000
}
```

然后您**必须**在 Gitea 的配置文件中正确的添加类似 `[server] ROOT_URL = http://git.example.com/git/` 的配置项。

## 使用 Traefik 作为反向代理服务

如果您想使用 traefik 作为 Gitea 的反向代理服务，您可以在 `docker-compose.yaml` 中添加 label 部分（假设使用 docker 作为 traefik 的 provider）：

```yaml
gitea:
  image: gitea/gitea
  ...
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.gitea.rule=Host(`example.com`)"
    - "traefik.http.services.gitea-websecure.loadbalancer.server.port=3000"
```

这份配置假设您使用 traefik 来处理 HTTPS 服务，并在其和 Gitea 之间使用 HTTP 进行通信。
