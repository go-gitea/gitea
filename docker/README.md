# Docker for Gitea

Visit [Docker Hub](https://hub.docker.com/r/gitea/) see all available images and tags.

## Usage

To keep your data out of Docker container, we do a volume (`/var/gitea` -> `/data`) here, and you can change it based on your situation.

```
# Pull image from Docker Hub.
$ docker pull gitea/gitea

# Create local directory for volume.
$ mkdir -p /var/gitea

# Use `docker run` for the first time.
$ docker run --name=gitea -p 10022:22 -p 10080:3000 -v /var/gitea:/data gitea/gitea

# Use `docker start` if you have stopped it.
$ docker start gitea
```

Note: It is important to map the gitea ssh service from the container to the host and set the appropriate SSH Port and URI settings when setting up gitea for the first time. To access and clone gitea Git repositories with the above configuration you would use: `git clone ssh://git@hostname:10022/username/myrepo.git` for example.

Files will be store in local path `/var/gitea` in my case.

Directory `/var/gitea` keeps Git repositories and gitea data:

    /var/gitea
    |-- git
    |   |-- gitea-repositories
    |-- ssh
    |   |-- # ssh public/private keys for gitea
    |-- gitea
        |-- conf
        |-- data
        |-- log

### Volume with data container

If you're more comfortable with mounting data to a data container, the commands you execute at the first time will look like as follows:

```
# Create data container
docker run --name=gitea-data --entrypoint /bin/true gitea/gitea

# Use `docker run` for the first time.
docker run --name=gitea --volumes-from gitea-data -p 10022:22 -p 10080:3000 gitea/gitea
```

#### Using Docker Volume command

```
# Create docker volume.
$ docker volume create --name gitea-data

# Use `docker run` for the first time.
$ docker run --name=gitea -p 10022:22 -p 10080:3000 -v gitea-data:/data gitea/gitea
```

## Settings

### Application

Most of settings are obvious and easy to understand, but there are some settings can be confusing by running gitea inside Docker:

- **Repository Root Path**: keep it as default value `/home/git/gitea-repositories` because `start.sh` already made a symbolic link for you.
- **Run User**: keep it as default value `git` because `start.sh` already setup a user with name `git`.
- **Domain**: fill in with Docker container IP (e.g. `192.168.99.100`). But if you want to access your gitea instance from a different physical machine, please fill in with the hostname or IP address of the Docker host machine.
- **SSH Port**: Use the exposed port from Docker container. For example, your SSH server listens on `22` inside Docker, but you expose it by `10022:22`, then use `10022` for this value. **Builtin SSH server is not recommended inside Docker Container**
- **HTTP Port**: Use port you want gitea to listen on inside Docker container. For example, your gitea listens on `3000` inside Docker, and you expose it by `10080:3000`, but you still use `3000` for this value.
- **Application URL**: Use combination of **Domain** and **exposed HTTP Port** values (e.g. `http://192.168.99.100:10080/`).

### Container options

This container have some options available via environment variables, these options are opt-in features that can help the administration of this container:

- **RUN_CROND**:
  - <u>Possible value:</u>
      `true`, `false`, `1`, `0`
  - <u>Default:</u>
      `false`
  - <u>Action:</u>
      Request crond to be run inside the container. Its default configuration will periodically run all scripts from `/etc/periodic/${period}` but custom crontabs can be added to `/var/spool/cron/crontabs/`.

## Upgrade

:exclamation::exclamation::exclamation:<span style="color: red">**Make sure you have volumed data to somewhere outside Docker container**</span>:exclamation::exclamation::exclamation:

Steps to upgrade gitea with Docker:

- `docker pull gitea/gitea`
- `docker stop gitea`
- `docker rm gitea`
- Finally, create container as the first time and don't forget to do same volume and port mapping.

## Known Issues

- The docker container can not currently be build on Raspberry 1 (armv6l) as our base image `alpine` does not have a `go` package available for this platform.
