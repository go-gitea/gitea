---
date: "2018-05-22T11:00:00+00:00"
title: "Usage: Reverse Proxies"
slug: "reverse-proxies"
weight: 17
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Reverse Proxies"
    weight: 16
    identifier: "reverse-proxies"
---

##  Using Nginx as a reverse proxy
If you want Nginx to serve your Gitea instance, you can the following `server` section inside the `http` section of `nginx.conf`:

```
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

## Using Nginx with a sub-path as a reverse proxy

In case you already have a site, and you want Gitea to share the domain name, you can setup Nginx to serve Gitea under a sub-path by adding the following `server` section inside the `http` section of `nginx.conf`:

```
server {
    listen 80;
    server_name git.example.com;

    location /git/ { # Note: Trailing slash
        proxy_pass http://localhost:3000/; # Note: Trailing slash
    }
}
```

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.

## Using Apache HTTPD as a reverse proxy

If you want Apache HTTPD to serve your Gitea instance, you can add the following to your Apache HTTPD configuration (usually located at `/etc/apache2/httpd.conf` in Ubuntu):

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

Note: The following Apache HTTPD mods must be enabled: `proxy`, `proxy_http`

## Using Apache HTTPD with a sub-path as a reverse proxy

In case you already have a site, and you want Gitea to share the domain name, you can setup Apache HTTPD to serve Gitea under a sub-path by adding the following to you Apache HTTPD configuration (usually located at `/etc/apache2/httpd.conf` in Ubuntu):

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

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.

Note: The following Apache HTTPD mods must be enabled: `proxy`, `proxy_http`

## Using Caddy as a reverse proxy

If you want Caddy to serve your Gitea instance, you can add the following server block to your Caddyfile:

```
git.example.com {
    proxy / http://localhost:3000
}
```

## Using Caddy with a sub-path as a reverse proxy

In case you already have a site, and you want Gitea to share the domain name, you can setup Caddy to serve Gitea under a sub-path by adding the following to your server block in your Caddyfile:

```
git.example.com {
    proxy /git/ http://localhost:3000 # Note: Trailing Slash after /git/
}
```

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.
