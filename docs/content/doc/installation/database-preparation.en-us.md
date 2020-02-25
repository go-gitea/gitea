---
date: "2020-01-16"
title: "Database Preparation"
slug: "database-prep"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Database preparation"
    weight: 20
    identifier: "database-prep"
---

You need a database to use Gitea. Gitea supports PostgreSQL, MySQL, SQLite, and MSSQL. This page will guide into preparing database. Only PostgreSQL and MySQL will be covered here since those database engines are widely-used in production.

Database instance can be on same machine as Gitea (local database setup), or on different machine (remote database).

Note: All steps below requires that the database engine of your choice is installed on your system. For remote database setup, install the server part on database instance and client part on your Gitea server. In addition, make sure you use same engine version for both server and client for some engine features to work. For security reason, protect `root` (MySQL) or `postgres` (PostgreSQL) database superuser with secure password.  The steps assumes that you run Linux for both database and Gitea servers.

## MySQL

1.  On database instance, login to database console as root:

    ```
    mysql -u root -p
    ```

    Enter the password as prompted.

2.  Create database user which will be used by Gitea, authenticated by password. This example uses `'gitea'` as password. Please use a secure password for your instance. 

    For local database:

    ```sql
    SET old_passwords=0;
    CREATE USER 'gitea' IDENTIFIED BY 'gitea';
    ```

    For remote database:

    ```sql
    SET old_passwords=0;
    CREATE USER 'gitea'@'12.34.56.78' IDENTIFIED BY 'gitea';
    ```

    where `12.34.56.78` is the IP address of your Gitea instance.

    Replace username and password above as appropriate.

3.  Create database with UTF-8 charset and collation. Make sure to use `utf8mb4` charset instead of `utf8` as the former supports all Unicode characters (including emojis) beyond *Basic Multilingual Plane*. Also, collation chosen depending on your expected content. When in doubt, use either `unicode_ci` or `general_ci`.

    ```sql
    CREATE DATABASE giteadb CHARACTER SET 'utf8mb4' COLLATE 'utf8mb4_unicode_ci';
    ```

    Replace database name as appropriate.

4.  Grant all privileges on the database to database user created above.

    For local database:

    ```sql
    GRANT ALL PRIVILEGES ON giteadb.* TO 'gitea';
    FLUSH PRIVILEGES;
    ```

    For remote database:

    ```sql
    GRANT ALL PRIVILEGES ON giteadb.* TO 'gitea'@'12.34.56.78';
    FLUSH PRIVILEGES;
    ```

5.  Quit from database console by `exit`.

6.  On your Gitea server, test connection to the database:

    ```
    mysql -u gitea -h 23.45.67.89 -p giteadb
    ```

    where `gitea` is database username, `giteadb` is database name, and `23.45.67.89` is IP address of database instance. Omit `-h` option for local database.

    You should be connected to the database.

## PostgreSQL

1.  PostgreSQL uses `md5` challenge-response encryption scheme for password authentication by default. Nowadays this scheme is not considered secure anymore. Use SCRAM-SHA-256 scheme instead by editing the `postgresql.conf` configuration file on the database server to:

    ```ini
    password_encryption = scram-sha-256
    ```

    Restart PostgreSQL to apply the setting.

2.  On the database server, login to the database console as superuser:

    ```
    su -c "psql" - postgres
    ```

3.  Create database user (role in PostgreSQL terms) with login privilege and password. Please use a secure, strong password instead of `'gitea'` below:

    ```sql
    CREATE ROLE gitea WITH LOGIN PASSWORD 'gitea';
    ```

    Replace username and password as appropriate.

4.  Create database with UTF-8 charset and owned by the database user created earlier. Any `libc` collations can be specified with `LC_COLLATE` and `LC_CTYPE` parameter, depending on expected content:

    ```sql
    CREATE DATABASE giteadb WITH OWNER gitea TEMPLATE template0 ENCODING UTF8 LC_COLLATE 'en_US.UTF-8' LC_CTYPE 'en_US.UTF-8';
    ```

    Replace database name as appropriate.

5.  Allow the database user to access the database created above by adding the following authentication rule to `pg_hba.conf`.

    For local database:

    ```ini
    local    giteadb    gitea    scram-sha-256
    ```

    For remote database:

    ```ini
    host    giteadb    gitea    12.34.56.78/32    scram-sha-256
    ```

    Replace database name, user, and IP address of Gitea instance with your own.

    Note: rules on `pg_hba.conf` are evaluated sequentially, that is the first matching rule will be used for authentication. Your PostgreSQL installation may come with generic authentication rules that match all users and databases. You may need to place the rules presented here above such generic rules if it is the case.

    Restart PostgreSQL to apply new authentication rules.
    
6.  On your Gitea server, test connection to the database.

    For local database:

    ```
    psql -U gitea -d giteadb
    ```

    For remote database:

    ```
    psql "postgres://gitea@23.45.67.89/giteadb"
    ```

    where `gitea` is database user, `giteadb` is database name, and `23.45.67.89` is IP address of your database instance.

    You should be prompted to enter password for the database user, and connected to the database.
