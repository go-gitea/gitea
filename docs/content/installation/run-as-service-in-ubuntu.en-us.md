---
date: "2017-07-21T12:00:00+02:00"
title: "Run as service in Linux"
slug: "linux-service"
sidebar_position: 40
toc: false
draft: false
aliases:
  - /en-us/linux-service
menu:
  sidebar:
    parent: "installation"
    name: "Linux service"
    sidebar_position: 40
    identifier: "linux-service"
---

### Run Gitea as Linux service

You can run Gitea as service, using either systemd or supervisor. The steps below tested on Ubuntu 16.04, but those should work on any Linux distributions (with little modification).

#### Using systemd

Copy the sample [gitea.service](https://github.com/go-gitea/gitea/blob/main/contrib/systemd/gitea.service) to `/etc/systemd/system/gitea.service`, then edit the file with your favorite editor.

Uncomment any service that needs to be enabled on this host, such as MySQL.

Change the user, home directory, and other required startup values. Change the
PORT or remove the -p flag if default port is used.

Enable and start Gitea at boot:

```
sudo systemctl enable gitea
sudo systemctl start gitea
```

If you have systemd version 220 or later, you can enable and immediately start Gitea at once by:

```
sudo systemctl enable gitea --now
```

#### Using supervisor

Install supervisor by running below command in terminal:

```
sudo apt install supervisor
```

Create a log dir for the supervisor logs:

```
# assuming Gitea is installed in /home/git/gitea/
mkdir /home/git/gitea/log/supervisor
```

Append the configuration from the sample
[supervisord config](https://github.com/go-gitea/gitea/blob/main/contrib/supervisor/gitea) to `/etc/supervisor/supervisord.conf`.

Using your favorite editor, change the user (`git`) and home
(`/home/git`) settings to match the deployment environment. Change the PORT
or remove the -p flag if default port is used.

Lastly enable and start supervisor at boot:

```
sudo systemctl enable supervisor
sudo systemctl start supervisor
```

If you have systemd version 220 or later, you can enable and immediately start supervisor by:

```
sudo systemctl enable supervisor --now
```
