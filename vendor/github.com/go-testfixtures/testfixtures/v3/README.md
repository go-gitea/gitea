# testfixtures

[![PkgGoDev](https://pkg.go.dev/badge/github.com/go-testfixtures/testfixtures/v3?tab=doc)](https://pkg.go.dev/github.com/go-testfixtures/testfixtures/v3?tab=doc)

> ***Warning***: this package will wipe the database data before loading the
fixtures! It is supposed to be used on a test database. Please, double check
if you are running it against the correct database.

> **TIP**: There are options not described in this README page. It's
> recommended that you also check [the documentation][doc].

Writing tests is hard, even more when you have to deal with an SQL database.
This package aims to make writing functional tests for web apps written in
Go easier.

Basically this package mimics the ["Ruby on Rails' way"][railstests] of writing tests
for database applications, where sample data is kept in fixtures files. Before
the execution of every test, the test database is cleaned and the fixture data
is loaded into the database.

The idea is running tests against a real database, instead of relying in mocks,
which is boring to setup and may lead to production bugs not being caught in
the tests.

## Installation

First, import it like this:

```go
import (
        "github.com/go-testfixtures/testfixtures/v3"
)
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
  created_at: 2020-12-31 23:59:59
  updated_at: 2020-12-31 23:59:59

- id: 2
  post_id: 2
  content: Another comment...
  author_name: John Doe
  author_email: john@doe.com
  created_at: 2020-12-31 23:59:59
  updated_at: 2020-12-31 23:59:59

# ...
```

An YAML object or array will be converted to JSON. It will be stored on a native
JSON type like JSONB on PostgreSQL & CockroachDB or as a TEXT or VARCHAR column on other
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

Binary columns can be represented as hexadecimal strings (should start with `0x`):

```yaml
- id: 1
  binary_column: 0x1234567890abcdef
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

        _ "github.com/lib/pq"
        "github.com/go-testfixtures/testfixtures/v3"
)

var (
        db *sql.DB
        fixtures *testfixtures.Loader
)

func TestMain(m *testing.M) {
        var err error

        // Open connection to the test database.
        // Do NOT import fixtures in a production database!
        // Existing data would be deleted.
        db, err = sql.Open("postgres", "dbname=myapp_test")
        if err != nil {
                ...
        }

        fixtures, err = testfixtures.New(
                testfixtures.Database(db), // You database connection
                testfixtures.Dialect("postgres"), // Available: "postgresql", "timescaledb", "mysql", "mariadb", "sqlite" and "sqlserver"
                testfixtures.Directory("testdata/fixtures"), // The directory containing the YAML files
        )
        if err != nil {
                ...
        }

        os.Exit(m.Run())
}

func prepareTestDatabase() {
        if err := fixtures.Load(); err != nil {
                ...
        }
}

func TestX(t *testing.T) {
        prepareTestDatabase()

        // Your test here ...
}

func TestY(t *testing.T) {
        prepareTestDatabase()

        // Your test here ...
}

func TestZ(t *testing.T) {
        prepareTestDatabase()

        // Your test here ...
}
```

Alternatively, you can use the `Files` option, to specify which
files you want to load into the database:

```go
fixtures, err := testfixtures.New(
        testfixtures.Database(db),
        testfixtures.Dialect("postgres"),
        testfixtures.Files(
                "fixtures/orders.yml",
                "fixtures/customers.yml",
        ),
)
if err != nil {
        ...
}
```

With `Paths` option, you can specify the paths that fixtures will load
from. Path can be directory or file. If directory, we will search YAML files
in it.

```go
fixtures, err := testfixtures.New(
        testfixtures.Database(db),
        testfixtures.Dialect("postgres"),
        testfixtures.Paths(
                "fixtures/orders.yml",
                "fixtures/customers.yml",
                "common_fixtures/users"
        ),
)
if err != nil {
        ...
}
```

## Security check

In order to prevent you from accidentally wiping the wrong database, this
package will refuse to load fixtures if the database name (or database
filename for SQLite) doesn't contains "test". If you want to disable this
check, use:

```go
testfixtures.New(
        ...
        testfixtures.DangerousSkipTestDatabaseCheck(),
)
```

## Sequences

For PostgreSQL and MySQL/MariaDB, this package also resets all
sequences to a high number to prevent duplicated primary keys while
running the tests.
The default is 10000, but you can change that with:

```go
testfixtures.New(
        ...
        testfixtures.ResetSequencesTo(10000),
)
```

Or, if you want to skip the reset of sequences entirely:

```go
testfixtures.New(
        ...
        testfixtures.SkipResetSequences(),
)
```

## Compatible databases

### PostgreSQL / TimescaleDB / CockroachDB

This package has three approaches to disable foreign keys while importing fixtures
for PostgreSQL databases:

#### With `DISABLE TRIGGER`

This is the default approach. For that use:

```go
testfixtures.New(
        ...
        testfixtures.Dialect("postgres"), // or "timescaledb"
)
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
testfixtures.New(
        ...
        testfixtures.Dialect("postgres"),
        testfixtures.UseAlterConstraint(),
)
```

#### With `DROP CONSTRAINT`

This approach is implemented to support databases that do not support above
methods (namely CockroachDB).

```go
testfixtures.New(
        ...
        testfixtures.Dialect("postgres"),
        testfixtures.UseDropConstraint(),
)
```

Tested using the [github.com/lib/pq](https://github.com/lib/pq) and
[github.com/jackc/pgx](https://github.com/jackc/pgx) drivers.

### MySQL / MariaDB

Just make sure the connection string have
[the multistatement parameter](https://github.com/go-sql-driver/mysql#multistatements)
set to true, and use:

```go
testfixtures.New(
        ...
        testfixtures.Dialect("mysql"), // or "mariadb"
)
```

Tested using the [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) driver.

### SQLite

SQLite is also supported. It is recommended to create foreign keys as
`DEFERRABLE` (the default) to prevent problems. See more
[on the SQLite documentation](https://www.sqlite.org/foreignkeys.html#fk_deferred).
(Foreign key constraints are no-op by default on SQLite, but enabling it is
recommended).

```go
testfixtures.New(
        ...
        testfixtures.Dialect("sqlite"),
)
```

Tested using the [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) driver.

### Microsoft SQL Server

SQL Server support requires SQL Server >= 2008. Inserting on `IDENTITY` columns
are handled as well. Just make sure you are logged in with a user with
`ALTER TABLE` permission.

```go
testfixtures.New(
        ...
        testfixtures.Dialect("sqlserver"),
)
```

Tested using the `mssql` and `sqlserver` drivers from the
[github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb) lib.

## Templating

Testfixtures supports templating, but it's disabled by default. Most people
won't need it, but it may be useful to dynamically generate data.

Enable it by doing:

```go
testfixtures.New(
        ...
        testfixtures.Template(),

        // the above options are optional
        TemplateFuncs(...),
        TemplateDelims("{{", "}}"),
        TemplateOptions("missingkey=zero"),
        TemplateData(...),
)
```

The YAML file could look like this:

```yaml
# It's possible generate values...
- id: {{sha256 "my-awesome-post}}
  title: My Awesome Post
  text: {{randomText}}

# ... or records
{{range $post := $.Posts}}
- id: {{$post.Id}}
  title: {{$post.Title}}
  text: {{$post.Text}}
{{end}}
```

## Generating fixtures for a existing database

The following code will generate a YAML file for each table of the database
into a given folder. It may be useful to boostrap a test scenario from a sample
database of your app.

```go
dumper, err := testfixtures.NewDumper(
        testfixtures.DumpDatabase(db),
        testfixtures.DumpDialect("postgres"), // or your database of choice
        testfixtures.DumpDirectory("tmp/fixtures"),
        testfixtures.DumpTables( // optional, will dump all table if not given
          "posts",
          "comments",
          "tags",
        ),
)
if err != nil {
        ...
}
if err := dumper.Dump(); err != nil {
        ...
}
```

> This was intended to run in small sample databases. It will likely break
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

If you're looking into being able to run tests in parallel you can try using
testfixtures together with the [txdb][gotxdb] package, which allows wrapping
each test run in a transaction.

## CLI

We also have a CLI to load fixtures in a given database.

Grab it from the [releases page](https://github.com/go-testfixtures/testfixtures/releases)
or install with Homebrew:

```bash
brew install go-testfixtures/tap/testfixtures
```

Usage is like this:

```bash
# load
testfixtures -d postgres -c "postgres://user:password@localhost/database" -D testdata/fixtures
```

```bash
# dump
testfixtures --dump -d postgres -c "postgres://user:password@localhost/database" -D testdata/fixtures
```

The connection string changes for each database driver.

Use `testfixtures --help` for all flags.

## Contributing

We recommend you to [install Task](https://taskfile.dev/#/installation) and
Docker before contributing to this package, since some stuff is automated
using these tools.

It's recommended to use Docker Compose to run tests, since it runs tests for
all supported databases once. To do that you just need to run:

```bash
task docker
```

But if you want to run tests locally, copy the `.sample.env` file as `.env`
and edit it according to your database setup. You'll need to create a database
(likely names `testfixtures_test`) before continuing. Then run the command
for the database you want to run tests against:

```bash
task test:pg # PostgreSQL
task test:crdb # CockroachDB
task test:mysql # MySQL
task test:sqlite # SQLite
task test:sqlserver # Microsoft SQL Server
```

GitHub Actions (CI) runs the same Docker setup available locally.

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

[doc]: https://pkg.go.dev/github.com/go-testfixtures/testfixtures/v3?tab=doc
[railstests]: http://guides.rubyonrails.org/testing.html#the-test-database
[gotxdb]: https://github.com/DATA-DOG/go-txdb
[gosqlmock]: https://github.com/DATA-DOG/go-sqlmock
[factorygo]: https://github.com/bluele/factory-go
[dbcleaner]: https://github.com/khaiql/dbcleaner
