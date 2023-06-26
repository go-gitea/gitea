---
date: "2020-03-19T19:27:00+02:00"
title: "Installation with Docker"
slug: "install-with-docker"
weight: 70
toc: false
draft: false
aliases:
  - /en-us/install-with-docker
menu:
  sidebar:
    parent: "installation"
    name: "With Docker"
    weight: 70
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
+      - GITEA__database__DB_TYPE=mysql
+      - GITEA__database__HOST=db:3306
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
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
+      - GITEA__database__DB_TYPE=postgres
+      - GITEA__database__HOST=db:5432
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
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
+    image: postgres:14
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
-      - ./gitea:/data
+      - gitea:/data
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

In addition to the environment variables above, any settings in `app.ini` can be set
or overridden with an environment variable of the form: `GITEA__SECTION_NAME__KEY_NAME`.
These settings are applied each time the docker container starts.
Full information [here](https://github.com/go-gitea/gitea/tree/master/contrib/environment-to-ini).

These environment variables can be passed to the docker container in `docker-compose.yml`.
The following example will enable an smtp mail server if the required env variables
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
      - GITEA__mailer__MAILER_TYPE=smtp
      - GITEA__mailer__HOST=${GITEA__mailer__HOST:?GITEA__mailer__HOST not set}
      - GITEA__mailer__IS_TLS_ENABLED=true
      - GITEA__mailer__USER=${GITEA__mailer__USER:-apikey}
      - GITEA__mailer__PASSWD="""${GITEA__mailer__PASSWD:?GITEA__mailer__PASSWD not set}"""
```

Gitea will generate new secrets/tokens for every new installation automatically and write them into the app.ini. If you want to set the secrets/tokens manually, you can use the following docker commands to use of Gitea's built-in [generate utility functions](https://docs.gitea.io/en-us/command-line/#generate). Do not lose/change your SECRET_KEY after the installation, otherwise the encrypted data can not be decrypted anymore.

The following commands will output a new `SECRET_KEY` and `INTERNAL_TOKEN` to `stdout`, which you can then place in your environment variables.

```bash
docker run -it --rm gitea/gitea:1 gitea generate secret SECRET_KEY
docker run -it --rm  gitea/gitea:1 gitea generate secret INTERNAL_TOKEN
```

```yaml
...
services:
  server:
    environment:
      - GITEA__security__SECRET_KEY=[value returned by generate secret SECRET_KEY]
      - GITEA__security__INTERNAL_TOKEN=[value returned by generate secret INTERNAL_TOKEN]
```

## SSH Container Passthrough

Since SSH is running inside the container, SSH needs to be passed through from the host to the container if SSH support is desired. One option would be to run the container SSH on a non-standard port (or moving the host port to a non-standard port). Another option which might be more straightforward is for Gitea users to ssh to a Gitea user on the host which will then relay those connections to the docker.

### Understanding SSH access to Gitea (without passthrough)

To understand what needs to happen, you first need to understand what happens without passthrough. So we will try to explain this:

1. The client adds their SSH public key to Gitea using the webpage.
2. Gitea will add an entry for this key to the `.ssh/authorized_keys` file of its running user, `git`.
3. This entry has the public key, but also has a `command=` option. It is this command that Gitea uses to match this key to the client user and manages authentication.
4. The client then makes an SSH request to the SSH server using the `git` user, e.g. `git clone git@domain:user/repo.git`.
5. The client will attempt to authenticate with the server, passing one or more public keys one at a time to the server.
6. For each key the client provides, the SSH server will first check its configuration for an `AuthorizedKeysCommand` to see if the public key matches, and then the `git` user's `authorized_keys` file.
7. The first entry that matches will be selected, and assuming this is a Gitea entry, the `command=` will now be executed.
8. The SSH server creates a user session for the `git` user, and using the shell for the `git` user runs the `command=`
9. This runs `gitea serv` which takes over control of the rest of the SSH session and manages gitea authentication & authorization of the git commands.

Now, for the SSH passthrough to work, we need the host SSH to match the public keys and then run the `gitea serv` on the docker. There are multiple ways of doing this. However, all of these require some information about the docker being passed to the host.

### SSHing Shim (with authorized_keys)

In this option, the idea is that the host simply uses the `authorized_keys` that gitea creates but at step 9 the `gitea` command that the host runs is a shim that actually runs ssh to go into the docker and then run the real docker `gitea` itself.

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

- Mount `/home/git/.ssh` of the host into the container. This ensures that the `authorized_keys` file is shared between the host `git` user and the container `git` user otherwise the SSH authentication cannot work inside the container.

  ```yaml
  volumes:
    - /home/git/.ssh/:/data/git/.ssh
  ```

- Now a SSH key pair needs to be created on the host. This key pair will be used to authenticate the `git` user on the host to the container. As an administrative user on the host run: (by administrative user we mean a user that can sudo to root)

  ```bash
  sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
  ```

- Please note depending on the local version of ssh you may want to consider using `-t ecdsa` here.

- `/home/git/.ssh/authorized_keys` on the host now needs to be modified. It needs to act in the same way as `authorized_keys` within the Gitea container. Therefore add the public key of the key you created above ("Gitea Host Key") to `~/git/.ssh/authorized_keys`. As an administrative user on the host run:

  ```bash
  sudo -u git cat /home/git/.ssh/id_rsa.pub | sudo -u git tee -a /home/git/.ssh/authorized_keys
  sudo -u git chmod 600 /home/git/.ssh/authorized_keys
  ```

  Important: The pubkey from the `git` user needs to be added "as is" while all other pubkeys added via the Gitea web interface will be prefixed with `command="/usr [...]`.

  `/home/git/.ssh/authorized_keys` should then look somewhat like

  ```bash
  # SSH pubkey from git user
  ssh-rsa <Gitea Host Key>

  # other keys from users
  command="/usr/local/bin/gitea --config=/data/gitea/conf/app.ini serv key-1",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <user pubkey>
  ```

- The next step is to create the fake host `gitea` command that will forward commands from the host to the container. The name of this file depends on your version of Gitea:

  - For Gitea v1.16.0+. As an administrative user on the host run:

    ```bash
    cat <<"EOF" | sudo tee /usr/local/bin/gitea
    #!/bin/sh
    ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
    EOF
    sudo chmod +x /usr/local/bin/gitea
    ```

Here is a detailed explanation what is happening when a SSH request is made:

1. The client adds their SSH public key to Gitea using the webpage.
2. Gitea in the container will add an entry for this key to the `.ssh/authorized_keys` file of its running user, `git`.
    - However, because `/home/git/.ssh/` on the host is mounted as `/data/git/.ssh` this means that the key has been added to the host `git` user's `authorized_keys` file too.
3. This entry has the public key, but also has a `command=` option.
    - This command matches the location of the Gitea binary on the container, but also the location of the shim on the host.
4. The client then makes an SSH request to the host SSH server using the `git` user, e.g. `git clone git@domain:user/repo.git`.
5. The client will attempt to authenticate with the server, passing one or more public keys in turn to the host.
6. For each key the client provides, the host SSH server will first check its configuration for an `AuthorizedKeysCommand` to see if the public key matches, and then the host `git` user's `authorized_keys` file.
    - Because `/home/git/.ssh/` on the host is mounted as `/data/git/.ssh` this means that the key they added to the Gitea web will be found
7. The first entry that matches will be selected, and assuming this is a Gitea entry, the `command=` will now be executed.
8. The host SSH server creates a user session for the `git` user, and using the shell for the host `git` user runs the `command=`
9. This means that the host runs the host `/usr/local/bin/gitea` shim that opens an SSH from the host to container passing the rest of the command arguments directly to `/usr/local/bin/gitea` on the container.
10. Meaning that the container `gitea serv` is run, taking over control of the rest of the SSH session and managing gitea authentication & authorization of the git commands.

**Notes**

SSH container passthrough using `authorized_keys` will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed (depending on the changes)

If you try to run `gitea` on the host, you will attempt to ssh to the container and thence run the `gitea` command there.

Never add the `Gitea Host Key` as a SSH key to a user on the Gitea interface.

### SSHing Shell (with authorized_keys)

In this option, the idea is that the host simply uses the `authorized_keys` that gitea creates but at step 8 above we change the shell that the host runs to ssh directly into the docker and then run the shell there. This means that the `gitea` that is then run is the real docker `gitea`.

- In this case we setup as per SSHing Shim except instead of creating `/usr/local/bin/gitea`
we create a new shell for the git user. As an administrative user on the host run:

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

1. The client adds their SSH public key to Gitea using the webpage.
2. Gitea in the container will add an entry for this key to the `.ssh/authorized_keys` file of its running user, `git`.
    - However, because `/home/git/.ssh/` on the host is mounted as `/data/git/.ssh` this means that the key has been added to the host `git` user's `authorized_keys` file too.
3. This entry has the public key, but also has a `command=` option.
    - This command matches the location of the Gitea binary on the container.
4. The client then makes an SSH request to the host SSH server using the `git` user, e.g. `git clone git@domain:user/repo.git`.
5. The client will attempt to authenticate with the server, passing one or more public keys in turn to the host.
6. For each key the client provides, the host SSH server will first check its configuration for an `AuthorizedKeysCommand` to see if the public key matches, and then the host `git` user's `authorized_keys` file.
    - Because `/home/git/.ssh/` on the host is mounted as `/data/git/.ssh` this means that the key they added to the Gitea web will be found
7. The first entry that matches will be selected, and assuming this is a Gitea entry, the `command=` will now be executed.
8. The host SSH server creates a user session for the `git` user, and using the shell for the host `git` user runs the `command=`
9. The shell of the host `git` user is now our `ssh-shell` which opens an SSH connection from the host to container, (which opens a shell on the container for the container `git`).
10. The container shell now runs the `command=` option meaning that the container `gitea serv` is run, taking over control of the rest of the SSH session and managing gitea authentication & authorization of the git commands.

**Notes**

SSH container passthrough using `authorized_keys` will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed (depending on the changes)

If you try to login as the `git` user on the host in future you will ssh directly to the docker.

Never add the `Gitea Host Key` as a SSH key to a user on the Gitea interface.

### Docker Shell (with authorized_keys)

Similar to the above ssh shell technique we can use a shell which simply uses `docker exec`. As an administrative user on the host run:

```bash
cat <<"EOF" | sudo tee /home/git/docker-shell
#!/bin/sh
/usr/bin/docker exec -i -u git --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
EOF
sudo chmod +x /home/git/docker-shell
sudo usermod -s /home/git/docker-shell git
```

Here is a detailed explanation what is happening when a SSH request is made:

1. The client adds their SSH public key to Gitea using the webpage.
2. Gitea in the container will add an entry for this key to the `.ssh/authorized_keys` file of its running user, `git`.
    - However, because `/home/git/.ssh/` on the host is mounted as `/data/git/.ssh` this means that the key has been added to the host `git` user's `authorized_keys` file too.
3. This entry has the public key, but also has a `command=` option.
    - This command matches the location of the Gitea binary on the container.
4. The client then makes an SSH request to the host SSH server using the `git` user, e.g. `git clone git@domain:user/repo.git`.
5. The client will attempt to authenticate with the server, passing one or more public keys in turn to the host.
6. For each key the client provides, the host SSH server will first check its configuration for an `AuthorizedKeysCommand` to see if the public key matches, and then the host `git` user's `authorized_keys` file.
    - Because `/home/git/.ssh/` on the host is mounted as `/data/git/.ssh` this means that the key they added to the Gitea web will be found
7. The first entry that matches will be selected, and assuming this is a Gitea entry, the `command=` will now be executed.
8. The host SSH server creates a user session for the `git` user, and using the shell for the host `git` user runs the `command=`
9. The shell of the host `git` user is now our `docker-shell` which uses `docker exec` to open a shell for the `git` user on the container.
10. The container shell now runs the `command=` option meaning that the container `gitea serv` is run, taking over control of the rest of the SSH session and managing gitea authentication & authorization of the git commands.

Note that `gitea` in the docker command above is the name of the container. If you named yours differently, don't forget to change that. The host `git` user also has to have
permission to run `docker exec`.

**Notes**

Docker shell passthrough using `authorized_keys` will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed (depending on the changes)

If you try to login as the `git` user on the host in future you will `docker exec` directly to the docker.

A Docker execing shim could be created similarly to above.

### Docker Shell with AuthorizedKeysCommand

The AuthorizedKeysCommand route provides another option that does not require many changes to the compose file or the `authorized_keys` - but does require changes to the host `/etc/sshd_config`.

In this option, the idea is that the host SSH uses an `AuthorizedKeysCommand` instead of relying on sharing the `authorized_keys` file that gitea creates. We continue to use a special shell at step 8 above to exec into the docker and then run the shell there. This means that the `gitea` that is then run is the real docker `gitea`.

- On the host create a `git` user with permission to run `docker exec`.
- We will again assume that the Gitea container is called `gitea`.
- Modify the `git` user's shell to forward commands to the `sh` executable inside the container using `docker exec`.  As an administrative user on the host run:

  ```bash
  cat <<"EOF" | sudo tee /home/git/docker-shell
  #!/bin/sh
  /usr/bin/docker exec -i --env SSH_ORIGINAL_COMMAND="$SSH_ORIGINAL_COMMAND" gitea sh "$@"
  EOF
  sudo chmod +x /home/git/docker-shell
  sudo usermod -s /home/git/docker-shell git
  ```

Now all attempts to login as the `git` user on the host will be forwarded to the docker - including the `SSH_ORIGINAL_COMMAND`. We now need to set-up SSH authentication on the host.

We will do this by leveraging the [SSH AuthorizedKeysCommand](https://docs.gitea.io/en-us/command-line/#keys) to match the keys against those accepted by Gitea.

Add the following block to `/etc/ssh/sshd_config`, on the host:

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/docker exec -i gitea /usr/local/bin/gitea keys -c /data/gitea/conf/app.ini -e git -u %u -t %t -k %k
```

(From 1.16.0 you will not need to set the `-c /data/gitea/conf/app.ini` option.)

Finally restart the SSH server. As an administrative user on the host run:

```bash
sudo systemctl restart sshd
```

Here is a detailed explanation what is happening when a SSH request is made:

1. The client adds their SSH public key to Gitea using the webpage.
2. Gitea in the container will add an entry for this key to its database.
3. The client then makes an SSH request to the host SSH server using the `git` user, e.g. `git clone git@domain:user/repo.git`.
4. The client will attempt to authenticate with the server, passing one or more public keys in turn to the host.
5. For each key the client provides, the host SSH server will checks its configuration for an `AuthorizedKeysCommand`.
6. The host runs the above `AuthorizedKeysCommand`, which execs in to the docker and then runs the `gitea keys` command.
7. Gitea on the docker will look in it's database to see if the public key matches and will return an entry like that of an `authorized_keys` command.
8. This entry has the public key, but also has a `command=` option which matches the location of the Gitea binary on the container.
9. The host SSH server creates a user session for the `git` user, and using the shell for the host `git` user runs the `command=`.
10. The shell of the host `git` user is now our `docker-shell` which uses `docker exec` to open a shell for the `git` user on the container.
11. The container shell now runs the `command=` option meaning that the container `gitea serv` is run, taking over control of the rest of the SSH session and managing gitea authentication & authorization of the git commands.

**Notes**

Docker shell passthrough using `AuthorizedKeysCommand` will work only if

- The host `git` user is allowed to run the `docker exec` command.

If you try to login as the `git` user on the host in future you will `docker exec` directly to the docker.

A Docker execing shim could be created similarly to above.

### SSH Shell with AuthorizedKeysCommand

Create a key for the host `git` user as above, add it to the docker `/data/git/.ssh/authorized_keys` then finally create and set the `ssh-shell` as above.

Add the following block to `/etc/ssh/sshd_config`, on the host:

```bash
Match User git
  AuthorizedKeysCommandUser git
  AuthorizedKeysCommand /usr/bin/ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 /usr/local/bin/gitea keys -c /data/gitea/conf/app.ini -e git -u %u -t %t -k %k
```

(From 1.16.0 you will not need to set the `-c /data/gitea/conf/app.ini` option.)

Finally restart the SSH server. As an administrative user on the host run:

```bash
sudo systemctl restart sshd
```

Here is a detailed explanation what is happening when a SSH request is made:

1. The client adds their SSH public key to Gitea using the webpage.
2. Gitea in the container will add an entry for this key to its database.
3. The client then makes an SSH request to the host SSH server using the `git` user, e.g. `git clone git@domain:user/repo.git`.
4. The client will attempt to authenticate with the server, passing one or more public keys in turn to the host.
5. For each key the client provides, the host SSH server will checks its configuration for an `AuthorizedKeysCommand`.
6. The host runs the above `AuthorizedKeysCommand`, which will SSH in to the docker and then run the `gitea keys` command.
7. Gitea on the docker will look in it's database to see if the public key matches and will return an entry like that of an `authorized_keys` command.
8. This entry has the public key, but also has a `command=` option which matches the location of the Gitea binary on the container.
9. The host SSH server creates a user session for the `git` user, and using the shell for the host `git` user runs the `command=`.
10. The shell of the host `git` user is now our `git-shell` which uses SSH to open a shell for the `git` user on the container.
11. The container shell now runs the `command=` option meaning that the container `gitea serv` is run, taking over control of the rest of the SSH session and managing gitea authentication & authorization of the git commands.

**Notes**

SSH container passthrough using `AuthorizedKeysCommand` will work only if

- `opensshd` is running on the container

If you try to login as the `git` user on the host in future you will `ssh` directly to the docker.

Never add the `Gitea Host Key` as a SSH key to a user on the Gitea interface.

SSHing shims could be created similarly to above.
