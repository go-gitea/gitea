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

1. Decode message and metadata signatures to hex, by running following commands, save output somewhere.

```sh
xxd -p package-1-1-x86_64.pkg.tar.zst.sig >> package-signature-hex
```

2. Paste your parameters and push package with [curl](https://curl.se/). Important, that time should be the same with metadata (signed md file), since this value is verified with GnuPG.

```sh
curl -X PUT \
  'https://{domain}/api/packages/{owner}/arch/push' \
  -H 'Authorization: {your-authorization-token}' \
  -H 'filename: package-1-1-x86_64.pkg.tar.zst' \
  -H 'distro: archlinux' \
  -H 'sign: {package-signature-hex}' \
  -H 'Content-Type: application/octet-stream' \
  --data-binary '@/path/to/package/file/package-1-1-x86_64.pkg.tar.zst'
```

## Delete packages

1. Send delete message with [curl](https://curl.se/). Time should be the same with saved in `md` file.

```sh
curl -X DELETE \
  http://localhost:3000/api/packages/{user}/arch/remove \
  -H 'Authorization: {your-authorization-token}' \
  -H "target: package" \
  -H "version: {version-release}"
```

## Clients

You can use gitea CLI tool to - [tea](https://gitea.com/gitea/tea) to push/remove arch packages from gitea. Alternatively, you can try [pack](https://fmnx.su/core/pack).
