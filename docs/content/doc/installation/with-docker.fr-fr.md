---
date: "2017-08-23T09:00:00+02:00"
title: "Installation avec Docker"
slug: "install-with-docker"
weight: 70
toc: false
draft: false
aliases:
  - /fr-fr/install-with-docker
menu:
  sidebar:
    parent: "installation"
    name: "Docker"
    weight: 70
    identifier: "install-with-docker"
---

# Installation avec Docker

Nous fournissons des images Docker mises à jour automatiquement via le Docker Hub de notre organisation. C'est à vous, lors devotre déploiement, de vous assurez d'utiliser toujours la dernière version stable ou d'utiliser un autre service qui met à jour l'image Docker pour vous.

{{< toc >}}

## Données stockées sur l'hôte

Tout d'abord, vous devez simplement récupérer l'image Docker avec la commande suivante :

```
docker pull gitea/gitea:latest
```

Pour garder vos dépôts et certaines autres données persistantes, vous devez créer un répertoire qui contiendra ces données à l'avenir.

```
sudo mkdir -p /var/lib/gitea
```

Il est temps de démarrer votre instance Docker, c'est un processus assez simple. Vous avez à définir le mappage des ports et le volume à utiliser pour la persistance de vos données :

```
docker run -d --name=gitea -p 10022:22 -p 10080:3000 -v /var/lib/gitea:/data gitea/gitea:latest
```

Vous devriez avoir une instance fonctionnelle de Gitea. Pour accèder à l'interface web, visitez l'adresse http://hostname:10080 avec votre navigateur web préféré. Si vous voulez clôner un dépôt, vous pouvez le faire avec la commande  `git clone ssh://git@hostname:10022/username/repo.git`.

## Named Volumes

Ce guide aboutira à une installation avec les données Gitea et PostgreSQL stockées dans des volumes nommés. Cela permet une sauvegarde, une restauration et des mises à niveau en toute simplicité.

### The Database

Création du volume nommé pour la base de données :

```
$ docker volume create --name gitea-db-data
```

Une fois votre volume pret, vous pouvez récupérer l'image Docker de PostgreSQL et créer une instance. Tout comme Gitea, c'est également une image Docker basée sur Alpine Linux, Le montage des données se fera sans aucun problème.

```
$ docker pull postgres:alpine
$ docker run -d --name gitea-db \
    -e POSTGRES_PASSWORD=<PASSWORD> \
    -v gitea-db-data:/var/lib/postgresql/data \
    -p 5432:5432 \
    postgres:alpine
```

Maintenant que la base de données est démarrée, il faut la configurer. N'oubliez pas le mot de passe que vous avez choisi, vous en aurez besoin lors de l'installation de Gitea.

```
$ docker exec -it gitea-db psql -U postgres
psql (9.6.1)
Type "help" for help.

postgres=# CREATE USER gitea WITH PASSWORD '<PASSWORD>';
CREATE ROLE
postgres=# CREATE DATABASE gitea OWNER gitea;
CREATE DATABASE
postgres=# \q
$
```

### Gitea

Premièrement, le volume nommé :

```
$ docker volume create --name gitea-data
```

Puis l'instance de Gitea :

```
$ docker run -d --name gitea \
	--link gitea-db:gitea-db \
	--dns 10.12.10.160 \
	-p 11180:3000 \
	-p 8322:22 \
	-v gitea-data:/data \
	gitea/gitea:latest
```

Vous devriez maintenant avoir deux conteneurs Docker pour Gitea et PostgreSQL plus deux volumes nommés Docker.

# Personnalisation

Les fichier personnalisés ([voir les instructions](https://docs.gitea.io/en-us/customizing-gitea/)) peuvent être placés dans le répertoire `/data/gitea`.

Le fichier de configuration sera sauvegardé à l'emplacement suivant : `/data/gitea/conf/app.ini`

## Il manque quelque chose ?

Est-ce que nous avons oublié quelque chose sur cette page ? N'hésitez pas à nous contacter sur notre [serveur Discord](https://discord.gg/Gitea), vous obtiendrez des réponses à toute vos questions assez rapidement.
