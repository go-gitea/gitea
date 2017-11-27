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

Run below command in terminal:  
```
sudo vim /etc/systemd/system/gitea.service
```  

Add code to the file from [here](https://github.com/go-gitea/gitea/blob/master/contrib/systemd/gitea.service).  

Uncomment any service need to be enabled like mysql in this case in Unit section.  

Change the user(git) accordingly to yours. And /home/git too if your username is different than git. Change the PORT or remove the -p flag if default port is used.  

Lastly start and enable gitea at boot:  
```
sudo systemctl start gitea
```  
```
sudo systemctl enable gitea
```  


#### Using supervisor  

Install supervisor by running below command in terminal:  
```
sudo apt install supervisor
```  

Create a log dir for the supervisor logs(assuming gitea is installed in /home/git/gitea/):  
```
mkdir /home/git/gitea/log/supervisor
```  

Open supervisor config file in vi/vim/nano etc.  
```
sudo vim /etc/supervisor/supervisord.conf
```  

And append the code at the end of the file from [here](https://github.com/go-gitea/gitea/blob/master/contrib/supervisor/gitea).  

Change the user(git) accordingly to yours. And /home/git too if your username is different than git. Change the PORT or remove the -p flag if default port is used.  

Lastly start and enable supervisor at boot:  
```
sudo systemctl start supervisor
```  
```
sudo systemctl enable supervisor
```  

