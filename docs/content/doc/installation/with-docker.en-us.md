---
date: "2020-03-19T19:27:00+02:00"
title: "Installation with Docker"
slug: "install-with-docker"
weight: 10
toc: false
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

This reference setup guides users through the setup based on `docker-compose`, but the installation
of `docker-compose` is out of scope of this documentation. To install `docker-compose` itself, follow
the official [install instructions](https://docs.docker.com/compose/install/).

**Table of Contents**

{{< toc >}}

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest`
image as a service. Since there is no database available, one can be initialized using SQLite3.
Create a directory like `gitea` and paste the following content into a file named `docker-compose.yml`.
Note that the volume should be owned by the user/group with the UID/GID specified in the config file.
If you don't give the volume correct permissions, the container may not start.
For a stable release you can use `:latest`, `:1` or specify a certain release like `:{{< version >}}`, but if you'd like to use the latest development version of Gitea then you could use the `:dev` tag. If you'd like to run the latest commit from a release branch you can use the `:1.x-dev` tag, where x is the minor version of Gitea. (e.g. `:1.16-dev`)

```yaml
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

## Ports

To bind the integrated OpenSSH daemon and the webserver on a different port, adjust
the port section. It's common to just change the host port and keep the ports within
the container like they are.

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-     - "3000:3000"
-     - "222:22"
+     - "8080:3000"
+     - "2221:22"
```

## Databases

### MySQL database

To start Gitea in combination with a MySQL database, apply these changes to the
`docker-compose.yml` file created above.

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
+     - GITEA__database__DB_TYPE=mysql
+     - GITEA__database__HOST=db:3306
+     - GITEA__database__NAME=gitea
+     - GITEA__database__USER=gitea
+     - GITEA__database__PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
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
+    networks:
+      - gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

### PostgreSQL database

To start Gitea in combination with a PostgreSQL database, apply these changes to
the `docker-compose.yml` file created above.

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
+     - GITEA__database__DB_TYPE=postgres
+     - GITEA__database__HOST=db:5432
+     - GITEA__database__NAME=gitea
+     - GITEA__database__USER=gitea
+     - GITEA__database__PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: postgres:13
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
create the required volume. You don't need to worry about permissions with
named volumes; Docker will deal with that automatically.

```diff
version: "3"

networks:
  gitea:
    external: false

+volumes:
+  gitea:
+    driver: local
+
services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    restart: always
    networks:
      - gitea
    volumes:
-     - ./gitea:/data
+     - gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

MySQL or PostgreSQL containers will need to be created separately.

## Startup

To start this setup based on `docker-compose`, execute `docker-compose up -d`,
to launch Gitea in the background. Using `docker-compose ps` will show if Gitea
started properly. Logs can be viewed with `docker-compose logs`.

To shut down the setup, execute `docker-compose down`. This will stop
and kill the containers. The volumes will still exist.

Notice: if using a non-3000 port on http, change app.ini to match
`LOCAL_ROOT_URL = http://localhost:3000/`.

## Installation

After starting the Docker setup via `docker-compose`, Gitea should be available using a
favorite browser to finalize the installation. Visit http://server-ip:3000 and follow the
installation wizard. If the database was started with the `docker-compose` setup as
documented above, please note that `db` must be used as the database hostname.

## Configure the user inside Gitea using environment variables 

- `USER`: **git**: The username of the user that runs Gitea within the container.
- `USER_UID`: **1000**: The UID (Unix user ID) of the user that runs Gitea within the container. Match this to the UID of the owner of the `/data` volume if using host volumes (this is not necessary with named volumes).
- `USER_GID`: **1000**: The GID (Unix group ID) of the user that runs Gitea within the container. Match this to the GID of the owner of the `/data` volume if using host volumes (this is not necessary with named volumes).

## Customization

Customization files described [here](https://docs.gitea.io/en-us/customizing-gitea/) should
be placed in `/data/gitea` directory. If using host volumes, it's quite easy to access these
files; for named volumes, this is done through another container or by direct access at
`/var/lib/docker/volumes/gitea_gitea/_data`. The configuration file will be saved at
`/data/gitea/conf/app.ini` after the installation.

## Upgrading

:exclamation::exclamation: **Make sure you have volumed data to somewhere outside Docker container** :exclamation::exclamation:

To upgrade your installation to the latest release:

```bash
# Edit `docker-compose.yml` to update the version, if you have one specified
# Pull new images
docker-compose pull
# Start a new container, automatically removes old one
docker-compose up -d
```

## Managing Deployments With Environment Variables

In addition to the environment variables above, any settings in `app.ini` can be set or overridden with an environment variable of the form: `GITEA__SECTION_NAME__KEY_NAME`. These settings are applied each time the docker container starts. Full information [here](https://github.com/go-gitea/gitea/tree/master/contrib/environment-to-ini).

These environment variables can be passed to the docker container in `docker-compose.yml`. The following example will enable an smtp mail server if the required env variables `GITEA__mailer__FROM`, `GITEA__mailer__HOST`, `GITEA__mailer__PASSWD` are set on the host or in a `.env` file in the same directory as `docker-compose.yml`:

```bash
...
services:
  server:
    environment:
    - GITEA__mailer__ENABLED=true
    - GITEA__mailer__FROM=${GITEA__mailer__FROM:?GITEA__mailer__FROM not set}
    - GITEA__mailer__MAILER_TYPE=smtp
    - GITEA__mailer__HOST=${GITEA__mailer__HOST:?GITEA__mailer__HOST not set}
    - GITEA__mailer__IS_TLS_ENABLED=true
    - GITEA__mailer__USER=${GITEA__mailer__USER:-apikey}
    - GITEA__mailer__PASSWD="""${GITEA__mailer__PASSWD:?GITEA__mailer__PASSWD not set}"""
```

To set required TOKEN and SECRET values, consider using Gitea's built-in [generate utility functions](https://docs.gitea.io/en-us/command-line/#generate).

## SSH Container Passthrough

Since SSH is running inside the container, SSH needs to be passed through from the host to the container if SSH support is desired. One option would be to run the container SSH on a non-standard port (or moving the host port to a non-standard port). Another option which might be more straightforward is to forward SSH connections from the host to the container.

There are multiple ways of doing this - however, all of these require some information about the docker being passed to the host.

### SSHing Shim (with authorized_keys)

The idea of this option is to use (essentially unchanged) the authorized_keys that gitea creates on the docker and simply shim the gitea binary the docker would use on the host to instead ssh into the docker ssh.

- To make the forwarding work, the SSH port of the container (22) needs to be mapped to the host port 2222 in `docker-compose.yml` . Since this port does not need to be exposed to the outside world, it can be mapped to the `localhost` of the host machine:

  ```yaml
  ports:
    # [...]
    - "127.0.0.1:2222:22"
  ```

- Next on the host create the `git` user which shares the same `UID`/ `GID` as the container values `USER_UID`/ `USER_GID`. These values can be set as environment variables in the `docker-compose.yml`:

  ```yaml
  environment:
    - USER_UID=1000
    - USER_GID=1000
  ```

- Mount `/home/git/.ssh` of the host into the container. Otherwise the SSH authentication cannot work inside the container.

  ```yaml
  volumes:
    - /home/git/.ssh/:/data/git/.ssh
  ```

- Now a SSH key pair needs to be created on the host. This key pair will be used to authenticate the `git` user on the host to the container.

  ```bash
  sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
  ```

- Please note depending on the local version of ssh you may want to consider using `-t ecdsa` here.

- `/home/git/.ssh/authorized_keys` on the host now needs to be modified. It needs to act in the same way as `authorized_keys` within the Gitea container. Therefore add the public key of the key you created above ("Gitea Host Key") to `~/git/.ssh/authorized_keys`.

  ```bash
  echo "$(cat /home/git/.ssh/id_rsa.pub)" >> /home/git/.ssh/authorized_keys
  ```

  Important: The pubkey from the `git` user needs to be added "as is" while all other pubkeys added via the Gitea web interface will be prefixed with `command="/usr [...]`.

  `/home/git/.ssh/authorized_keys` should then look somewhat like

  ```bash
  # SSH pubkey from git user
  ssh-rsa <Gitea Host Key>

  # other keys from users
  command="/usr/local/bin/gitea --config=/data/gitea/conf/app.ini serv key-1",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <user pubkey>
  ```

- The next step is to create the file that will issue the SSH forwarding from the host to the container. The name of this file depends on your version of Gitea:

  - For Gitea v1.16.0+:

    ```bash
    cat <<"EOF" | sudo tee /usr/local/bin/gitea
    #!/bin/sh
    ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
    EOF
    chmod +x /usr/local/bin/gitea
    ```

  - For Gitea v1.15.x and earlier

    ```bash
    cat <<"EOF" | sudo tee /app/gitea/gitea
    #!/bin/sh
    ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
    EOF
    sudo chmod +x /app/gitea/gitea
    ```

Here is a detailed explanation what is happening when a SSH request is made:

1. A SSH request is made against the host (usually port 22) using the `git` user, e.g. `git clone git@domain:user/repo.git`.
2. In `/home/git/.ssh/authorized_keys` , the command executes the `/usr/local/bin/gitea` script.
3. `/usr/local/bin/gitea` forwards the SSH request to port 2222 which is mapped to the SSH port (ssh 22) of the container.
4. Due to the existence of the public key of the `git` user in `/home/git/.ssh/authorized_keys` the authentication host → container succeeds and the SSH request get forwarded to Gitea running in the docker container.

If a new SSH key is added in the Gitea web interface, it will be appended to `.ssh/authorized_keys` in the same way as the already existing key.

**Notes**

SSH container passthrough using `authorized_keys` will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed (depending on the changes)

### SSHing Shell (with authorized_keys)

The idea of this option is to use (essentially unchanged) the authorized_keys that gitea creates on the docker and use a special shell for git user that uses ssh to shell to the docker git user.

- In this case we setup as above except instead of creating `/usr/local/bin/gitea` or `/app/gitea/gitea`
we create a new shell for the git user:

  ```bash
  cat <<"EOF" | sudo tee /home/git/ssh-shell
  #!/bin/sh
  shift
  ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $@"
  EOF
  sudo chmod +x /home/git/ssh-shell
  sudo usermod -s /home/git/ssh-shell git
  ```

  Be careful here - if you try to login as the git user in future you will ssh directly to the docker.

Here is a detailed explanation what is happening when a SSH request is made:

1. A SSH request is made against the host (usually port 22) using the `git` user, e.g. `git clone git@domain:user/repo.git`.
2. In `/home/git/.ssh/authorized_keys` , the command in the command portion is passed to the `ssh-shell` script
3. `ssh-shell` forwards the SSH request to port 2222 overriding whi is mapped to the SSH port (ssh 22) of the container.
4. Due to the existence of the public key of the `git` user in `/home/git/.ssh/authorized_keys` the authentication host → container succeeds and the SSH request get forwarded to Gitea running in the docker container.

If a new SSH key is added in the Gitea web interface, it will be appended to `.ssh/authorized_keys` in the same way as the already existing key.

**Notes**

SSH container passthrough using `authorized_keys` will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed (depending on the changes)

### Docker Shell (with authorized_keys)

Similar to the above ssh shell technique we can use a shell which simply uses `docker exec`:

```bash
cat <<"EOF" | sudo tee /home/git/docker-shell
#!/bin/sh
/usr/bin/docker exec -i --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
EOF
sudo chmod +x /home/git/docker-shell
sudo usermod -s /home/git/docker-shell git
```

Note that `gitea` in the docker command above is the name of the container. If you named yours differently, don't forget to change that. The `git` user also have to have
permission to run `docker exec`.

**Notes**

Docker shell passthrough using `authorized_keys` will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed (depending on the changes)

A Docker execing shim could be created similarly to above.

### Docker Shell with AuthorizedKeysCommand

The AuthorizedKeysCommand route provides another option that does not require many changes to the compose file or the `authorized_keys` - but does require changes to the host `/etc/sshd_config`.

- On the host create called `git` with permission to run `docker exec`.
- We will again assume that the Gitea container is called `gitea`.
- Modify the `git` user's shell to forward commands to the `sh` executable inside the container using `docker exec` as previously described:

  ```bash
  cat <<"EOF" | sudo tee /home/git/docker-shell
  #!/bin/sh
  /usr/bin/docker exec -i --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
  EOF
  sudo chmod +x /home/git/docker-shell
  sudo usermod -s /home/git/docker-shell git
  ```

Now all attempts to login as the `git` user will be forwarded to the docker - including the `SSH_ORIGINAL_COMMAND`. We now need to set-up SSH authenitication on the host.

We will do this by leveraging the [SSH AuthorizedKeysCommand](https://docs.gitea.io/en-us/command-line/#keys) to match the keys against those accepted by Gitea. 

Add the following block to `/etc/ssh/sshd_config`, on the host:

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/docker exec -i gitea /usr/local/bin/gitea keys -c /data/gitea/conf/app.ini -e git -u %u -t %t -k %k
```

(From 1.16.0 you will not need to set the `-c /data/gitea/conf/app.ini` option.)

Finally restart the SSH server:

```bash
sudo systemctl restart sshd
```

**Notes**

Docker shell passthrough using `AuthorizedKeysCommand` will work only if

- The host `git` user is allowed to run the `docker exec` command.

A Docker execing shim could be created similarly to above.

### SSH Shell with AuthorizedKeysCommand

Create a key for the host `git` user as above, add it to the docker `/data/git/.ssh/authorized_keys` then finally create and set the `ssh-shell` as above.

Add the following block to `/etc/ssh/sshd_config`, on the host:

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 /usr/local/bin/gitea keys -c /data/gitea/conf/app.ini -e git -u %u -t %t -k %k
```

(From 1.16.0 you will not need to set the `-c /data/gitea/conf/app.ini` option.)

Finally restart the SSH server:

```bash
sudo systemctl restart sshd
```

**Notes**

SSH container passthrough using `AuthorizedKeysCommand` will work only if

- `opensshd` is running on the container

SSHing shims could be created similarly to above.
