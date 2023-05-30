---
date: "2017-07-21T12:00:00+02:00"
title: "在 Linux 中以 service 方式运行"
slug: "linux-service"
weight: 40
toc: false
draft: false
aliases:
  - /zh-cn/linux-service
menu:
  sidebar:
    parent: "installation"
    name: "在Linux中以service方式运行"
    weight: 40
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
#开启开机自启动服务
sudo systemctl enable gitea
#启动服务
sudo systemctl start gitea
```
如果需要查看已有的服务，可以使用:
```
sudo systemctl list-units
```

如果有些服务文件，虽然已经在服务文件的目录下，但是它不是热生效的，即不是说咱们把它放进去就能被识别、注册、启动的。它没有被 systemctl 的 start 或 enable 命令登记到Systemd，如果你需要的话，得自己做这个操作。
如果想查看没被激活的服务文件怎么办呢？
```
sudo systemctl list-unit-files
```

如果我们修改了一个服务文件，可是Systemd不知道，因为它缓存了一份服务，所以需要重新载入，否则它还是使用旧的。这个行为类似于service nginx reload，在这里是：
```
sudo systemctl daemon-reload
```

最后，如果是删除服务文件的话，又不一样了，即使我们reload，Systemd已然可以使用自己缓存的服务文件，哪怕你用了daemon-reload更新。所以这时候要告诉Systemd，我们已经放弃不存在的服务文件了，让它也放弃自己缓存的那份：
```
systemctl reset-failed
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
