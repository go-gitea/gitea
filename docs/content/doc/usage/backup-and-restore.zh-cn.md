---
date: "2018-06-06T09:33:00+08:00"
title: "使用：备份与恢复"
slug: "backup-and-restore"
weight: 11
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "备份与恢复"
    weight: 11
    identifier: "backup-and-restore"
---

# 备份与恢复

Gitea 已经实现了 `dump` 命令可以用来备份所有需要的文件到一个zip压缩文件。该压缩文件可以被用来进行数据恢复。

## 备份命令 (`dump`)

先转到git用户的权限: `su git`. 再Gitea目录运行 `./gitea dump`。一般会显示类似如下的输出：

```
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

最后生成的 `gitea-dump-1482906742.zip` 文件将会包含如下内容：

* `custom` - 所有保存在 `custom/` 目录下的配置和自定义的文件。
* `data` - 数据目录下的所有内容不包含使用文件session的文件。该目录包含 `attachments`, `avatars`, `lfs`, `indexers`, 如果使用sqlite 还会包含 sqlite 数据库文件。
* `gitea-db.sql` - 数据库dump出来的 SQL。
* `gitea-repo.zip` - Git仓库压缩文件。
* `log/` - Logs文件，如果用作迁移不是必须的。

中间备份文件将会在临时目录进行创建，如果您要重新指定临时目录，可以用 `--tempdir` 参数，或者用 `TMPDIR` 环境变量。

## Restore Command (`restore`)

当前还没有恢复命令，恢复需要人工进行。主要是把文件和数据库进行恢复。

例如：

```
apt-get install gitea
unzip gitea-dump-1482906742.zip
cd gitea-dump-1482906742
mv custom/conf/app.ini /etc/gitea/conf/app.ini
unzip gitea-repo.zip
mv gitea-repo/* /var/lib/gitea/repositories/
chown -R gitea:gitea /etc/gitea/conf/app.ini /var/lib/gitea/repositories/
mysql -u$USER -p$PASS $DATABASE <gitea-db.sql
# or  sqlite3 $DATABASE_PATH <gitea-db.sql
service gitea restart
```
