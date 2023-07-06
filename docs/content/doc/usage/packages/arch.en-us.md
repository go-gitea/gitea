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

Gitea has arch package registry, which can act as a fully working [arch linux mirror](https://wiki.archlinux.org/title/mirrors) and connected directly in `/etc/pacman.conf`. Gitea automatically creates pacman database for packages in user space when new arch package is uploaded.

**Table of Contents**

{{< toc >}}

## Requirements

You can install packages in any environment with [pacman](https://wiki.archlinux.org/title/Pacman). Alternatively you can use [pack](https://fmnx.su/core/pack) which connects specified registries automatically and provides simple interface for package uploads and deletions.

## Install packages

First, you need to update your pacman configuration, adding following lines:

```conf
[{owner}.{domain}]
Server = https://{domain}/api/packages/{owner}/arch/{distribution}/{architecture}
```

Then, you can run pacman sync command (with -y flag to load connected database file), to install your package.

```sh
pacman -Sy package
```

## GPG Verification

Upload and remove operation are validated with [GnuPG](https://gnupg.org/). First, you need to export and upload your public gpg key to `SSH/GPG Keys` in account settings. This works similarly with SSH key. You can export gpg key with command:

```sh
gpg --armor --export
```

```
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBGSYoJUBCADSJ6v8Egst/gNJVC2206o8JqTzRBxTULKm/DH5J7AzrhJBxC2/
...

-----END PGP PUBLIC KEY BLOCK-----
```

## Upload packages

1. Ensure, that your package have been signed with your gpg key (more about arch package signing)[https://wiki.archlinux.org/title/DeveloperWiki:Package_signing]. You can do that by running following command:

```sh
gpg --verify package-ver-1-x86_64.pkg.tar.zst.sig
```

2. Sign message metadata, which consists of package owner (namespace in gitea), package file name and send time. You can do that by running following command:

```sh
echo -n {owner}{package}$(date --rfc-3339=seconds | tr " " T) >> md
gpg --detach-sign md
```

3. Decode message and metadata signatures to hex, by running following commands, save output somewhere.

```sh
xxd -p md.sig >> md.sig.hex
xxd -p package-1-1-x86_64.pkg.tar.zst.sig >> pkg.sig.hex
```

4. Paste your parameters and push package with [curl](https://curl.se/). Important, that time should be the same with metadata (signed md file), since this value is verified with GnuPG.

```sh
curl -X PUT \
  'https://{domain}/api/packages/{owner}/arch/push' \
  --header 'filename: {package}-1-1-x86_64.pkg.tar.zst' \
  --header 'email: dancheg97@fmnx.su' \
  --header 'distro: archlinux' \
  --header 'time: {metadata-time}' \
  --header 'pkgsign: {package-signature-hex}' \
  --header 'metasign: {metadata-signature-hex}' \
  --header 'Content-Type: application/octet-stream' \
  --data-binary '@/path/to/package/file/{package}-1-1-x86_64.pkg.tar.zst'
```

Full script for package upload:

```sh
owner=user
package=package-0.1.0-1-x86_64.pkg.tar.zst
email=user@example.com

time=`date --rfc-3339=seconds | tr " " T`
pkgsignhex=`xxd -p $package.sig | tr -d "\n"`

echo -n $owner$package$time >> mddata
gpg --detach-sign mddata
mdsignhex=`xxd -p mddata.sig | tr -d "\n"`

curl -X PUT \
  http://{domain}/api/packages/$owner/arch/push \
  --header "filename: $package" \
  --header "email: $email" \
  --header "time: $time" \
  --header "distro: archlinux" \
  --header "metasign: $mdsignhex" \
  --header "pkgsign: $pkgsignhex" \
  --header 'Content-Type: application/octet-stream' \
  --data-binary @$package
```

## Delete packages

1. Prepare signature for delete message.

```sh
echo -n {owner}{package}$(date --rfc-3339=seconds | tr " " T) >> md
gpg --detach-sign md
```

2. Send delete message with [curl](https://curl.se/). Time should be the same with saved in `md` file.

```sh
curl -X DELETE \
  http://localhost:3000/api/packages/{user}/arch/remove \
  --header "username: {user}" \
  --header "email: user@email.com" \
  --header "target: package" \
  --header "time: {rmtime}" \
  --header "version: {version-release}" \
  --header 'Content-Type: application/octet-stream' \
  --data-binary @md.sig
```

Full script for package deletion:

```sh
owner=user
package=package
version=0.1.0-1
email=user@example.com
arch=x86_64
time=`date --rfc-3339=seconds | tr " " T`

sudo rm -rf md md.sig
echo -n $owner$package$time >> md
gpg --detach-sign md

curl -X DELETE \
  http://{domain}/api/packages/$owner/arch/remove \
  --header "username: $owner" \
  --header "email: $email" \
  --header "target: $package" \
  --header "time: $time" \
  --header "version: $version" \
  --header 'Content-Type: application/octet-stream' \
  --data-binary @md.sig
```
