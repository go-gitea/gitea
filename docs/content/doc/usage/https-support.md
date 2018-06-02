---
date: "2018-06-02T11:00:00+02:00"
title: "Usage: HTTPS setup"
slug: "https-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "HTTPS setup"
    weight: 12
    identifier: "https-setup"
---

# HTTPS setup to encrypt connections to Gitea

## Using built-in server

Before you enable HTTPS make sure that you have valid SSL/TLS certificates.
You could use self-generated certificates for evaluation and testing. Please run `gitea cert --host [HOST]` to generate a self signed certificate.

To use Gitea's built-in HTTPS support you must change your `app.ini` file:

```ini
[server]
PROTOCOL=https
ROOT_URL = `https://git.example.com:3000/`
HTTP_PORT = 3000
CERT_FILE = cert.pem
KEY_FILE = key.pem
```
To learn more about the config values, please checkout the [Config Cheat Sheet](../config-cheat-sheet#server).

## Using reverse proxy

Setup up your reverse proxy like shown in the [reverse proxy guide](../reverse-proxies).

After that, enable HTTPS by following one of these guides:

* [nginx](https//nginx.org/en/docs/http/configuring_https_servers.html)
* [apache2/httpd](https://httpd.apache.org/docs/2.4/ssl/ssl_howto.html)
* [caddy](https://caddyserver.com/docs/tls)

Note: You connection between your reverse proxy and gitea might be unencrypted. To encrypt it too follow the [built-in server guide](#using-built-in-server) and change
the proxy url to `https://[URL]`.
