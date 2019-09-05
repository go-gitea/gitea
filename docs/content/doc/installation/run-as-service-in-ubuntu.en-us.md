---
date: "2017-07-21T12:00:00+02:00"
title: "Run as service in Linux"
slug: "linux-service"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Linux service"
    weight: 20
    identifier: "linux-service"
---

### Run as service in Debian (and derivative distros, i.e. Ubuntu 16.04 LTS)

#### Using systemd

Run the below command in a terminal:
```
sudo vim /etc/systemd/system/gitea.service
```

Copy the sample [gitea.service](https://github.com/go-gitea/gitea/blob/master/contrib/systemd/gitea.service).

Uncomment any service that needs to be enabled on this host, such as MySQL.
Also, the service file linked above uses /root/gitea, which you may need to create.

Change the user, home directory, and other required startup values. Change the
PORT or remove the -p flag if default port is used.

Enable and start Gitea at boot:
```
sudo systemctl enable gitea
sudo systemctl start gitea
```

You can customize the ExecStart line to use a different configuration file:
```
ExecStart=/usr/local/bin/gitea web -c /etc/gitea/app.ini
```
Edit the gitea.service file as needed to use a gitea configuration stored in a non-default location.

Also configurable are the Working Directory and Environment lines in the \[Service\] section of the gitea.service file:

```
WorkingDirectory=/var/lib/gitea/
```

```
Environment=USER=git HOME=/home/git GITEA_WORK_DIR=/var/lib/gitea
```

The above example lines are necessary for more customized and secure Gitea installations that do not run as the root user.
Another reason to customize the gitea.service file is running multiple separate installations on one server.

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

Open supervisor config file in a file editor:
```
sudo vim /etc/supervisor/supervisord.conf
```

Append the configuration from the sample
[supervisord config](https://github.com/go-gitea/gitea/blob/master/contrib/supervisor/gitea).

Change the user (git) and home (/home/git) settings to match the deployment
environment. Change the PORT or remove the -p flag if default port is used.

Lastly enable and start supervisor at boot:
```
sudo systemctl enable supervisor
sudo systemctl start supervisor
```
