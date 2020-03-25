## Contributing to xorm

`xorm` has a backlog of [pull requests](https://help.github.com/articles/using-pull-requests), but contributions are still very
much welcome. You can help with patch review, submitting bug reports,
or adding new functionality. There is no formal style guide, but
please conform to the style of existing code and general Go formatting
conventions when submitting patches.

* [fork a repo](https://help.github.com/articles/fork-a-repo)
* [creating a pull request ](https://help.github.com/articles/creating-a-pull-request)

### Language

Since `xorm` is a world-wide open source project, please describe your issues or code changes in English as soon as possible.

### Sign your codes with comments
```
// !<you github id>! your comments

e.g.,

// !lunny! this is comments made by lunny
```

### Build xorm and test it locally

Once you write some codes on your feature branch, you could build and test locally at first. Just

```
make build
```
and
```
make test
```

The `make test` is an alias of `make test-sqlite`, it will run the tests on a sqlite database file. No extra thing needed to do except you need to cgo compile enviroment.

If you write a new test method, you could run

```
make test-sqlite#TestMyNewMethod
```

that will only run the special test method.

If you want to run another datase, you have to prepare a running database at first, and then, you could

```
TEST_MYSQL_HOST= TEST_MYSQL_CHARSET= TEST_MYSQL_DBNAME= TEST_MYSQL_USERNAME= TEST_MYSQL_PASSWORD= make test-mysql
```

or other databases:
```
TEST_MSSQL_HOST= TEST_MSSQL_DBNAME= TEST_MSSQL_USERNAME= TEST_MSSQL_PASSWORD= make test-mssql
```
```
TEST_PGSQL_HOST= TEST_PGSQL_SCHEMA= TEST_PGSQL_DBNAME= TEST_PGSQL_USERNAME= TEST_PGSQL_PASSWORD= make test-postgres
```
```
TEST_TIDB_HOST= TEST_TIDB_DBNAME= TEST_TIDB_USERNAME= TEST_TIDB_PASSWORD= make test-tidb
```

And if your branch is related with cache, you could also enable it via `TEST_CACHE_ENABLE=true`.

### Patch review

Help review existing open [pull requests](https://help.github.com/articles/using-pull-requests) by commenting on the code or
proposed functionality.

### Bug reports

We appreciate any bug reports, but especially ones with self-contained
(doesn't depend on code outside of xorm), minimal (can't be simplified
further) test cases. It's especially helpful if you can submit a pull
request with just the failing test case(you can find some example test file like [session_get_test.go](https://gitea.com/xorm/xorm/src/branch/master/session_get_test.go)).

If you implements a new database interface, you maybe need to add a test_<databasename>.sh file.
For example, [mysql_test.go](https://gitea.com/xorm/xorm/src/branch/master/test_mysql.sh)

### New functionality

There are a number of pending patches for new functionality, so
additional feature patches will take a while to merge. Still, patches
are generally reviewed based on usefulness and complexity in addition
to time-in-queue, so if you have a knockout idea, take a shot. Feel
free to open an issue discussion your proposed patch beforehand.
