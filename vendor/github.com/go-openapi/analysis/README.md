# OpenAPI initiative analysis

[![Build Status](https://travis-ci.org/go-openapi/analysis.svg?branch=master)](https://travis-ci.org/go-openapi/analysis)
[![Build status](https://ci.appveyor.com/api/projects/status/x377t5o9ennm847o/branch/master?svg=true)](https://ci.appveyor.com/project/casualjim/go-openapi/analysis/branch/master)
[![codecov](https://codecov.io/gh/go-openapi/analysis/branch/master/graph/badge.svg)](https://codecov.io/gh/go-openapi/analysis)
[![Slack Status](https://slackin.goswagger.io/badge.svg)](https://slackin.goswagger.io)
[![license](http://img.shields.io/badge/license-Apache%20v2-orange.svg)](https://raw.githubusercontent.com/go-openapi/analysis/master/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-openapi/analysis.svg)](https://pkg.go.dev/github.com/go-openapi/analysis)
[![GolangCI](https://golangci.com/badges/github.com/go-openapi/analysis.svg)](https://golangci.com)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-openapi/analysis)](https://goreportcard.com/report/github.com/go-openapi/analysis)


A foundational library to analyze an OAI specification document for easier reasoning about the content.

## What's inside?

* A analyzer providing methods to walk the functional content of a specification
* A spec flattener producing a self-contained document bundle, while preserving `$ref`s
* A spec merger ("mixin") to merge several spec documents into a primary spec
* A spec "fixer" ensuring that response descriptions are non empty

[Documentation](https://godoc.org/github.com/go-openapi/analysis)

## FAQ

* Does this library support OpenAPI 3?

> No.
> This package currently only supports OpenAPI 2.0 (aka Swagger 2.0).
> There is no plan to make it evolve toward supporting OpenAPI 3.x.
> This [discussion thread](https://github.com/go-openapi/spec/issues/21) relates the full story.
>
