---
date: "2020-02-09T20:00:00+02:00"
title: "Installation with Docker (rootless)"
slug: "install-with-docker-rootless"
sidebar_position: 60
toc: false
draft: false
aliases:
  - /en-us/install-with-docker-rootless
menu:
  sidebar:
    parent: "installation"
    name: "With Docker Rootless"
    sidebar_position: 60
    identifier: "install-with-docker-rootless"
---

# Installation with Docker

Gitea provides automatically updated Docker images within its Docker Hub organization. It is
possible to always use the latest stable tag or to use another service that handles updating
Docker images.

The rootless image uses Gitea internal SSH to provide Git protocol and doesn't support OpenSSH.

This reference setup guides users through the setup based on `docker-compose`, but the installation
of `docker-compose` is out of scope of this documentation. To install `docker-compose` itself, follow
the official [install instructions](https://docs.docker.com/compose/install/).

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest-rootless`
image as a service. Since there is no database available, one can be initialized using SQLite3.

Create a directory for `data` and `config`:

```sh
mkdir -p gitea/{data,config}
cd gitea
touch docker-compose.yml
```

Then paste the following content into a file named `docker-compose.yml`:

```yaml
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

Note that the volume should be owned by the user/group with the UID/GID specified in the config file. By default Gitea in docker will use uid:1000 gid:1000. If needed you can set ownership on those folders with the command:

```sh
sudo chown 1000:1000 config/ data/
```

> If you don't give the volume correct permissions, the container may not start.

For a stable release you could use `:latest-rootless`, `:1-rootless` or specify a certain release like `:@version@-rootless`, but if you'd like to use the latest development version then `:nightly-rootless` would be an appropriate tag. If you'd like to run the latest commit from a release branch you can use the `:1.x-nightly-rootless` tag, where x is the minor version of Gitea. (e.g. `:1.16-nightly-rootless`)

## Custom port

To bind the integrated ssh and the webserver on a different port, adjust
the port section. It's common to just change the host port and keep the ports within
the container like they are.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-      - "3000:3000"
-      - "2222:2222"
+      - "80:3000"
+      - "22:2222"
```

## MySQL database

To start Gitea in combination with a MySQL database, apply these changes to the
`docker-compose.yml` file created above.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
+    environment:
+      - GITEA__database__DB_TYPE=mysql
+      - GITEA__database__HOST=db:3306
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
+    depends_on:
+      - db
+
+  db:
+    image: mysql:8
+    restart: always
+    environment:
+      - MYSQL_ROOT_PASSWORD=gitea
+      - MYSQL_USER=gitea
+      - MYSQL_PASSWORD=gitea
+      - MYSQL_DATABASE=gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

## PostgreSQL database

To start Gitea in combination with a PostgreSQL database, apply these changes to
the `docker-compose.yml` file created above.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    environment:
+      - GITEA__database__DB_TYPE=postgres
+      - GITEA__database__HOST=db:5432
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
+    depends_on:
+      - db
+
+  db:
+    image: postgres:14
+    restart: always
+    environment:
+      - POSTGRES_USER=gitea
+      - POSTGRES_PASSWORD=gitea
+      - POSTGRES_DB=gitea
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## Named volumes

To use named volumes instead of host volumes, define and use the named volume
within the `docker-compose.yml` configuration. This change will automatically
create the required volume. You don't need to worry about permissions with
named volumes; Docker will deal with that automatically.

```diff
version: "2"

+volumes:
+  gitea-data:
+    driver: local
+  gitea-config:
+    driver: local
+
services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
    volumes:
-      - ./data:/var/lib/gitea
+      - gitea-data:/var/lib/gitea
-      - ./config:/etc/gitea
+      - gitea-config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

MySQL or PostgreSQL containers will need to be created separately.

## Custom user

You can choose to use a custom user (following --user flag definition https://docs.docker.com/engine/reference/run/#user).
As an example to clone the host user `git` definition use the command `id -u git` and add it to `docker-compose.yml` file:
Please make sure that the mounted folders are writable by the user.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:@version@-rootless
    restart: always
+    user: 1001
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

## Start

To start this setup based on `docker-compose`, execute `docker-compose up -d`,
to launch Gitea in the background. Using `docker-compose ps` will show if Gitea
started properly. Logs can be viewed with `docker-compose logs`.

To shut down the setup, execute `docker-compose down`. This will stop
and kill the containers. The volumes will still exist.

Notice: if using a non-3000 port on http, change app.ini to match
`LOCAL_ROOT_URL = http://localhost:3000/`.

## Install

After starting the Docker setup via `docker-compose`, Gitea should be available using a
favorite browser to finalize the installation. Visit http://server-ip:3000 and follow the
installation wizard. If the database was started with the `docker-compose` setup as
documented above, please note that `db` must be used as the database hostname.

# Customization

Customization files described [here](administration/customizing-gitea.md) should
be placed in `/var/lib/gitea/custom` directory. If using host volumes, it's quite easy to access these
files; for named volumes, this is done through another container or by direct access at
`/var/lib/docker/volumes/gitea_gitea/_/var_lib_gitea`. The configuration file will be saved at
`/etc/gitea/app.ini` after the installation.

# Upgrading

:exclamation::exclamation: **Make sure you have volumed data to somewhere outside Docker container** :exclamation::exclamation:

To upgrade your installation to the latest release:

```
# Edit `docker-compose.yml` to update the version, if you have one specified
# Pull new images
docker-compose pull
# Start a new container, automatically removes old one
docker-compose up -d
```

# Upgrading from standard image

- Backup your setup
- Change volume mountpoint from /data to /var/lib/gitea
- If you used a custom app.ini move it to a new volume mounted to /etc/gitea
- Rename folder (inside volume) gitea to custom
- Edit app.ini if needed
  - Set START_SSH_SERVER = true
- Use image gitea/gitea:@version@-rootless

## Managing Deployments With Environment Variables

In addition to the environment variables above, any settings in `app.ini` can be set
or overridden with an environment variable of the form: `GITEA__SECTION_NAME__KEY_NAME`.
These settings are applied each time the docker container starts, and won't be passed into Gitea's sub-processes.
Full information [here](https://github.com/go-gitea/gitea/tree/main/contrib/environment-to-ini).

These environment variables can be passed to the docker container in `docker-compose.yml`.
The following example will enable a smtp mail server if the required env variables
`GITEA__mailer__FROM`, `GITEA__mailer__HOST`, `GITEA__mailer__PASSWD` are set on the host
or in a `.env` file in the same directory as `docker-compose.yml`.

The settings can be also set or overridden with the content of a file by defining an environment variable of the form:
`GITEA__section_name__KEY_NAME__FILE` that points to a file.

```bash
...
services:
  server:
    environment:
      - GITEA__mailer__ENABLED=true
      - GITEA__mailer__FROM=${GITEA__mailer__FROM:?GITEA__mailer__FROM not set}
      - GITEA__mailer__PROTOCOL=smtp
      - GITEA__mailer__HOST=${GITEA__mailer__HOST:?GITEA__mailer__HOST not set}
      - GITEA__mailer__IS_TLS_ENABLED=true
      - GITEA__mailer__USER=${GITEA__mailer__USER:-apikey}
      - GITEA__mailer__PASSWD="""${GITEA__mailer__PASSWD:?GITEA__mailer__PASSWD not set}"""
```

To set required TOKEN and SECRET values, consider using Gitea's built-in [generate utility functions](administration/command-line.md#generate).

# SSH Container Passthrough

Since SSH is running inside the container, SSH needs to be passed through from the host to the container if SSH support is desired. One option would be to run the container SSH on a non-standard port (or moving the host port to a non-standard port). Another option which might be more straightforward is to forward SSH commands from the host to the container. This setup is explained in the following.

This guide assumes that you have created a user on the host called `git` with permission to run `docker exec`, and that the Gitea container is called `gitea`. You will need to modify that user's shell to forward the commands to the `sh` executable inside the container, using `docker exec`.

First, create the file `/usr/local/bin/gitea-shell` on the host, with the following contents:

```bash
#!/bin/sh
/usr/bin/docker exec -i --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
```

Note that `gitea` in the docker command above is the name of the container. If you named yours differently, don't forget to change that.

You should also make sure that you’ve set the permissions of the shell wrapper correctly:

```bash
sudo chmod +x /usr/local/bin/gitea-shell
```

Once the wrapper is in place, you can make it the shell for the `git` user:

```bash
sudo usermod -s /usr/local/bin/gitea-shell git
```

Now that all the SSH commands are forwarded to the container, you need to set up the SSH authentication on the host. This is done by leveraging the [SSH AuthorizedKeysCommand](administration/command-line.md#keys) to match the keys against those accepted by Gitea. Add the following block to `/etc/ssh/sshd_config`, on the host:

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/docker exec -i gitea /usr/local/bin/gitea keys -c /etc/gitea/app.ini -e git -u %u -t %t -k %k
```

(From 1.16.0 you will not need to set the `-c /etc/gitea/app.ini` option.)

All that is left to do is restart the SSH server:

```bash
sudo systemctl restart sshd
```

**Notes**

This isn't actually using the docker SSH - it is simply using the commands around it.
You could theoretically not run the internal SSH server.
