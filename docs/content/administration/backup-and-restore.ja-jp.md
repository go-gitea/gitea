---
date: "2017-01-01T16:00:00+02:00"
title: "バックアップとリストア"
slug: "backup-and-restore"
sidebar_position: 11
toc: false
draft: false
aliases:
  - /ja-jp/backup-and-restore
menu:
  sidebar:
    parent: "administration"
    name: "バックアップとリストア"
    sidebar_position: 11
    identifier: "backup-and-restore"
---

# バックアップとリストア

Gitea の `dump` コマンドを使い、インスタンス内のデータを ZIP ファイルに保存する機能が実装されています。
このファイルを解凍して、インスタンスの復元に使用できます。

## バックアップの一貫性

Gitea インスタンスの一貫性を確保するには、バックアップ中にインスタンスをシャットダウンする必要があります。

Gitea はデータベース、ファイル、Git リポジトリで構成されており、稼働中に変更される可能性があります。マイグレーションが進行している場合、git リポジトリがコピーされている間にデータベースにトランザクションが作成されます。もしバックアップがマイグレーションの途中で行われた場合、データベースはGit リポジトリの後でダンプされるため、データベースで記録されている Git リポジトリが存在しない、つまり Git リポジトリファイルは不完全である可能性があります。このような競合状態を回避する唯一の方法は、バックアップ中に Gitea インスタンスを停止することです。

## バックアップコマンド (`dump`)

Gitea を実行しているユーザーに切り替えます：`su git`。Gitea のインストールフォルダで `./gitea dump -c /path/to/app.ini` を実行します。
次のような出力が表示されるはず：

```none
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

`gitea-dump-1482906742.zip` ファイルの中には下記があるます：

- `app.ini` - デフォルトの `custom/` ディレクトリの外に保存されていた場合、設定ファイルはオプションとしてコピーできます。
- `custom/` - `custom/` にあるすべての設定ファイルとカスタマイズファイル。
- `data/` - データディレクトリ (APP_DATA_PATH) (ファイルセッションを使用している場合はセッションファイルを除く)。このディレクトリには、`attachments`、`avatar`、`lfs`、`indexers`、SQLite ファイルが含まれます (SQLite を使用している場合)。
- `repos/` - リポジトリディレクトリの完全なコピー。
- `gitea-db.sql` - データベースのSQLダンプ
- `log/` - いろいろなログ。これらはリカバリとマイグレーションには必要がありません。

中間バックアップファイルは一時ディレクトリに作成されます。一時ディレクトリの場所は `--tempdir` パラメータまたは `TMPDIR` 環境変数で指定できます。

## データベースのバックアップ

`gitea dump` によって作成された SQL ダンプは XORM を使用するため、Gitea 管理者は代わりにネイティブの MySQL または PostgreSQL ダンプツールを使用する場合があります。データベースのダンプに XORM を使用する場合、データベースを復元しようとすると問題が発生する可能性があるという未解決の問題があります。

```sh
# mysql
mysqldump -u$USER -p$PASS --database $DATABASE > gitea-db.sql
# postgres
pg_dump -U $USER $DATABASE > gitea-db.sql
```

### Dockerを利用する場合 (`dump`)

Docker 環境で `dump` コマンドを使用する場合には、いくつかの注意事項があります。

このコマンドは `gitea/conf/app.ini` で指定された `RUN_USER = <OS_USERNAME>` アカウントで実行する必要があります。また、バックアップフォルダーの圧縮を行う時に、パーミッションエラーを回避するには、コマンド `docker exec` を `--tempdir` フォルダ内で実行する必要があります。

例：

```none
docker exec -u <OS_USERNAME> -it -w <--tempdir> $(docker ps -qf 'name=^<NAME_OF_DOCKER_CONTAINER>$') bash -c '/usr/local/bin/gitea dump -c </path/to/app.ini>'
```

`--tempdir` は、Gitea によって使用される Docker 環境の一時ディレクトリを指します。カスタムの `--tempdir` を指定していない場合、Gitea は `/tmp` または Docker コンテナの `TMPDIR` 環境変数を使用します。`--tempdir` を指定する場合、`docker exec` コマンドオプションを調整してください。

バックアップの結果は、指定された `--tempdir` に、`gitea-dump-1482906742.zip` というフォーマットで保存されたファイルになります。

## リストアコマンド (`restore`)

現在、リストアコマンドはサポートされていません。ほとんどの場合は手動でファイルを正しい場所に移動して、データベースダンプを復元するという手順で行います。

例:

```sh
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
mv app.ini /etc/gitea/conf/app.ini
mv data/* /var/lib/gitea/data/
mv log/* /var/lib/gitea/log/
mv repos/* /var/lib/gitea/gitea-repositories/
chown -R gitea:gitea /etc/gitea/conf/app.ini /var/lib/gitea

# mysql
mysql --default-character-set=utf8mb4 -u$USER -p$PASS $DATABASE <gitea-db.sql
# sqlite3
sqlite3 $DATABASE_PATH <gitea-db.sql
# postgres
psql -U $USER -d $DATABASE < gitea-db.sql

service gitea restart
```

インストール方法が変更された場合 (例: バイナリ -> Docker)、または Gitea が以前のインストールとは異なるディレクトリにインストールされている場合、リポジトリの Git フックを再生成する必要があります。

Gitea を実行し、Gitea のバイナリが配置されているディレクトリから `./gitea admin regenerate hooks` を実行します。

This ensures that application and configuration file paths in repository Git Hooks are consistent and applicable to the current installation. If these paths are not updated, repository `push` actions will fail.

### Using Docker (`restore`)

There is also no support for a recovery command in a Docker-based gitea instance. The restore process contains the same steps as described in the previous section but with different paths.

Example:

```sh
# open bash session in container
docker exec --user git -it 2a83b293548e bash
# unzip your backup file within the container
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
# restore the gitea data
mv data/* /data/gitea
# restore the repositories itself
mv repos/* /data/git/gitea-repositories/
# adjust file permissions
chown -R git:git /data
# Regenerate Git Hooks
/usr/local/bin/gitea -c '/data/gitea/conf/app.ini' admin regenerate hooks
```

The default user in the gitea container is `git` (1000:1000). Please replace `2a83b293548e` with your gitea container id or name.

### Using Docker-rootless (`restore`)

The restore workflow in Docker-rootless containers differs only in the directories to be used:

```sh
# open bash session in container
docker exec --user git -it 2a83b293548e bash
# unzip your backup file within the container
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
# restore the app.ini
mv data/conf/app.ini /etc/gitea/app.ini
# restore the gitea data
mv data/* /var/lib/gitea
# restore the repositories itself
mv repos/* /var/lib/gitea/git/gitea-repositories
# adjust file permissions
chown -R git:git /etc/gitea/app.ini /var/lib/gitea
# Regenerate Git Hooks
/usr/local/bin/gitea -c '/etc/gitea/app.ini' admin regenerate hooks
```
