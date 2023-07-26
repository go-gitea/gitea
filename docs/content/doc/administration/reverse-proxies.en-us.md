---
date: "2018-05-22T11:00:00+00:00"
title: "Reverse Proxies"
slug: "reverse-proxies"
weight: 16
toc: false
draft: false
aliases:
  - /en-us/reverse-proxies
menu:
  sidebar:
    parent: "administration"
    name: "Reverse Proxies"
    weight: 16
    identifier: "reverse-proxies"
---

# Reverse Proxies

**Table of Contents**

{{< toc >}}

## Nginx

If you want Nginx to serve your Gitea instance, add the following `server` section to the `http` section of `nginx.conf`:

```
server {
    listen 80;
    server_name git.example.com;

    location / {
        client_max_body_size 512M;
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Resolving Error: 413 Request Entity Too Large

This error indicates nginx is configured to restrict the file upload size,
it affects attachment uploading, form posting, package uploading and LFS pushing, etc.
You can fine tune the `client_max_body_size` option according to [nginx document](http://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size).

## Nginx with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Nginx to serve Gitea under a sub-path by adding the following `server` section inside the `http` section of `nginx.conf`:

```
server {
    listen 80;
    server_name git.example.com;

    # Note: Trailing slash
    location /gitea/ {
        client_max_body_size 512M;

        # make nginx use unescaped URI, keep "%2F" as is
        rewrite ^ $request_uri;
        rewrite ^/gitea(/.*) $1 break;
        proxy_pass http://127.0.0.1:3000$uri;

        # other common HTTP headers, see the "Nginx" config section above
        proxy_set_header ...
    }
}
```

Then you **MUST** set something like `[server] ROOT_URL = http://git.example.com/git/` correctly in your configuration.

## Nginx and serve static resources directly

We can tune the performance in splitting requests into categories static and dynamic.

CSS files, JavaScript files, images and web fonts are static content.
The front page, a repository view or issue list is dynamic content.

