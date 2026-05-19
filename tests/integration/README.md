# Integration tests

Integration tests can be run with command `make test-integration`.
Environment variable `GITEA_TEST_DATABASE` can be used to specify the database type for testing.

If you encounter some errors like mismatched database version, SSH push errors, etc.,
you can try to perform a clean build by: `make clean build`.

## Run sqlite integration tests

Start tests directly (empty `GITEA_TEST_DATABASE` defaults to sqlite):

```
make test-integration
```

## Run MySQL integration tests

Set up a MySQL database inside docker:

```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:latest #(just ctrl-c to stop db and clean the container)
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" --rm --name elasticsearch elasticsearch:7.6.0 #(in a second terminal, just ctrl-c to stop db and clean the container)
```

Start tests based on the database container:

```
GITEA_TEST_DATABASE=mysql TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-integration
```

## Run pgsql integration tests

Set up a pgsql database inside docker:

```
docker run -e "POSTGRES_DB=test" -e "POSTGRES_USER=postgres" -e "POSTGRES_PASSWORD=postgres" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```

Set up minio inside docker:

```
docker run --rm -p 9000:9000 -e MINIO_ROOT_USER=123456 -e MINIO_ROOT_PASSWORD=12345678 --name minio bitnamilegacy/minio:2023.8.31
```

Start tests based on the database container:

```
GITEA_TEST_DATABASE=pgsql TEST_MINIO_ENDPOINT=localhost:9000 TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=postgres TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-integration
```

## Run mssql integration tests

Set up a mssql database inside docker:

```
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest #(just ctrl-c to stop db and clean the container)
```

Start tests based on the database container:

```
GITEA_TEST_DATABASE=mssql TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-integration
```

## Running individual tests

Example command to run GPG test:

```
GITEA_TEST_DATABASE=... make test-integration#GPG
```

## Run Gitea Actions tests via local act_runner

### Run all jobs

```
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest
```

Warning: This file defines many jobs, so it will be resource-intensive and therefore not recommended.

### Run single job

```SHELL
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -j <job_name>
```

You can list all job names via:

```SHELL
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -l
```
