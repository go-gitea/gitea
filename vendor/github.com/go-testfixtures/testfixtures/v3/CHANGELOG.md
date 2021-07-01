# Changelog

## v3.6.1 - 2021-05-20

- Fix possible security vulnerability by upgrading golang.org/x/crypto
  ([#100](https://github.com/go-testfixtures/testfixtures/pull/100)).

## v3.6.0 - 2021-04-17

- Add support for dumping a database using the CLI (use the `--dump` flag)
  ([#88](https://github.com/go-testfixtures/testfixtures/pull/88), [#63](https://github.com/go-testfixtures/testfixtures/issues/63)).
- Support SkipResetSequences and ResetSequencesTo for MySQL and MariaDB
  ([#91](https://github.com/go-testfixtures/testfixtures/pull/91)).

## v3.5.0 - 2021-01-11

- Fix insert of JSON values on PostgreSQL when using `binary_parameters=yes` in
  the connection string
  ([#83](https://github.com/go-testfixtures/testfixtures/issues/83), [#84](https://github.com/go-testfixtures/testfixtures/pull/84), [lib/pq#528](https://github.com/lib/pq/issues/528)).
- Officially support binary columns through hexadecimal strings
  ([#48](https://github.com/go-testfixtures/testfixtures/issues/48), [#82](https://github.com/go-testfixtures/testfixtures/pull/82)).

## v3.4.1 - 2020-10-19

- Fix for Microsoft SQL Server databases with views
  ([#78](https://github.com/go-testfixtures/testfixtures/pull/78)).

## v3.4.0 - 2020-08-09

- Add support to CockroachDB
  ([#77](https://github.com/go-testfixtures/testfixtures/pull/77)).

## v3.3.0 - 2020-06-27

- Add support for the [github.com/jackc/pgx](https://github.com/jackc/pgx)
  PostgreSQL driver
  ([#71](https://github.com/go-testfixtures/testfixtures/issues/71), [#74](https://github.com/go-testfixtures/testfixtures/pull/74)).
- Fix bug where some tables were empty due to `ON DELETE CASCADE`
  ([#67](https://github.com/go-testfixtures/testfixtures/issues/67), [#70](https://github.com/go-testfixtures/testfixtures/pull/70)).
- Fix SQLite version
  ([#73](https://github.com/go-testfixtures/testfixtures/pull/73)).
- On MySQL, return a clearer error message when a table doesn't exist
  ([#69](https://github.com/go-testfixtures/testfixtures/pull/69)).

## v3.2.0 - 2020-05-10

- Add support for loading multiple files and directories
  ([#65](https://github.com/go-testfixtures/testfixtures/pull/65)).

## v3.1.2 - 2020-04-26

- Dump: Fix column order in generated YAML files
  ([#62](https://github.com/go-testfixtures/testfixtures/pull/62)).

## v3.1.1 - 2020-01-11

- testfixtures now work with both `mssql` and `sqlserver` drivers.
  Note that [the `mssql` one is deprecated](https://github.com/denisenkom/go-mssqldb#deprecated),
  though. So try to migrate to `sqlserver` once possible.

## v3.1.0 - 2020-01-09

- Using `sqlserver` driver instead of the deprecated `mssql`
  ([#58](https://github.com/go-testfixtures/testfixtures/pull/58)).

## v3.0.0 - 2019-12-26

### Breaking changes

- The import path changed from `gopkg.in/testfixtures.v2` to
  `github.com/go-testfixtures/testfixtures/v3`.
- This package no longer support Oracle databases. This decision was
  taken because too few people actually used this package with Oracle and it
  was the most difficult to test (we didn't run on CI due the lack of an
  official Docker image, etc).
- The public API was totally rewritten to be more flexible and ideomatic.
  It now uses functional options. It differs from v2, but should be easy
  enough to upgrade.
- Some deprecated APIs from v2 were removed as well.
- This now requires Go >= 1.13.

### New features

- We now have a CLI so you can easily use testfixtures to load a sample
  database from fixtures if you want.
- Templating via [text/template](https://golang.org/pkg/text/template/)
  is now available. This allows some fancier use cases like generating data
  or specific columns dynamically.
- It's now possible to choose which time zone to use when parsing timestamps
  from fixtures. The default is the same as before, whatever is set on
  `time.Local`.
- Errors now use the new `%w` verb only available on Go >= 1.13.

### MISC

- Travis and AppVeyor are gone. We're using GitHub Actions exclusively now.
  The whole suite is ran inside Docker (with help of Docker Compose), so it's
  easy to run tests locally as well.

Check the new README for some examples!

## v2.6.0 - 2019-10-24

- Add support for TimescaleDB
  ([#53](https://github.com/go-testfixtures/testfixtures/pull/53)).

## v2.5.3 - 2018-12-15

- Fixes related to use of foreign key pragmas on MySQL (#43).

## v2.5.2 - 2018-11-25

- This library now supports [Go Modules](https://github.com/golang/go/wiki/Modules);
- Also allow `.yaml` (as an alternative to `.yml`) as the file extension (#42).

## v2.5.1 - 2018-11-04

- Allowing disabling reset of PostgreSQL sequences (#38).

## v2.5.0 - 2018-09-07

- Add public function DetectTestDatabase (#35, #36).

## v2.4.5 - 2018-07-07

- Fix for MySQL/MariaDB: ignoring views on operations that should be run only on tables (#33).

## v2.4.4 - 2018-07-02

- Fix for multiple schemas on Microsoft SQL Server (#29 and #30);
- Configuring AppVeyor CI to also test for Microsoft SQL Server.

---

Sorry, we don't have changelog for older releases 😢.
