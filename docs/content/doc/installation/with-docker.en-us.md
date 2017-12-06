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

We provide automatically updated Docker images within our Docker Hub organization. It is up to you and your deployment to always use the latest stable tag or to use another service that updates the Docker image for you.

This reference setup guides you through the setup based on `docker-compose`, the installation of `docker-compose` is out of scope of this documentation. To install `docker-compose` follow the official [install instructions](https://docs.docker.com/compose/install/).

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest` image as a service. Since there is no database available you can start it only with SQLite3. Create a directory like `gitea` and paste the following content into a file named `docker-compose.yml`.

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

To bind the integrated openSSH daemon and the webserver on a different port, you just need to adjust the port section. It's common to just change the host port and keep the ports within the container like they are.

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

To start Gitea in combination with a MySQL database you should apply these changes to the `docker-compose.yml` file created above.

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

To start Gitea in combination with a PostgreSQL database you should apply these changes to the `docker-compose.yml` file created above.

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

To use named volumes instead of host volumes you just have to define and use the named volume within the `docker-compose.yml` configuration. This change will automatically create the required volume.

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

If you are using MySQL or PostgreSQL it's up to you to create named volumes for these containers as well.

## Start

To start this setup based on `docker-compose` you just have to execute `docker-compose up -d` to launch Gitea in the background. You can see if it started properly via `docker-compose ps`, and you can tail the log output via `docker-compose logs`.

If you want to shutdown the setup again just execute `docker-compose down`, this will stop and kill the containers, the volumes will still exist.

Notice: if you use a non 3000 port on http, you need change app.ini `LOCAL_ROOT_URL  = http://localhost:3000/`.

## Install

After starting the Docker setup via `docker-compose` you should access Gitea with your favorite browser to finalize the installation. Please visit http://server-ip:3000 and follow the installation wizard. If you have started a database with the `docker-compose` setup as documented above please note that you have to use `db` as the database hostname.

# Customization

Customization files described [here](https://docs.gitea.io/en-us/customizing-gitea/) should be placed in `/data/gitea` directory. If you are using host volumes it's quite easy to access these files, for named volumes you have to do it through another container or you should directly access `/var/lib/docker/volumes/gitea_gitea/_data`. The configuration file will be saved at `/data/gitea/conf/app.ini` after the installation.

# Anything missing?

Are we missing anything on this page? Then feel free to reach out to us on our [Discord server](https://discord.gg/NsatcWJ), there you will get answers to any question pretty fast.
