# Change log

## v2.2.1 - 2019-11-10

- Implement pre 0.9.0 behavior
  ([#39](https://github.com/editorconfig/editorconfig-core-go/pull/39));
- Fix values inheritance (regression)
  ([#43](https://github.com/editorconfig/editorconfig-core-go/pull/43));

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
