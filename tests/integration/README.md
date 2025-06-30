# Integration tests

Integration tests can be run with make commands for the
appropriate backends, namely:
```shell
make test-sqlite
make test-pgsql
make test-mysql
make test-mssql
```

Make sure to perform a clean build before running tests:
```
make clean build
```

## Run tests via local act_runner

### Run all jobs

```
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest
```

Warning: This file defines many jobs, so it will be resource-intensive and therefor not recommended.

### Run single job

```SHELL
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -j <job_name>
```

You can list all job names via:

```SHELL
act_runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -l
```

## Run sqlite integration tests
Start tests
```
make test-sqlite
```

## Run MySQL integration tests
Setup a MySQL database inside docker
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:latest #(just ctrl-c to stop db and clean the container)
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" --rm --name elasticsearch elasticsearch:7.6.0 #(in a second terminal, just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-mysql
```

## Run pgsql integration tests
Setup a pgsql database inside docker
```
docker run -e "POSTGRES_DB=test" -e "POSTGRES_USER=postgres" -e "POSTGRES_PASSWORD=postgres" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```
Setup minio inside docker
```
docker run --rm -p 9000:9000 -e MINIO_ROOT_USER=123456 -e MINIO_ROOT_PASSWORD=12345678 --name minio bitnami/minio:2023.8.31
```
Start tests based on the database container
```
TEST_MINIO_ENDPOINT=localhost:9000 TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=postgres TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## Run mssql integration tests
Setup a mssql database inside docker
```
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest #(just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql
```

## Running individual tests

Example command to run GPG test:

For SQLite:

```
make test-sqlite#GPG
```

For other databases(replace `mssql` to `mysql`, or `pgsql`):

```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql#GPG
```

## Setting timeouts for declaring long-tests and long-flushes

We appreciate that some testing machines may not be very powerful and
the default timeouts for declaring a slow test or a slow clean-up flush
may not be appropriate.

You can set the following environment variables:

```bash
GITEA_TEST_SLOW_RUN="10s" GITEA_TEST_SLOW_FLUSH="1s" make test-sqlite
```
