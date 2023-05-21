---
date: "2016-12-01T16:00:00+02:00"
title: "Installation depuis le gestionnaire de paquets"
slug: "install-from-package"
weight: 20
toc: false
draft: false
aliases:
  - /fr-fr/install-from-package
menu:
  sidebar:
    parent: "installation"
    name: "Gestionnaire de paquets"
    weight: 20
    identifier: "install-from-package"
---

# Installation depuis le gestionnaire de paquets

## Linux

Nous n'avons pas encore publié de paquet pour Linux, nous allons mettre à jour cette page directement lorsque nous commencerons à publier des paquets pour toutes distributions Linux. En attendant, vous devriez suivre les [instructions d'installation]({{< relref "from-binary.fr-fr.md" >}}) avec le binaire pré-compilé.

## Windows

Nous n'avons pas encore publié de paquet pour Windows, nous allons mettre à jour cette page directement lorsque nous commencerons à publier des paquets sous la forme de fichiers `MSI` ou via [Chocolatey](https://chocolatey.org/). En attendant, vous devriez suivre les [instructions d'installation]({{< relref "from-binary.fr-fr.md" >}}) avec le binaire pré-compilé.

## macOS

Actuellement, nous ne supportons que l'installation via `brew` pour macOS. Si vous n'utilisez pas [Homebrew](http://brew.sh/), vous pouvez suivre les [instructions d'installation]({{< relref "from-binary.fr-fr.md" >}}) avec le binaire pré-compilé. Pour installer Gitea depuis `brew`, utilisez les commandes suivantes :

```
brew tap go-gitea/gitea
brew install gitea
```

## FreeBSD

Le portage FreeBSD `www/gitea` est disponible.  Vous pouvez également installer le paquet pré-compilé avec la commande suivante:

```
pkg install gitea
```

Pour une version plus récente, ou pour les instructions de compilations, veuillez consulter la documentation officielle de FreeBSD : [install it from the port](https://www.freebsd.org/doc/handbook/ports-using.html)

```
su -
cd /usr/ports/www/gitea
make install clean
```

Le port utilise la schéma standard du système de fichiers FreeBSD : Les fichiers de configuration sont localisés dans le répertoire `/usr/local/etc/gitea`, les modèles, options, plugins et thèmes sont localisés dans le répertoire `/usr/local/share/gitea`, et le script de démarrage se situe dans `/usr/local/etc/rc.d/gitea`.

Pour exécuter Gitea en tant que service, utilisez la commande `sysrc gitea_enable=YES` et la commande `service gitea start` pour démarrer le service.

## Il manque quelque chose ?

Est-ce que nous avons oublié quelque chose sur cette page ? N'hésitez pas à nous contacter sur notre [serveur Discord](https://discord.gg/Gitea), vous obtiendrez des réponses à toute vos questions assez rapidement.
