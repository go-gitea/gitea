# Change log

## v2.4.1 - 2021-02-25

- Fix for Go 1.16 os.IsNotExist wrapping
  ([#102](https://github.com/editorconfig/editorconfig-core-go/pull/102)).

## v2.4.0 - 2021-02-22

- Fix new core-test
  ([#100](https://github.com/editorconfig/editorconfig-core-go/pull/100));
- Upgrade github CI versions
  ([#99](https://github.com/editorconfig/editorconfig-core-go/pull/99));
- Upgrade x/mod v0.4.1
  ([#98](https://github.com/editorconfig/editorconfig-core-go/pull/98));
- Fix goreleaser deprecations
  ([#97](https://github.com/editorconfig/editorconfig-core-go/pull/97)).

## v2.3.10 - 2021-02-05

- Upgrade core-test
  ([#93](https://github.com/editorconfig/editorconfig-core-go/pull/93));
- Upgrade x/mod v0.4.0
  ([#94](https://github.com/editorconfig/editorconfig-core-go/pull/94));
- Upgrade golangci-lint to v1.34
  ([#95](https://github.com/editorconfig/editorconfig-core-go/pull/95)).

## v2.3.9 - 2020-11-28

- Fix path separator on Windows
  ([#69](https://github.com/editorconfig/editorconfig-core-go/pull/69));
- Upgrade go-cmp v0.5.4
  ([#91](https://github.com/editorconfig/editorconfig-core-go/pull/91)).

## v2.3.8 - 2020-10-17

- Feat more tests
  ([#83](https://github.com/editorconfig/editorconfig-core-go/pull/83));
- Upgrade go-ini v1.61.0
  ([#84](https://github.com/editorconfig/editorconfig-core-go/pull/84));
- Upgrade go-ini v1.62.0
  ([#85](https://github.com/editorconfig/editorconfig-core-go/pull/85)).

## v2.3.7 - 2020-09-05

- Upgrade go-ini v1.60.2, and go-cmp v0.5.2
  ([#81](https://github.com/editorconfig/editorconfig-core-go/pull/81)).

## v2.3.6 - 2020-08-25

- Use goerr113 linter
  ([#77](https://github.com/editorconfig/editorconfig-core-go/pull/77));
- Upgrade go-ini v1.60.0
  ([#78](https://github.com/editorconfig/editorconfig-core-go/pull/78));
- Upgrade go-ini v1.60.1
  ([#79](https://github.com/editorconfig/editorconfig-core-go/pull/79)).

## v2.3.5 - 2020-08-20

- Upgrade go-cmp v0.5.1
  ([#73](https://github.com/editorconfig/editorconfig-core-go/pull/73));
- Replace custom GitHub Action with official GolangCI Lint
  ([#74](https://github.com/editorconfig/editorconfig-core-go/pull/74));
- Upgrade go-ini v1.58.0
  ([#75](https://github.com/editorconfig/editorconfig-core-go/pull/75)).

## v2.3.4 - 2020-06-22

- Wrap errors using Go 1.13 syntax
  ([#61](https://github.com/editorconfig/editorconfig-core-go/pull/61));
- Upgrade base Docker image
  ([#68](https://github.com/editorconfig/editorconfig-core-go/pull/68));
- Upgrade go-ini v1.57.0, go-cmp v0.5.0
  ([#70](https://github.com/editorconfig/editorconfig-core-go/pull/70)).

## v2.3.3 - 2020-05-19

- Using goreleaser
  ([#22](https://github.com/editorconfig/editorconfig-core-go/pull/22));
- Upgrade go-cmp, go-ini, x/mod
  ([#60](https://github.com/editorconfig/editorconfig-core-go/pull/65));
- Update CI actions
  ([#63](https://github.com/editorconfig/editorconfig-core-go/pull/63));

## v2.3.2 - 2020-04-21

- Upgrade go-ini v1.55.0
  ([#60](https://github.com/editorconfig/editorconfig-core-go/pull/60));
- Build on latest Go
  ([#54](https://github.com/editorconfig/editorconfig-core-go/pull/54));
- Use GitHub action instead of Travis CI
  ([#50](https://github.com/editorconfig/editorconfig-core-go/pull/50));

## v2.3.1 - 2020-03-16

- Use golang/x/mod/semver for semantic versioning checks
  ([#55](https://github.com/editorconfig/editorconfig-core-go/pull/55));
- Enable wsl (WhiteSpace linter)
  ([#56](https://github.com/editorconfig/editorconfig-core-go/pull/56));
- Replace testify dependency with Google's go-cmp
  ([#57](https://github.com/editorconfig/editorconfig-core-go/pull/57));
- Upgrade go-ini to v1.54.0
  ([#58](https://github.com/editorconfig/editorconfig-core-go/pull/58)).

## v2.3.0 - 2020-02-14

- Implement a cached `Parser` to allow getting the definition of many files
  at once without re-reading the `.editorconfig` or parsing the _globbing_
  expression more than once.
  ([#51](https://github.com/editorconfig/editorconfig-core-go/pull/51));
- Run golangci-lint on travis
  ([#26](https://github.com/editorconfig/editorconfig-core-go/pull/26)).

## v2.2.2 - 2020-01-19

- Bump core test to master
  ([#42](https://github.com/editorconfig/editorconfig-core-go/pull/42));
- Bugfix error mangled when reading a file which could create a panic
  ([#47](https://github.com/editorconfig/editorconfig-core-go/pull/47));
- Bugfix INI file generated would not show the correct value
  ([#46](https://github.com/editorconfig/editorconfig-core-go/pull/46)).

## v2.2.1 - 2019-11-10

- Implement pre 0.9.0 behavior
  ([#39](https://github.com/editorconfig/editorconfig-core-go/pull/39));
- Fix values inheritance (regression)
  ([#43](https://github.com/editorconfig/editorconfig-core-go/pull/43)).

## v2.2.0 - 2019-10-12

- Allow parsing from an `io.Reader`, effectively deprecating `ParseBytes`
  by [@mvdan](https://github.com/mvdan)
  ([#32](https://github.com/editorconfig/editorconfig-core-go/pull/32));
- Add support for the special `unset` value by [@greut](https://github.com/greut)
  ([#19](https://github.com/editorconfig/editorconfig-core-go/pull/19));
- Skip values, properties or section that are considered too long
  ([#35](https://github.com/editorconfig/editorconfig-core-go/pull/35));
- Clean up and documentation work by [@mstruebing](https://github.com/mstruebing/)
  ([#23](https://github.com/editorconfig/editorconfig-core-go/pull/23),
  [#24](https://github.com/editorconfig/editorconfig-core-go/pull/24)).

## v2.1.1 - 2019-08-18

- Fix a small path bug
  ([#17](https://github.com/editorconfig/editorconfig-core-go/issues/17),
  [#18](https://github.com/editorconfig/editorconfig-core-go/pull/18)).

## v2.1.0 - 2019-08-10

- This package is now *way* more compliant with the Editorconfig definition
  thanks to a refactor work made by [@greut](https://github.com/greut)
  ([#15](https://github.com/editorconfig/editorconfig-core-go/pull/15)).

## v2.0.0 - 2019-07-14

- This project now uses [Go Modules](https://blog.golang.org/using-go-modules)
  ([#14](https://github.com/editorconfig/editorconfig-core-go/pull/14));
- The import path has been changed from `gopkg.in/editorconfig/editorconfig-core-go.v1`
  to `github.com/editorconfig/editorconfig-core-go/v2`.
