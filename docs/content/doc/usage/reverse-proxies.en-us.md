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
If you want Nginx to serve your Gitea instance, add the following `server` section to the `http` section of `nginx.conf`:

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

##  Using Nginx as a reverse proxy and serve static resources directly
We can tune the performance in splitting requests into categories static and dynamic. 

CSS files, JavaScript files, images and web fonts are static content.
The front page, a repository view or issue list is dynamic content.

Nginx can serve static resources directly and proxy only the dynamic requests to gitea.
Nginx is optimized for serving static content, while the proxying of large responses might be the opposite of that
 (see https://serverfault.com/q/587386).

Download a snap shot of the gitea source repository to `/path/to/gitea/`.

We are only interested in the `public/` directory and you can delete the rest.

Depending on the scale of your user base, you might want to split the traffic to two distinct servers,
 or use a cdn for the static files.

### using a single node and a single domain

Set `[server] STATIC_URL_PREFIX = /_/static` in your configuration.

```
server {
    listen 80;
    server_name git.example.com;

    location /_/static {
        alias /path/to/gitea/public;
    }

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

### using two nodes and two domains

Set `[server] STATIC_URL_PREFIX = http://cdn.example.com/gitea` in your configuration.

```
# application server running gitea
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

```
# static content delivery server
server {
    listen 80;
    server_name cdn.example.com;

    location /gitea {
        alias /path/to/gitea/public;
    }

    location / {
        return 404;
    }
}
```

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

If you wish to use Let's Encrypt with webroot validation, add the line `ProxyPass /.well-known !` before `ProxyPass` to disable proxying these requests to Gitea.

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
