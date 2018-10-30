---
date: "2017-07-21T12:00:00+02:00"
title: "在 Linux 中以 service 方式运行"
slug: "linux-service"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "在Linux中以service方式运行"
    weight: 20
    identifier: "linux-service"
---

### 在 Ubuntu 16.04 LTS 中以 service 方式运行

#### systemd 方式

在 terminal 中执行以下命令：
```
sudo vim /etc/systemd/system/gitea.service
```

接着拷贝示例代码 [gitea.service](https://github.com/go-gitea/gitea/blob/master/contrib/systemd/gitea.service) 并取消对任何需要运行在主机上的服务部分的注释，譬如 MySQL。

修改 user，home 目录以及其他必须的初始化参数，如果使用自定义端口，则需修改 PORT 参数，反之如果使用默认端口则需删除 -p 标记。

激活 gitea 并将它作为系统自启动服务：
```
sudo systemctl enable gitea
sudo systemctl start gitea
```


#### 使用 supervisor

在 terminal 中执行以下命令安装 supervisor：
```
sudo apt install supervisor
```

为 supervisor 配置日志路径：
```
# assuming gitea is installed in /home/git/gitea/
mkdir /home/git/gitea/log/supervisor
```

在文件编辑器中打开 supervisor 的配置文件：
```
sudo vim /etc/supervisor/supervisord.conf
```

增加如下示例配置
[supervisord config](https://github.com/go-gitea/gitea/blob/master/contrib/supervisor/gitea)。

将 user(git) 和 home(/home/git) 设置为与上文部署中匹配的值。如果使用自定义端口，则需修改 PORT 参数，反之如果使用默认端口则需删除 -p 标记。

最后激活 supervisor 并将它作为系统自启动服务：
```
sudo systemctl enable supervisor
sudo systemctl start supervisor
```
