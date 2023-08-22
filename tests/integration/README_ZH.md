# 关于集成测试

使用如下 make 命令可以运行指定的集成测试：
```shell
make test-mysql
make test-pgsql
make test-sqlite
```

在执行集成测试命令前请确保清理了之前的构建环境，清理命令如下：
```
make clean build
```

## 如何在本地 act_runner 上运行测试

### 运行所有任务

```
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest
```

警告:由于在此文件中定义了许多任务，因此此操作将花费太多的CPU和内存来运行。所以不建议这样做。

### 运行单个任务

```SHELL
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -j <job_name>
```

您可以通过以下方式列出所有任务名称:
```SHELL
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -l
```

## 如何使用 sqlite 数据库进行集成测试
使用该命令执行集成测试
```
make test-sqlite
```

## 如何使用 mysql 数据库进行集成测试
首先在docker容器里部署一个 mysql 数据库
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:8 #(just ctrl-c to stop db and clean the container) 
```
之后便可以基于这个数据库进行集成测试
```
TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-mysql
```

## 如何使用 pgsql 数据库进行集成测试
同上，首先在 docker 容器里部署一个 pgsql 数据库
```
docker run -e "POSTGRES_DB=test" -p 5432:5432 --rm --name pgsql postgres:14 #(just ctrl-c to stop db and clean the container) 
```
之后便可以基于这个数据库进行集成测试
```
TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=test TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## Run mssql integration tests
同上，首先在 docker 容器里部署一个 mssql 数据库
```
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest #(just ctrl-c to stop db and clean the container) 
```
之后便可以基于这个数据库进行集成测试
```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql
```

## 如何进行自定义的集成测试

下面的示例展示了怎样在集成测试中只进行 GPG 测试：

sqlite 数据库:

```
make test-sqlite#GPG
```

其它数据库(把 MSSQL 替换为 MYSQL, MYSQL8, PGSQL):

```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql#GPG
```

