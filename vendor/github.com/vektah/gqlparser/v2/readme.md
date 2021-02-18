gqlparser [![CircleCI](https://badgen.net/circleci/github/vektah/gqlparser/master)](https://circleci.com/gh/vektah/gqlparser) [![Go Report Card](https://goreportcard.com/badge/github.com/vektah/gqlparser/v2)](https://goreportcard.com/report/github.com/vektah/gqlparser/v2) [![Coverage Status](https://badgen.net/coveralls/c/github/vektah/gqlparser)](https://coveralls.io/github/vektah/gqlparser?branch=master)
===

This is a parser for graphql, written to mirror the graphql-js reference implementation as closely while remaining idiomatic and easy to use.

spec target: June 2018 (Schema definition language, block strings as descriptions, error paths & extension)

This parser is used by [gqlgen](https://github.com/99designs/gqlgen), and it should be reasonablly stable.

Guiding principles:

 - maintainability: It should be easy to stay up to date with the spec
 - well tested: It shouldnt need a graphql server to validate itself. Changes to this repo should be self contained.
 - server agnostic: It should be usable by any of the graphql server implementations, and any graphql client tooling.
 - idiomatic & stable api: It should follow go best practices, especially around forwards compatibility.
 - fast: Where it doesnt impact on the above it should be fast. Avoid unnecessary allocs in hot paths.
 - close to reference: Where it doesnt impact on the above, it should stay close to the [graphql/graphql-js](https://github.com/graphql/graphql-js) reference implementation.
