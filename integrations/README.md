# Integrations tests

Integration tests can be run with make commands for the
appropriate backends, namely:
```shell
make test-mysql
make test-pgsql
make test-sqlite
```

Make sure to perform a clean build before running tests:
```
make clean build
```

## Run all tests via local drone
```
drone exec --local --build-event "pull_request"
```

## Run sqlite integrations tests
Start tests 
```
make test-sqlite
```

## Run mysql integrations tests
Setup a mysql database inside docker
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" --rm --name mysql mysql:5.7 #(just ctrl-c to stop db and clean the container) 
```
Start tests based on the database container
```
TEST_MYSQL_HOST="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' mysql):3306" TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-mysql
```

## Run pgsql integrations tests
Setup a pgsql database inside docker
```
docker run -e "POSTGRES_DB=test" --rm --name pgsql postgres:9.5 #(just ctrl-c to stop db and clean the container) 
```
Start tests based on the database container
```
TEST_PGSQL_HOST=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pgsql) TEST_PGSQL_DBNAME=test TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## Running individual tests

Example command to run GPG test with sqlite backend:
```
go test -c code.gitea.io/gitea/integrations \
  -o integrations.sqlite.test -tags 'sqlite' &&
  GITEA_ROOT="$GOPATH/src/code.gitea.io/gitea" \
  GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test \
  -test.v -test.run GPG
```

