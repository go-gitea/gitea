---
date: "2016-11-08T16:00:00+02:00"
title: "Title"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "packages"
    name: "Arch"
    weight: 10
    identifier: "arch"
---

# Arch package registry

Gitea has arch package registry, which can act as a fully working [arch linux mirror](https://wiki.archlinux.org/title/mirrors) and connected directly in `/etc/pacman.conf`. Gitea automatically creates pacman database for packages in user/organization space when new arch package is uploaded.

**Table of Contents**

{{< toc >}}

## Install packages

First, you need to update your pacman configuration, adding following lines:

```conf
[{owner}.{domain}]
SigLevel = Optional TrustAll
Server = https://{domain}/api/packages/{owner}/arch/{distribution}/{architecture}
```

Then, you can run pacman sync command (with -y flag to load connected database file), to install your package.

```sh
pacman -Sy package
```

## Upload packages

Get into folder with package and signature, push package with [curl](https://curl.se/).

```sh
curl -X PUT \
  'https://{domain}/api/packages/{owner}/arch/push' \
  --header "Authorization: {your-authorization-token}" \
  --header "filename: package-1-1-x86_64.pkg.tar.zst" \
  --header "distro: archlinux" \
  --header "sign: $(xxd -p package-1-1-x86_64.pkg.tar.zst.sig | tr -d '\n')"
  --header "Content-Type: application/octet-stream" \
  --data-binary '@/path/to/package/file/package-1-1-x86_64.pkg.tar.zst'
```

## Delete packages

Send delete message with [curl](https://curl.se/).

```sh
curl -X DELETE \
  http://localhost:3000/api/packages/{user}/arch/remove \
  --header "Authorization: {your-authorization-token}" \
  --header "target: package" \
  --header "version: {version-release}"
```

## Clients

You can use gitea CLI tool to - [tea](https://gitea.com/gitea/tea) to push/remove arch packages from gitea. Alternatively, you can try [pack](https://fmnx.su/core/pack).
