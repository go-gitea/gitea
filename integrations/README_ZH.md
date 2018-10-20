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

## 如何在本地 drone 服务器上运行所有测试
```
drone exec --local --build-event "pull_request"
```

## 如何使用 sqlite 数据库进行集成测试
使用该命令执行集成测试
```
make test-sqlite
```

## 如何使用 mysql 数据库进行集成测试
首先在docker容器里部署一个 mysql 数据库
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" --rm --name mysql mysql:5.7 #(just ctrl-c to stop db and clean the container) 
```
之后便可以基于这个数据库进行集成测试
```
TEST_MYSQL_HOST="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' mysql):3306" TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-mysql
```

## 如何使用 pgsql 数据库进行集成测试
同上，首先在 docker 容器里部署一个 pgsql 数据库
```
docker run -e "POSTGRES_DB=test" --rm --name pgsql postgres:9.5 #(just ctrl-c to stop db and clean the container) 
```
之后便可以基于这个数据库进行集成测试
```
TEST_PGSQL_HOST=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pgsql) TEST_PGSQL_DBNAME=test TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## 如何进行自定义的集成测试

下面的示例展示了怎样基于 sqlite 数据库进行 GPG 测试：
```
go test -c code.gitea.io/gitea/integrations \
  -o integrations.sqlite.test -tags 'sqlite' &&
  GITEA_ROOT="$GOPATH/src/code.gitea.io/gitea" \
  GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test \
  -test.v -test.run GPG
```

