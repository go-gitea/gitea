---
date: "2021-07-20T00:00:00+00:00"
title: "Container Registry"
slug: "container"
sidebar_position: 30
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Container Registry"
    sidebar_position: 30
    identifier: "container"
---

# Container Registry

Publish [Open Container Initiative](https://opencontainers.org/) compliant images for your user or organization.
The container registry follows the OCI specs and supports all compatible images like [Docker](https://www.docker.com/) and [Helm Charts](https://helm.sh/).

## Requirements

To work with the Container registry, you can use the tools for your specific image type.
The following examples use the `docker` client.

## Login to the container registry

To push an image or if the image is in a private registry, you have to authenticate:

```shell
docker login gitea.example.com
```

If you are using 2FA or OAuth use a [personal access token](development/api-usage.md#authentication) instead of the password.

## Image naming convention

Images must follow this naming convention:

`{registry}/{owner}/{image}`

For example, these are all valid image names for the owner `testuser`:

`gitea.example.com/testuser/myimage`

`gitea.example.com/testuser/my-image`

`gitea.example.com/testuser/my/image`

**NOTE:** The registry only supports case-insensitive tag names. So `image:tag` and `image:Tag` get treated as the same image and tag.

## Push an image

Push an image by executing the following command:

```shell
docker push gitea.example.com/{owner}/{image}:{tag}
```

| Parameter | Description |
| ----------| ----------- |
| `owner`   | The owner of the image. |
| `image`   | The name of the image. |
| `tag`     | The tag of the image. |

For example:

```shell
docker push gitea.example.com/testuser/myimage:latest
```

## Pull an image

Pull an image by executing the following command:

```shell
docker pull gitea.example.com/{owner}/{image}:{tag}
```

| Parameter | Description |
| ----------| ----------- |
| `owner`   | The owner of the image. |
| `image`   | The name of the image. |
| `tag`     | The tag of the image. |

For example:

```shell
docker pull gitea.example.com/testuser/myimage:latest
```