Nginx can serve static resources directly and proxy only the dynamic requests to Gitea.
Nginx is optimized for serving static content, while the proxying of large responses might be the opposite of that
(see [https://serverfault.com/q/587386](https://serverfault.com/q/587386)).

Download a snapshot of the Gitea source repository to `/path/to/gitea/`.
After this, run `make frontend` in the repository directory to generate the static resources. We are only interested in the `public/` directory for this task, so you can delete the rest.
(You will need to have [Node with npm](https://nodejs.org/en/download/) and `make` installed to generate the static resources)

Depending on the scale of your user base, you might want to split the traffic to two distinct servers,
or use a cdn for the static files.

### Single node and single domain

Set `[server] STATIC_URL_PREFIX = /_/static` in your configuration.

```apacheconf
server {
    listen 80;
    server_name git.example.com;

    location /_/static/assets/ {
        alias /path/to/gitea/public/;
    }

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

### Two nodes and two domains

Set `[server] STATIC_URL_PREFIX = http://cdn.example.com/gitea` in your configuration.

```apacheconf
# application server running Gitea
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

```apacheconf
# static content delivery server
server {
    listen 80;
    server_name cdn.example.com;

    location /gitea/ {
        alias /path/to/gitea/public/;
    }

    location / {
        return 404;
    }
}
```

## Apache HTTPD

If you want Apache HTTPD to serve your Gitea instance, you can add the following to your Apache HTTPD configuration (usually located at `/etc/apache2/httpd.conf` in Ubuntu):

```apacheconf
<VirtualHost *:80>
    ...
    ProxyPreserveHost On
    ProxyRequests off
    AllowEncodedSlashes NoDecode
    ProxyPass / http://localhost:3000/ nocanon
</VirtualHost>
```

Note: The following Apache HTTPD mods must be enabled: `proxy`, `proxy_http`.

If you wish to use Let's Encrypt with webroot validation, add the line `ProxyPass /.well-known !` before `ProxyPass` to disable proxying these requests to Gitea.

## Apache HTTPD with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Apache HTTPD to serve Gitea under a sub-path by adding the following to you Apache HTTPD configuration (usually located at `/etc/apache2/httpd.conf` in Ubuntu):

```apacheconf
<VirtualHost *:80>
    ...
    <Proxy *>
         Order allow,deny
         Allow from all
    </Proxy>
    AllowEncodedSlashes NoDecode
    # Note: no trailing slash after either /git or port
    ProxyPass /git http://localhost:3000 nocanon
</VirtualHost>
```

Then you **MUST** set something like `[server] ROOT_URL = http://git.example.com/git/` correctly in your configuration.

Note: The following Apache HTTPD mods must be enabled: `proxy`, `proxy_http`.

## Caddy

If you want Caddy to serve your Gitea instance, you can add the following server block to your Caddyfile:

```apacheconf
git.example.com {
    reverse_proxy localhost:3000
}
```

## Caddy with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Caddy to serve Gitea under a sub-path by adding the following to your server block in your Caddyfile:

```apacheconf
git.example.com {
    route /git/* {
        uri strip_prefix /git
        reverse_proxy localhost:3000
    }
}
```

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.

## IIS

If you wish to run Gitea with IIS. You will need to setup IIS with URL Rewrite as reverse proxy.

1. Setup an empty website in IIS, named let's say, `Gitea Proxy`.
2. Follow the first two steps in [Microsoft's Technical Community Guide to Setup IIS with URL Rewrite](https://techcommunity.microsoft.com/t5/iis-support-blog/setup-iis-with-url-rewrite-as-a-reverse-proxy-for-real-world/ba-p/846222#M343). That is:

- Install Application Request Routing (ARR for short) either by using the Microsoft Web Platform Installer 5.1 (WebPI) or downloading the extension from [IIS.net](https://www.iis.net/downloads/microsoft/application-request-routing)
- Once the module is installed in IIS, you will see a new Icon in the IIS Administration Console called URL Rewrite.
- Open the IIS Manager Console and click on the `Gitea Proxy` Website from the tree view on the left. Select and double click the URL Rewrite Icon from the middle pane to load the URL Rewrite interface.
- Choose the `Add Rule` action from the right pane of the management console and select the `Reverse Proxy Rule` from the `Inbound and Outbound Rules` category.
- In the Inbound Rules section, set the server name to be the host that Gitea is running on with its port. e.g. if you are running Gitea on the localhost with port 3000, the following should work: `127.0.0.1:3000`
- Enable SSL Offloading
- In the Outbound Rules, ensure `Rewrite the domain names of the links in HTTP response` is set and set the `From:` field as above and the `To:` to your external hostname, say: `git.example.com`
- Now edit the `web.config` for your website to match the following: (changing `127.0.0.1:3000` and `git.example.com` as appropriate)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <system.web>
        <httpRuntime requestPathInvalidCharacters="" />
    </system.web>
    <system.webServer>
        <security>
          <requestFiltering>
            <hiddenSegments>
              <clear />
            </hiddenSegments>
            <denyUrlSequences>
              <clear />
            </denyUrlSequences>
            <fileExtensions allowUnlisted="true">
              <clear />
            </fileExtensions>
          </requestFiltering>
        </security>
        <rewrite>
            <rules useOriginalURLEncoding="false">
                <rule name="ReverseProxyInboundRule1" stopProcessing="true">
                    <match url="(.*)" />
                    <action type="Rewrite" url="http://127.0.0.1:3000{UNENCODED_URL}" />
                    <serverVariables>
                        <set name="HTTP_X_ORIGINAL_ACCEPT_ENCODING" value="HTTP_ACCEPT_ENCODING" />
                        <set name="HTTP_ACCEPT_ENCODING" value="" />
                    </serverVariables>
                </rule>
            </rules>
            <outboundRules>
                <rule name="ReverseProxyOutboundRule1" preCondition="ResponseIsHtml1">
                    <!-- set the pattern correctly here - if you only want to accept http or https -->
                    <!-- change the pattern and the action value as appropriate -->
                    <match filterByTags="A, Form, Img" pattern="^http(s)?://127.0.0.1:3000/(.*)" />
                    <action type="Rewrite" value="http{R:1}://git.example.com/{R:2}" />
                </rule>
                <rule name="RestoreAcceptEncoding" preCondition="NeedsRestoringAcceptEncoding">
                    <match serverVariable="HTTP_ACCEPT_ENCODING" pattern="^(.*)" />
                    <action type="Rewrite" value="{HTTP_X_ORIGINAL_ACCEPT_ENCODING}" />
                </rule>
                <preConditions>
                    <preCondition name="ResponseIsHtml1">
                        <add input="{RESPONSE_CONTENT_TYPE}" pattern="^text/html" />
                    </preCondition>
                    <preCondition name="NeedsRestoringAcceptEncoding">
                        <add input="{HTTP_X_ORIGINAL_ACCEPT_ENCODING}" pattern=".+" />
                    </preCondition>
                </preConditions>
            </outboundRules>
        </rewrite>
        <urlCompression doDynamicCompression="true" />
        <handlers>
          <clear />
          <add name="StaticFile" path="*" verb="*" modules="StaticFileModule,DefaultDocumentModule,DirectoryListingModule" resourceType="Either" requireAccess="Read" />
        </handlers>
        <!-- Map all extensions to the same MIME type, so all files can be
               downloaded. -->
        <staticContent>
          <clear />
          <mimeMap fileExtension="*" mimeType="application/octet-stream" />
        </staticContent>
    </system.webServer>
</configuration>
```

## HAProxy

If you want HAProxy to serve your Gitea instance, you can add the following to your HAProxy configuration

add an acl in the frontend section to redirect calls to gitea.example.com to the correct backend

```
frontend http-in
    ...
    acl acl_gitea hdr(host) -i gitea.example.com
    use_backend gitea if acl_gitea
    ...
```

add the previously defined backend section

```
backend gitea
    server localhost:3000 check
```

If you redirect the http content to https, the configuration work the same way, just remember that the connection between HAProxy and Gitea will be done via http so you do not have to enable https in Gitea's configuration.

## HAProxy with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup HAProxy to serve Gitea under a sub-path by adding the following to you HAProxy configuration:

```
frontend http-in
    ...
    acl acl_gitea path_beg /gitea
    use_backend gitea if acl_gitea
    ...
```

With that configuration http://example.com/gitea/ will redirect to your Gitea instance.

then for the backend section

```
backend gitea
    http-request replace-path /gitea\/?(.*) \/\1
    server localhost:3000 check
```

The added http-request will automatically add a trailing slash if needed and internally remove /gitea from the path to allow it to work correctly with Gitea by setting properly http://example.com/gitea as the root.

Then you **MUST** set something like `[server] ROOT_URL = http://example.com/gitea/` correctly in your configuration.

## Traefik

If you want traefik to serve your Gitea instance, you can add the following label section to your `docker-compose.yaml` (Assuming the provider is docker).

```yaml
gitea:
  image: gitea/gitea
  ...
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.gitea.rule=Host(`example.com`)"
    - "traefik.http.services.gitea-websecure.loadbalancer.server.port=3000"
```

This config assumes that you are handling HTTPS on the traefik side and using HTTP between Gitea and traefik.

## Traefik with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Traefik to serve Gitea under a sub-path by adding the following to your `docker-compose.yaml` (Assuming the provider is docker) :

```yaml
gitea:
  image: gitea/gitea
  ...
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.gitea.rule=Host(`example.com`) && PathPrefix(`/gitea`)"
    - "traefik.http.services.gitea-websecure.loadbalancer.server.port=3000"
    - "traefik.http.middlewares.gitea-stripprefix.stripprefix.prefixes=/gitea"
    - "traefik.http.routers.gitea.middlewares=gitea-stripprefix"
```

This config assumes that you are handling HTTPS on the traefik side and using HTTP between Gitea and traefik.

Then you **MUST** set something like `[server] ROOT_URL = http://example.com/gitea/` correctly in your configuration.

## General sub-path configuration

Usually it's not recommended to put Gitea in a sub-path, it's not widely used and may have some issues in rare cases.

If you really need to do so, to make Gitea works with sub-path (eg: `http://example.com/gitea/`), here are the requirements:

1. Set `[server] ROOT_URL = http://example.com/gitea/` in your `app.ini` file.
2. Make the reverse-proxy pass `http://example.com/gitea/foo` to `http://gitea-server:3000/foo`.
3. Make sure the reverse-proxy not decode the URI, the request `http://example.com/gitea/a%2Fb` should be passed as `http://gitea-server:3000/a%2Fb`.
