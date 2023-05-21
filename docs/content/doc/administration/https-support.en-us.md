---
date: "2018-06-02T11:00:00+02:00"
title: "HTTPS setup"
slug: "https-setup"
weight: 12
toc: false
draft: false
aliases:
  - /en-us/https-setup
menu:
  sidebar:
    parent: "administration"
    name: "HTTPS setup"
    weight: 12
    identifier: "https-setup"
---

# HTTPS setup to encrypt connections to Gitea

**Table of Contents**

{{< toc >}}

## Using the built-in server

Before you enable HTTPS, make sure that you have valid SSL/TLS certificates.
You could use self-generated certificates for evaluation and testing. Please run `gitea cert --host [HOST]` to generate a self signed certificate.

If you are using Apache or nginx on the server, it's recommended to check the [reverse proxy guide]({{< relref "doc/administration/reverse-proxies.en-us.md" >}}).

To use Gitea's built-in HTTPS support, you must change your `app.ini` file:

```ini
[server]
PROTOCOL  = https
ROOT_URL  = https://git.example.com:3000/
HTTP_PORT = 3000
CERT_FILE = cert.pem
KEY_FILE  = key.pem
```

Note that if your certificate is signed by a third party certificate authority (i.e. not self-signed), then cert.pem should contain the certificate chain. The server certificate must be the first entry in cert.pem, followed by the intermediaries in order (if any). The root certificate does not have to be included because the connecting client must already have it in order to estalbish the trust relationship.
To learn more about the config values, please checkout the [Config Cheat Sheet](../config-cheat-sheet#server-server).

For the `CERT_FILE` or `KEY_FILE` field, the file path is relative to the `GITEA_CUSTOM` environment variable when it is a relative path. It can be an absolute path as well.

### Setting up HTTP redirection

The Gitea server is only able to listen to one port; to redirect HTTP requests to the HTTPS port, you will need to enable the HTTP redirection service:

```ini
[server]
REDIRECT_OTHER_PORT = true
; Port the redirection service should listen on
PORT_TO_REDIRECT = 3080
```

If you are using Docker, make sure that this port is configured in your `docker-compose.yml` file.

## Using ACME (Default: Let's Encrypt)

[ACME](https://tools.ietf.org/html/rfc8555) is a Certificate Authority standard protocol that allows you to automatically request and renew SSL/TLS certificates. [Let's Encrypt](https://letsencrypt.org/) is a free publicly trusted Certificate Authority server using this standard. Only `HTTP-01` and `TLS-ALPN-01` challenges are implemented. In order for ACME challenges to pass and verify your domain ownership, external traffic to the gitea domain on port `80` (`HTTP-01`) or port `443` (`TLS-ALPN-01`) has to be served by the gitea instance. Setting up [HTTP redirection](#setting-up-http-redirection) and port-forwards might be needed for external traffic to route correctly. Normal traffic to port `80` will otherwise be automatically redirected to HTTPS. **You must consent** to the ACME provider's terms of service (default Let's Encrypt's [terms of service](https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf)).

Minimum setup using the default Let's Encrypt:

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

Minimum setup using a [smallstep CA](https://github.com/smallstep/certificates), refer to [their tutorial](https://smallstep.com/docs/tutorials/acme-challenge) for more information.

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

To learn more about the config values, please checkout the [Config Cheat Sheet](../config-cheat-sheet#server-server).

## Using a reverse proxy

Setup up your reverse proxy as shown in the [reverse proxy guide](../reverse-proxies).

After that, enable HTTPS by following one of these guides:

- [nginx](https://nginx.org/en/docs/http/configuring_https_servers.html)
- [apache2/httpd](https://httpd.apache.org/docs/2.4/ssl/ssl_howto.html)
- [caddy](https://caddyserver.com/docs/tls)

Note: Enabling HTTPS only at the proxy level is referred as [TLS Termination Proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy). The proxy server accepts incoming TLS connections, decrypts the contents, and passes the now unencrypted contents to Gitea. This is normally fine as long as both the proxy and Gitea instances are either on the same machine, or on different machines within private network (with the proxy is exposed to outside network). If your Gitea instance is separated from your proxy over a public network, or if you want full end-to-end encryption, you can also [enable HTTPS support directly in Gitea using built-in server](#using-the-built-in-server) and forward the connections over HTTPS instead.
