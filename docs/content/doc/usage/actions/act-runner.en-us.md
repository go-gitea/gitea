---
date: "2023-04-27T15:00:00+08:00"
title: "Act Runner"
slug: "act-runner"
weight: 20
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "Act Runner"
    weight: 20
    identifier: "actions-runner"
---

# Act Runner

This page will introduce the [act runner](https://gitea.com/gitea/act_runner) in detail, which is the runner of Gitea Actions.

**Table of Contents**

{{< toc >}}

## Requirements

It is recommended to run jobs in a docker container, so you need to install docker first.
And make sure that the docker daemon is running.

Other OCI container engines which are compatible with Docker's API should also work, but are untested.

However, if you are sure that you want to run jobs directly on the host only, then docker is not required.

## Installation

There are multiple ways to install the act runner.

### Download the binary

You can download the binary from the [release page](https://gitea.com/gitea/act_runner/releases).
However, if you want to use the latest nightly build, you can download it from the [download page](https://dl.gitea.com/act_runner/).

When you download the binary, please make sure that you have downloaded the correct one for your platform.
You can check it by running the following command:

```bash
chmod +x act_runner
./act_runner --version
```

If you see the version information, it means that you have downloaded the correct binary.

### Use the docker image

You can use the docker image from the [docker hub](https://hub.docker.com/r/gitea/act_runner/tags).
Just like the binary, you can use the latest nightly build by using the `nightly` tag, while the `latest` tag is the latest stable release.

```bash
docker pull gitea/act_runner:latest # for the latest stable release
docker pull gitea/act_runner:nightly # for the latest nightly build
```

## Configuration

Configuration is done via a configuration file. It is optional, and the default configuration will be used when no configuration file is specified.

You can generate a configuration file by running the following command:

```bash
./act_runner generate-config
```

The default configuration is safe to use without any modification, so you can just use it directly.

```bash
./act_runner generate-config > config.yaml
./act_runner --config config.yaml [command]
```

You could also generate config file with docker:

```bash
docker run --entrypoint="" --rm -it gitea/act_runner:latest act_runner generate-config > config.yaml
```

When you are using the docker image, you can specify the configuration file by using the `CONFIG_FILE` environment variable. Make sure that the file is mounted into the container as a volume:

```bash
docker run -v $(pwd)/config.yaml:/config.yaml -e CONFIG_FILE=/config.yaml ...
```

You may notice the commands above are both incomplete, because it is not the time to run the act runner yet.
Before running the act runner, we need to register it to your Gitea instance first.

## Registration

Registration is required before running the act runner, because the runner needs to know where to get jobs from.
And it is also important to Gitea instance to identify the runner.

### Runner levels

You can register a runner in different levels, it can be:

- Instance level: The runner will run jobs for all repositories in the instance.
- Organization level: The runner will run jobs for all repositories in the organization.
- Repository level: The runner will run jobs for the repository it belongs to.

Note that the repository may still use instance-level or organization-level runners even if it has its own repository-level runners. A future release may provide an option to allow more control over this.

### Obtain a registration token

The level of the runner determines where to obtain the registration token.

- Instance level: The admin settings page, like `<your_gitea.com>/admin/runners`.
- Organization level: The organization settings page, like `<your_gitea.com>/<org>/settings/runners`.
- Repository level: The repository settings page, like `<your_gitea.com>/<owner>/<repo>/settings/runners`.

If you cannot see the settings page, please make sure that you have the right permissions and that Actions have been enabled.

The format of the registration token is a random string `D0gvfu2iHfUjNqCYVljVyRV14fISpJxxxxxxxxxx`.

### Register the runner

The act runner can be registered by running the following command:

```bash
./act_runner register
```

Alternatively, you can use the `--config` option to specify the configuration file mentioned in the previous section.

```bash
./act_runner --config config.yaml register
```

You will be asked to input the registration information step by step. Includes:

- The Gitea instance URL, like `https://gitea.com/` or `http://192.168.8.8:3000/`.
- The registration token.
- The runner name, which is optional. If you leave it blank, the hostname will be used.
- The runner labels, which is optional. If you leave it blank, the default labels will be used.

You may be confused about the runner labels, which will be explained later.

If you want to register the runner in a non-interactive way, you can use arguments to do it.

```bash
./act_runner register --no-interactive --instance <instance_url> --token <registration_token> --name <runner_name> --labels <runner_labels>
```

When you have registered the runner, you can find a new file named `.runner` in the current directory.
This file stores the registration information.
Please do not edit it manually.
If this file is missing or corrupted, you can simply remove it and register again.

If you want to store the registration information in another place, you can specify it in the configuration file,
and don't forget to specify the `--config` option.

### Register the runner with docker

If you are using the docker image, behaviour will be slightly different. Registration and running are combined into one step in this case, so you need to specify the registration information when running the act runner.

```bash
docker run \
    -v $(pwd)/config.yaml:/config.yaml \
    -v $(pwd)/data:/data \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e CONFIG_FILE=/config.yaml \
    -e GITEA_INSTANCE_URL=<instance_url> \
    -e GITEA_RUNNER_REGISTRATION_TOKEN=<registration_token> \
    -e GITEA_RUNNER_NAME=<runner_name> \
    -e GITEA_RUNNER_LABELS=<runner_labels> \
    --name my_runner \
    -d gitea/act_runner:nightly
```

You may notice that we have mounted the `/var/run/docker.sock` into the container.
It is because the act runner will run jobs in docker containers, so it needs to communicate with the docker daemon.
As mentioned, you can remove it if you want to run jobs in the host directly.
To be clear, the "host" actually means the container which is running the act runner now, instead of the host machine.

### Set up the runner using docker compose

You could also set up the runner using the following `docker-compose.yml`:

```yml
version: "3.8"
services:
  runner:
    image: gitea/act_runner:nightly
    environment:
      CONFIG_FILE: /config.yaml
      GITEA_INSTANCE_URL: "${INSTANCE_URL}"
      GITEA_RUNNER_REGISTRATION_TOKEN: "${REGISTRATION_TOKEN}"
      GITEA_RUNNER_NAME: "${RUNNER_NAME}"
      GITEA_RUNNER_LABELS: "${RUNNER_LABELS}"
    volumes:
      - ./config.yaml:/config.yaml
      - ./data:/data
      - /var/run/docker.sock:/var/run/docker.sock
```

### Configuring cache when starting a Runner using docker image

If you do not intend to use `actions/cache` in workflow, you can ignore this section.

If you use `actions/cache` without any additional configuration, it will return the following error:
> Failed to restore: getCacheEntry failed: connect ETIMEDOUT IP:PORT

The error occurs because the runner container and job container are on different networks, so the job container cannot access the runner container.

Therefore, it is essential to configure the cache action to ensure its proper functioning. Follow these steps:

- 1.Obtain the LAN IP address of the host machine where the runner container is running.
- 2.Find an available port number on the host machine where the runner container is running.
- 3.Configure the following settings in the configuration file:

```yaml
cache:
  enabled: true
  dir: ""
  # Use the LAN IP obtained in step 1
  host: "192.168.8.17"
  # Use the port number obtained in step 2
  port: 8088
```

- 4.When starting the container, map the cache port to the host machine:

```bash
docker run \
  --name gitea-docker-runner \
  -p 8088:8088 \
  -d gitea/act_runner:nightly
```

### Labels

The labels of a runner are used to determine which jobs the runner can run, and how to run them.

The default labels are `ubuntu-latest:docker://node:16-bullseye,ubuntu-22.04:docker://node:16-bullseye,ubuntu-20.04:docker://node:16-bullseye,ubuntu-18.04:docker://node:16-buster`.
It is a comma-separated list, and each item is a label.

Let's take `ubuntu-22.04:docker://node:16-bullseye` as an example.
It means that the runner can run jobs with `runs-on: ubuntu-22.04`, and the job will be run in a docker container with the image `node:16-bullseye`.

If the default image is insufficient for your needs, and you have enough disk space to use a better and bigger one, you can change it to `ubuntu-22.04:docker://<the image you like>`.
You can find more useful images on [act images](https://github.com/nektos/act/blob/master/IMAGES.md).

If you want to run jobs in the host directly, you can change it to `ubuntu-22.04:host` or just `ubuntu-22.04`, the `:host` is optional.
However, we suggest you to use a special name like `linux_amd64:host` or `windows:host` to avoid misusing it.

One more thing is that it is recommended to register the runner if you want to change the labels.
It may be annoying to do this, so we may provide a better way to do it in the future.

## Running

After you have registered the runner, you can run it by running the following command:

```bash
./act_runner daemon
# or
./act_runner daemon --config config.yaml
```

The runner will fetch jobs from the Gitea instance and run them automatically.

Since act runner is still in development, it is recommended to check the latest version and upgrade it regularly.
