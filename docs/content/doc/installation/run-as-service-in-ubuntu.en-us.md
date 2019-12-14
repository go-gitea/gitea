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

### Run as service in Ubuntu 16.04 LTS

#### Using systemd

Run the below command in a terminal:
```
sudo vim /etc/systemd/system/gitea.service
```

Copy the sample [gitea.service](https://github.com/go-gitea/gitea/blob/master/contrib/systemd/gitea.service).

Uncomment any service that needs to be enabled on this host, such as MySQL.

Change the user, home directory, and other required startup values. Change the
PORT or remove the -p flag if default port is used.

Enable and start Gitea at boot:
```
sudo systemctl enable gitea
sudo systemctl start gitea
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
