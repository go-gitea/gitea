---
date: "2017-08-23T09:00:00+02:00"
title: "Installation avec le binaire pré-compilé"
slug: "install-from-binary"
weight: 15
toc: false
draft: false
aliases:
  - /fr-fr/install-from-binary
menu:
  sidebar:
    parent: "installation"
    name: "Binaire pré-compilé"
    weight: 15
    identifier: "install-from-binary"
---

# Installation avec le binaire pré-compilé

Tous les binaires sont livrés avec le support de SQLite, MySQL et PostgreSQL, et sont construits avec les ressources incorporées. Gardez à l'esprit que cela peut être différent pour les versions antérieures. L'installation basée sur nos binaires est assez simple, il suffit de choisir le fichier correspondant à votre plateforme à partir de la [page de téléchargement](https://dl.gitea.com/gitea). Copiez l'URL et remplacer l'URL dans les commandes suivantes par la nouvelle:

```
wget -O gitea https://dl.gitea.com/gitea/{{< version >}}/gitea-{{< version >}}-linux-amd64
chmod +x gitea
```

## Test

Après avoir suivi les étapes ci-dessus, vous aurez un binaire `gitea` dans votre répertoire de travail. En premier lieu, vous pouvez tester si le binaire fonctionne comme prévu et ensuite vous pouvez le copier vers la destination où vous souhaitez le stocker. Lorsque vous lancez Gitea manuellement à partir de votre CLI, vous pouvez toujours le tuer en appuyant sur `Ctrl + C`.

```
./gitea web
```

## Dépannage

### Anciennes version de glibc

Les anciennes distributions Linux (comme Debian 7 ou CentOS 6) peuvent ne pas être capable d'exécuter le binaire Gitea, résultant généralement une erreur du type ```./gitea: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.14' not found (required by ./gitea)```. Cette erreur est due au driver SQLite que nous intégrons dans le binaire Gitea. Dans le futur, nous fournirons des binaires sans la dépendance pour la bibliothèque glibc. En attendant, vous pouvez mettre à jour votre distribution ou installer Gitea depuis le [code source]({{< relref "doc/installation/from-source.fr-fr.md" >}}).

### Exécuter Gitea avec un autre port

Si vous obtenez l'erreur `702 runWeb()] [E] Failed to start server: listen tcp 0.0.0.0:3000: bind: address already in use`, Gitea à besoin d'utiliser un autre port. Vous pouvez changer le port par défaut en utilisant `./gitea web -p $PORT`.

## Il manque quelque chose ?

Est-ce que nous avons oublié quelque chose sur cette page ? N'hésitez pas à nous contacter sur notre [serveur Discord](https://discord.gg/Gitea), vous obtiendrez des réponses à toute vos questions assez rapidement.
