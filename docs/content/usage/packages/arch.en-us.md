---
date: "2016-11-08T16:00:00+02:00"
title: "Arch Package Registry"
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

Gitea has a Arch Linux package registry, which can act as a fully working [Arch linux mirror](https://wiki.archlinux.org/title/mirrors) and connected directly in `/etc/pacman.conf`. Gitea automatically creates pacman database for packages in user/organization space when a new Arch package is uploaded.

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

When uploading package to gitea, you have to prepare package file with `.pkg.tar.zst` extension and it's `.pkg.tar.zst.sig` signature. You can use [curl](https://curl.se/) or any other HTTP client, gitea supports multiple [authentication schemes](https://docs.gitea.com/usage/authentication). Upload command will create 3 files: package, signature and desc file for pacman database (which will be created automatically on request).

Following command will upload arch package and related signature to gitea. Example with basic auth:

```sh
curl -X PUT \
  https://{domain}/api/packages/{owner}/arch/push/{package-1-1-x86_64.pkg.tar.zst}/{archlinux}/$(xxd -p package-1-1-x86_64.pkg.tar.zst.sig | tr -d '\n') \
  --user your_username:your_token_or_password \
  --header "Content-Type: application/octet-stream" \
  --data-binary '@/path/to/package/file/package-1-1-x86_64.pkg.tar.zst'
```

## Delete packages

Delete operation will remove specific package version, and all package files related to that version.

```sh
curl -X DELETE \
  https://{domain}/api/packages/{user}/arch/remove/{package}/{version} \
  --user your_username:your_token_or_password
```

## Clients

Any `pacman` compatible package manager/AUR-helper can be used to install packages from gitea ([yay](https://github.com/Jguer/yay), [paru](https://github.com/Morganamilo/paru), [pikaur](https://github.com/actionless/pikaur), [aura](https://github.com/fosskers/aura)). Alternatively, you can try [pack](https://fmnx.su/core/pack) which supports full gitea API (install/push/remove). Also, any HTTP client can be used to execute get/push/remove operations ([curl](https://curl.se/), [postman](https://www.postman.com/), [thunder-client](https://www.thunderclient.com/)).
