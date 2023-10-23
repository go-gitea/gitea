---
date: "2023-04-09T11:00:00+02:00"
title: "HTTPS配置"
slug: "https-setup"
sidebar_position: 12
toc: false
draft: false
menu:
  sidebar:
    parent: "administration"
    name: "HTTPS setup"
    sidebar_position: 12
    identifier: "https-setup"
---

# HTTPS setup to encrypt connections to Gitea

## 使用内置服务器

在启用HTTPS之前，确保您拥有有效的SSL/TLS证书。
建议在测试和评估情况下使用自签名证书，请运行 `gitea cert --host [HOST]` 以生成自签名证书

如果您在服务器上使用阿帕奇（Apache）或Nginx，建议参考 [反向代理指南](administration/reverse-proxies.md)。

要使用Gitea内置HTTPS支持，您必须编辑`app.ini`文件。

```ini
[server]
PROTOCOL  = https
ROOT_URL  = https://git.example.com:3000/
HTTP_PORT = 3000
CERT_FILE = cert.pem
KEY_FILE  = key.pem
```

请注意，如果您的证书由第三方证书颁发机构签名（即不是自签名的），则 cert.pem 应包含证书链。服务器证书必须是 cert.pem 中的第一个条目，后跟中介（如果有）。不必包含根证书，因为连接客户端必须已经拥有根证书才能建立信任关系。要了解有关配置值的更多信息，请查看 [配置备忘单](../config-cheat-sheet#server-server)。

对于“CERT_FILE”或“KEY_FILE”字段，当文件路径是相对路径时，文件路径相对于“GITEA_CUSTOM”环境变量。它也可以是绝对路径。

### 设置HTTP重定向

Gitea服务器仅支持监听一个端口；要重定向HTTP请求致HTTPS端口，您需要启用HTTP重定向服务：

```ini
[server]
REDIRECT_OTHER_PORT = true
; Port the redirection service should listen on
PORT_TO_REDIRECT = 3080
```

如果您使用Docker，确保端口已配置在 `docker-compose.yml` 文件

## 使用 ACME (默认: Let's Encrypt)

[ACME](https://tools.ietf.org/html/rfc8555) 是一种证书颁发机构标准协议，允许您自动请求和续订 SSL/TLS 证书。[Let`s Encrypt](https://letsencrypt.org/) 是使用此标准的免费公开信任的证书颁发机构服务器。仅实施“HTTP-01”和“TLS-ALPN-01”挑战。为了使 ACME 质询通过并验证您的域所有权，“80”端口（“HTTP-01”）或“443”端口（“TLS-ALPN-01”）上 gitea 域的外部流量必须由 gitea 实例提供服务。可能需要设置 [HTTP 重定向](#设置http重定向) 和端口转发才能正确路由外部流量。否则，到端口“80”的正常流量将自动重定向到 HTTPS。**您必须同意**ACME提供商的服务条款（默认为Let's Encrypt的 [服务条款](https://letsencrypt.org/documents/LE-SA-v1.2-2017年11月15日.pdf)。

使用默认 Let's Encrypt 的最小配置如下：

```ini
[server]
PROTOCOL=https
DOMAIN=git.example.com
ENABLE_ACME=true
ACME_ACCEPTTOS=true
ACME_DIRECTORY=https
;; Email can be omitted here and provided manually at first run, after which it is cached
ACME_EMAIL=email@example.com
```

小型配置请使用 [smallstep CA](https://github.com/smallstep/certificates), 点击 [教程](https://smallstep.com/docs/tutorials/acme-challenge) 了解更多信息。

```ini
[server]
PROTOCOL=https
DOMAIN=git.example.com
ENABLE_ACME=true
ACME_ACCEPTTOS=true
ACME_URL=https://ca.example.com/acme/acme/directory
;; Can be omitted if using the system's trust is preferred
;ACME_CA_ROOT=/path/to/root_ca.crt
ACME_DIRECTORY=https
ACME_EMAIL=email@example.com
```

要了解关于配置, 请访问 [配置备忘单](administration/config-cheat-sheet.md#server-server)获取更多信息

## 使用反向代理服务器

按照 [reverse proxy guide](administration/reverse-proxies.md) 的规则设置你的反向代理服务器

然后，按照下面的向导启用 HTTPS：

- [nginx](https://nginx.org/en/docs/http/configuring_https_servers.html)
- [apache2/httpd](https://httpd.apache.org/docs/2.4/ssl/ssl_howto.html)
- [caddy](https://caddyserver.com/docs/tls)

注意：仅在代理层启用 HTTPS 被称为 [TLS 终止代理](https://en.wikipedia.org/wiki/TLS_termination_proxy)。代理服务器接受传入的 TLS 连接，解密内容，然后将现在未加密的内容传递给 Gitea。只要代理和 Gitea 实例在同一台计算机上或在私有网络中的不同计算机上（代理暴露给外部网络），这通常是可以接受的。如果您的 Gitea 实例与代理隔离在公共网络上，或者如果您想要全端到端的加密，您还可以直接在 Gitea 中 [启用内置服务器的 HTTPS 支持](#使用内置服务器)，并将连接转发到 HTTPS 上。
