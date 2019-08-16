---
date: "2018-05-22T11:00:00+00:00"
title: "使用：反向代理"
slug: "reverse-proxies"
weight: 17
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "反向代理"
    weight: 16
    identifier: "reverse-proxies"
---

## 使用 Nginx 作为反向代理服务

如果您想使用 Nginx 作为 Gitea 的反向代理服务，您可以参照以下 `nginx.conf` 配置中 `server` 的 `http` 部分：

```
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

## 使用 Nginx 作为反向代理服务并将 Gitea 路由至一个子路径

如果您已经有一个域名并且想与 Gitea 共享该域名，您可以增加以下 `nginx.conf` 配置中 `server` 的 `http` 部分，为 Gitea 添加路由规则：

```
server {
    listen 80;
    server_name git.example.com;

    location /git/ { # Note: Trailing slash
        proxy_pass http://localhost:3000/; # Note: Trailing slash
    }
}
```

然后在您的 Gitea 配置文件中添加 `[server] ROOT_URL = http://git.example.com/git/`。

## 使用 Apache HTTPD 作为反向代理服务

如果您想使用 Apache HTTPD 作为 Gitea 的反向代理服务，您可以为您的 Apache HTTPD 作如下配置（在 Ubuntu 中，配置文件通常在 `/etc/apache2/httpd.conf` 目录下）：

```
<VirtualHost *:80>
    ...
    ProxyPreserveHost On
    ProxyRequests off
    AllowEncodedSlashes NoDecode
    ProxyPass / http://localhost:3000/ nocanon
    ProxyPassReverse / http://localhost:3000/
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
    # Note: no trailing slash after either /git or port
    ProxyPass /git http://localhost:3000 nocanon
    ProxyPassReverse /git http://localhost:3000
</VirtualHost>
```

然后在您的 Gitea 配置文件中添加 `[server] ROOT_URL = http://git.example.com/git/`。

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
    proxy /git/ http://localhost:3000 # Note: Trailing Slash after /git/
}
```

然后在您的 Gitea 配置文件中添加 `[server] ROOT_URL = http://git.example.com/git/`。
