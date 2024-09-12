# End to end tests

E2e tests largely follow the same syntax as [integration tests](../integration).
Whereas integration tests are intended to mock and stress the back-end, server-side code, e2e tests the interface between front-end and back-end, as well as visual regressions with both assertions and visual comparisons.
They can be run with make commands for the appropriate backends, namely:
```shell
make test-sqlite
make test-pgsql
make test-mysql
make test-mssql
```

Make sure to perform a clean front-end build before running tests:
```
make clean frontend
```

## Install playwright system dependencies
```
npx playwright install-deps
```


## Run all tests via local act_runner
```
act_runner exec -W ./.github/workflows/pull-e2e-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest
```

## Run sqlite e2e tests
Start tests
```
make test-e2e-sqlite
```

## Run MySQL e2e tests
Setup a MySQL database inside docker
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:latest #(just ctrl-c to stop db and clean the container)
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" --rm --name elasticsearch elasticsearch:7.6.0 #(in a second terminal, just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-e2e-mysql
```

## Run pgsql e2e tests
Setup a pgsql database inside docker
```
docker run -e "POSTGRES_DB=test" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=test TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-e2e-pgsql
```

## Run mssql e2e tests
Setup a mssql database inside docker
```
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest #(just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-e2e-mssql
```

## Running individual tests

Example command to run `example.test.e2e.ts` test file:

_Note: unlike integration tests, this filtering is at the file level, not function_

For SQLite:

```
make test-e2e-sqlite#example
```

For other databases(replace `mssql` to `mysql` or `pgsql`):

```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-e2e-mssql#example
```

## Visual testing

Although the main goal of e2e is assertion testing, we have added a framework for visual regress testing. If you are working on front-end features, please use the following:
 - Check out `main`, `make clean frontend`, and run e2e tests with `VISUAL_TEST=1` to generate outputs. This will initially fail, as no screenshots exist. You can run the e2e tests again to assert it passes.
 - Check out your branch, `make clean frontend`, and run e2e tests with `VISUAL_TEST=1`. You should be able to assert you front-end changes don't break any other tests unintentionally. 

VISUAL_TEST=1 will create screenshots in tests/e2e/test-snapshots. The test will fail the first time this is enabled (until we get visual test image persistence figured out), because it will be testing against an empty screenshot folder. 

ACCEPT_VISUAL=1 will overwrite the snapshot images with new images.
