---
date: "2016-12-01T16:00:00+02:00"
title: "Installation with Docker"
slug: "install-with-docker"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "With Docker"
    weight: 10
    identifier: "install-with-docker"
---

# Installation with Docker

Gitea provides automatically updated Docker images within its Docker Hub organization. It is
possible to always use the latest stable tag or to use another service that handles updating
Docker images.

This reference setup guides users through the setup based on `docker-compose`, the installation
of `docker-compose` is out of scope of this documentation. To install `docker-compose` follow
the official [install instructions](https://docs.docker.com/compose/install/).

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest`
image as a service. Since there is no database available one can be initialized using SQLite3.
Create a directory like `gitea` and paste the following content into a file named `docker-compose.yml`.

```yaml
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
    ports:
      - "3000:3000"
      - "222:22"
```

## Custom port

To bind the integrated openSSH daemon and the webserver on a different port, adjust
the port section. It's common to just change the host port and keep the ports within
the container like they are.

```diff
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
    ports:
-      - "3000:3000"
-      - "222:22"
+      - "8080:3000"
+      - "2221:22"
```

## MySQL database

To start Gitea in combination with a MySQL database, apply these changes to the
`docker-compose.yml` file created above.

```diff
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
     ports:
       - "3000:3000"
       - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: mysql:5.7
+    restart: always
+    environment:
+      - MYSQL_ROOT_PASSWORD=gitea
+      - MYSQL_USER=gitea
+      - MYSQL_PASSWORD=gitea
+      - MYSQL_DATABASE=gitea
+    networks:
+      - gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

## PostgreSQL database

To start Gitea in combination with a PostgreSQL database, apply these changes to
the `docker-compose.yml` file created above.

```diff
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
     ports:
       - "3000:3000"
       - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: postgres:9.6
+    restart: always
+    environment:
+      - POSTGRES_USER=gitea
+      - POSTGRES_PASSWORD=gitea
+      - POSTGRES_DB=gitea
+    networks:
+      - gitea
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## Named volumes

To use named volumes instead of host volumes, define and use the named volume
within the `docker-compose.yml` configuration. This change will automatically
create the required volume.

```diff
version: "2"

networks:
  gitea:
    external: false

+volumes:
+  gitea:
+    driver: local
+
services:
  server:
    image: gitea/gitea:latest
    restart: always
    networks:
      - gitea
    volumes:
-      - ./gitea:/data
+      - gitea:/data
    ports:
      - "3000:3000"
      - "222:22"
```

MySQL or PostgreSQL containers will need to be created separately.

## Start

To start this setup based on `docker-compose`, execute `docker-compose up -d`,
to launch Gitea in the background. Using `docker-compose ps` will show if Gitea
started properly. Logs can be viewed with `docker-compose logs`.

To shut down the setup, execute `docker-compose down`. This will stop
and kill the containers. The volumes will still exist.

Notice: if using a non-3000 port on http, change app.ini to match
`LOCAL_ROOT_URL = http://localhost:3000/`.

## Install

After starting the Docker setup via `docker-compose` Gitea should be available using a
favorite browser to finalize the installation. Visit http://server-ip:3000 and follow the
installation wizard. If the database was started with the `docker-compose` setup as
documented above please note that `db` must be used as the database hostname.

# Customization

Customization files described [here](https://docs.gitea.io/en-us/customizing-gitea/) should
be placed in `/data/gitea` directory. If using host volumes it's quite easy to access these
files; for named volumes this is done through another container or by direct access at
`/var/lib/docker/volumes/gitea_gitea/_data`. The configuration file will be saved at
`/data/gitea/conf/app.ini` after the installation.
