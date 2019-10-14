# Go Test Fixtures

[![GoDoc](https://godoc.org/gopkg.in/testfixtures.v2?status.svg)](https://godoc.org/gopkg.in/testfixtures.v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-testfixtures/testfixtures)](https://goreportcard.com/report/github.com/go-testfixtures/testfixtures)
[![Build Status](https://travis-ci.org/go-testfixtures/testfixtures.svg?branch=master)](https://travis-ci.org/go-testfixtures/testfixtures)
[![Build status](https://ci.appveyor.com/api/projects/status/d2h6gq37wxbus1x7?svg=true)](https://ci.appveyor.com/project/andreynering/testfixtures)

> ***Warning***: this package will wipe the database data before loading the
fixtures! It is supposed to be used on a test database. Please, double check
if you are running it against the correct database.

Writing tests is hard, even more when you have to deal with an SQL database.
This package aims to make writing functional tests for web apps written in
Go easier.

Basically this package mimics the ["Rails' way"][railstests] of writing tests
for database applications, where sample data is kept in fixtures files. Before
the execution of every test, the test database is cleaned and the fixture data
is loaded into the database.

The idea is running tests against a real database, instead of relying in mocks,
which is boring to setup and may lead to production bugs not being caught in
the tests.

## Installation

First, get it:

```bash
go get -u -v gopkg.in/testfixtures.v2
```

## Usage

Create a folder for the fixture files. Each file should contain data for a
single table and have the name `<table_name>.yml`:

```
myapp/
  myapp.go
  myapp_test.go
  ...
  fixtures/
    posts.yml
    comments.yml
    tags.yml
    posts_tags.yml
    ...
```

The file would look like this (it can have as many record you want):

```yml
# comments.yml
- id: 1
  post_id: 1
  content: A comment...
  author_name: John Doe
  author_email: john@doe.com
  created_at: 2016-01-01 12:30:12
  updated_at: 2016-01-01 12:30:12

- id: 2
  post_id: 2
  content: Another comment...
  author_name: John Doe
  author_email: john@doe.com
  created_at: 2016-01-01 12:30:12
  updated_at: 2016-01-01 12:30:12

# ...
```

An YAML object or array will be converted to JSON. It can be stored on a native
JSON type like JSONB on PostgreSQL or as a TEXT or VARCHAR column on other
databases.

```yml
- id: 1
  post_attributes:
    author: John Due
    author_email: john@due.com
    title: "..."
    tags:
      - programming
      - go
      - testing
    post: "..."
```

If you need to write raw SQL, probably to call a function, prefix the value
of the column with `RAW=`:

```yml
- id: 1
  uuid_column: RAW=uuid_generate_v4()
  postgis_type_column: RAW=ST_GeomFromText('params...')
  created_at: RAW=NOW()
  updated_at: RAW=NOW()
```

Your tests would look like this:

```go
package myapp

import (
    "database/sql"
    "log"

    _ "github.com/lib/pq"
    "gopkg.in/testfixtures.v2"
)

var (
    db *sql.DB
    fixtures *testfixtures.Context
)

func TestMain(m *testing.M) {
    var err error

    // Open connection with the test database.
    // Do NOT import fixtures in a production database!
    // Existing data would be deleted
    db, err = sql.Open("postgres", "dbname=myapp_test")
    if err != nil {
        log.Fatal(err)
    }

    // creating the context that hold the fixtures
    // see about all compatible databases in this page below
    fixtures, err = testfixtures.NewFolder(db, &testfixtures.PostgreSQL{}, "testdata/fixtures")
    if err != nil {
        log.Fatal(err)
    }

    os.Exit(m.Run())
}

func prepareTestDatabase() {
    if err := fixtures.Load(); err != nil {
        log.Fatal(err)
    }
}

func TestX(t *testing.T) {
    prepareTestDatabase()
    // your test here ...
}

func TestY(t *testing.T) {
    prepareTestDatabase()
    // your test here ...
}

func TestZ(t *testing.T) {
    prepareTestDatabase()
    // your test here ...
}
```

Alternatively, you can use the `NewFiles` function, to specify which
files you want to load into the database:

```go
fixtures, err := testfixtures.NewFiles(db, &testfixtures.PostgreSQL{},
    "fixtures/orders.yml",
    "fixtures/customers.yml",
    // add as many files you want
)
if err != nil {
    log.Fatal(err)
}
```

## Security check

In order to prevent you from accidentally wiping the wrong database, this
package will refuse to load fixtures if the database name (or database
filename for SQLite) doesn't contains "test". If you want to disable this
check, use:

```go
testfixtures.SkipDatabaseNameCheck(true)
```

## Sequences

For PostgreSQL or Oracle, this package also resets all sequences to a high
number to prevent duplicated primary keys while running the tests.
The default is 10000, but you can change that with:

```go
testfixtures.ResetSequencesTo(10000)
```

## Compatible databases

### PostgreSQL

This package has two approaches to disable foreign keys while importing fixtures
in PostgreSQL databases:

#### With `DISABLE TRIGGER`

This is the default approach. For that use:

```go
&testfixtures.PostgreSQL{}
```

With the above snippet this package will use `DISABLE TRIGGER` to temporarily
disabling foreign key constraints while loading fixtures. This work with any
version of PostgreSQL, but it is **required** to be connected in the database
as a SUPERUSER. You can make a PostgreSQL user a SUPERUSER with:

```sql
ALTER USER your_user SUPERUSER;
```

#### With `ALTER CONSTRAINT`

This approach don't require to be connected as a SUPERUSER, but only work with
PostgreSQL versions >= 9.4. Try this if you are getting foreign key violation
errors with the previous approach. It is as simple as using:

```go
&testfixtures.PostgreSQL{UseAlterConstraint: true}
```

#### Skipping reset of sequences

You can skip the reset of PostgreSQL sequences if you're debugging a problem
with it, but most of the time you shouldn't do it:

```go
&testfixtures.PostgreSQL{SkipResetSequences: true}
```

### MySQL / MariaDB

Just make sure the connection string have
[the multistatement parameter](https://github.com/go-sql-driver/mysql#multistatements)
set to true, and use:

```go
&testfixtures.MySQL{}
```

### SQLite

SQLite is also supported. It is recommended to create foreign keys as
`DEFERRABLE` (the default) to prevent problems. See more
[on the SQLite documentation](https://www.sqlite.org/foreignkeys.html#fk_deferred).
(Foreign key constraints are no-op by default on SQLite, but enabling it is
recommended).

```go
&testfixtures.SQLite{}
```

### Microsoft SQL Server

SQL Server support requires SQL Server >= 2008. Inserting on `IDENTITY` columns
are handled as well. Just make sure you are logged in with a user with
`ALTER TABLE` permission.

```go
&testfixtures.SQLServer{}
```

### Oracle

Oracle is supported as well. Use:

```go
&testfixtures.Oracle{}
```

## Generating fixtures for a existing database (experimental)

The following code will generate a YAML file for each table of the database in
the given folder. It may be useful to boostrap a test scenario from a sample
database of your app.

```go
err := testfixtures.GenerateFixtures(db, &testfixtures.PostgreSQL{}, "testdata/fixtures")
if err != nil {
    log.Fatalf("Error generating fixtures: %v", err)
}
```

Or

```go
err := testfixtures.GenerateFixturesForTables(
    db,
    []*TableInfo{
        &TableInfo{Name: "table_name", Where: "foo = 'bar'"},
        // ...
    },
    &testfixtures.PostgreSQL{},
    "testdata/fixtures",
)
if err != nil {
    log.Fatalf("Error generating fixtures: %v", err)
}
```

> This was thought to run in small sample databases. It will likely break
if run in a production/big database.

## Gotchas

### Parallel testing

This library doesn't yet support running tests in parallel! Running tests
in parallel can result in random data being present in the database, which
will likely cause tests to randomly/intermittently fail.

This is specially tricky since it's not immediately clear that `go test ./...`
run tests for each package in parallel. If more than one package use this
library, you can face this issue. Please, use `go test -p 1 ./...` or run tests
for each package in separated commands to fix this issue.

See [#40](https://github.com/go-testfixtures/testfixtures/issues/40)
and [golang/go#11521](https://github.com/golang/go/issues/11521) for more information.

We're also planning to implement transactional tests to allow running tests
in parallel (see [#24](https://github.com/go-testfixtures/testfixtures/issues/24)).
Running each test package in a separated database would also allow you to do that.
Open issues for other ideas :slightly_smiling_face:.

## Contributing

Tests were written to ensure everything work as expected. You can run the tests
with:

```bash
# running tests for PostgreSQL
go test -tags postgresql

# running test for MySQL
go test -tags mysql

# running tests for SQLite
go test -tags sqlite

# running tests for SQL Server
go test -tags sqlserver

# running tests for Oracle
go test -tags oracle

# running test for multiple databases at once
go test -tags 'sqlite postgresql mysql'

# running tests + benchmark
go test -v -bench=. -tags postgresql
```

Travis runs tests for PostgreSQL, MySQL and SQLite. AppVeyor run for all
these and also Microsoft SQL Server.

To set the connection string of tests for each database, copy the `.sample.env`
file as `.env` and edit it according to your environment.

## Alternatives

If you don't think using fixtures is a good idea, you can try one of these
packages instead:

- [factory-go][factorygo]: Factory for Go. Inspired by Python's Factory Boy
and Ruby's Factory Girl
- [go-txdb (Single transaction SQL driver for Go)][gotxdb]: Use a single
database transaction for each functional test, so you can rollback to
previous state between tests to have the same database state in all tests
- [go-sqlmock][gosqlmock]: A mock for the sql.DB interface. This allow you to
unit test database code without having to connect to a real database
- [dbcleaner][dbcleaner] - Clean database for testing, inspired by
database_cleaner for Ruby

[railstests]: http://guides.rubyonrails.org/testing.html#the-test-database
[gotxdb]: https://github.com/DATA-DOG/go-txdb
[gosqlmock]: https://github.com/DATA-DOG/go-sqlmock
[factorygo]: https://github.com/bluele/factory-go
[dbcleaner]: https://github.com/khaiql/dbcleaner
