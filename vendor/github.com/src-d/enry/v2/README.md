# enry [![GoDoc](https://godoc.org/github.com/src-d/enry?status.svg)](https://godoc.org/github.com/src-d/enry) [![Build Status](https://travis-ci.com/src-d/enry.svg?branch=master)](https://travis-ci.com/src-d/enry) [![codecov](https://codecov.io/gh/src-d/enry/branch/master/graph/badge.svg)](https://codecov.io/gh/src-d/enry)

File programming language detector and toolbox to ignore binary or vendored files. *enry*, started as a port to _Go_ of the original [linguist](https://github.com/github/linguist) _Ruby_ library, that has an improved *2x performance*.

* [Installation](#installation)
* [Examples](#examples)
* [CLI](#cli)
* [Java bindings](#java-bindings)
* [Python bindings](#python-bindings)
* [Divergences from linguist](#divergences-from-linguist)
* [Benchmarks](#benchmarks)
* [Why Enry?](#why-enry)
* [Development](#development)
    * [Sync with github/linguist upstream](#sync-with-githublinguist-upstream)
* [Misc](#misc)
    * [Benchmark](#benchmark)
    * [Faster regexp engine (optional)](#faster-regexp-engine-optional)
* [License](#license)

Installation
------------

The recommended way to install enry is to either [download a release](https://github.com/src-d/enry/releases) or

```
go get github.com/src-d/enry/cmd/enry
```

This project is now part of [source{d} Engine](https://sourced.tech/engine),
which provides the simplest way to get started with a single command.
Visit [sourced.tech/engine](https://sourced.tech/engine) for more information.


Examples
------------

```go
lang, safe := enry.GetLanguageByExtension("foo.go")
fmt.Println(lang, safe)
// result: Go true

lang, safe := enry.GetLanguageByContent("foo.m", []byte("<matlab-code>"))
fmt.Println(lang, safe)
// result: Matlab true

lang, safe := enry.GetLanguageByContent("bar.m", []byte("<objective-c-code>"))
fmt.Println(lang, safe)
// result: Objective-C true

// all strategies together
lang := enry.GetLanguage("foo.cpp", []byte("<cpp-code>"))
// result: C++ true
```

Note that the returned boolean value `safe` is set either to `true`, if there is only one possible language detected, or to `false` otherwise.

To get a list of possible languages for a given file, you can use the plural version of the detecting functions.

```go
langs := enry.GetLanguages("foo.h",  []byte("<cpp-code>"))
// result: []string{"C", "C++", "Objective-C}

langs := enry.GetLanguagesByExtension("foo.asc", []byte("<content>"), nil)
// result: []string{"AGS Script", "AsciiDoc", "Public Key"}

langs := enry.GetLanguagesByFilename("Gemfile", []byte("<content>"), []string{})
// result: []string{"Ruby"}
```


CLI
------------

You can use enry as a command,

```bash
$ enry --help
enry v2.0.0 build: 05-08-2019_20_40_35 commit: 6ccf0b6, based on linguist commit: e456098
enry, A simple (and faster) implementation of github/linguist
usage: enry [-mode=(file|line|byte)] [-prog] <path>
        enry [-mode=(file|line|byte)] [-prog] [-json] [-breakdown] <path>
        enry [-mode=(file|line|byte)] [-prog] [-json] [-breakdown]
        enry [-version]
```

and on repository root, it'll return an output similar to *linguist*'s output,

```bash
$ enry
97.71%	Go
1.60%	C
0.31%	Shell
0.22%	Java
0.07%	Ruby
0.05%	Makefile
0.04%	Scala
0.01%	Gnuplot
```

but not only the output; its flags are also the same as *linguist*'s ones,

```bash
$ enry --breakdown
97.71%	Go
1.60%	C
0.31%	Shell
0.22%	Java
0.07%	Ruby
0.05%	Makefile
0.04%	Scala
0.01%	Gnuplot

Scala
java/build.sbt
java/project/plugins.sbt

Java
java/src/main/java/tech/sourced/enry/Enry.java
java/src/main/java/tech/sourced/enry/GoUtils.java
java/src/main/java/tech/sourced/enry/Guess.java
java/src/test/java/tech/sourced/enry/EnryTest.java

Makefile
Makefile
java/Makefile

Go
benchmark_test.go
```

even the JSON flag,

```bash
$ enry --json | jq .
{
  "C": [
    "internal/tokenizer/flex/lex.linguist_yy.c",
    "internal/tokenizer/flex/lex.linguist_yy.h",
    "internal/tokenizer/flex/linguist.h",
    "python/_c_enry.c",
    "python/enry.c"
  ],
  "Gnuplot": [
    "benchmarks/plot-histogram.gp"
  ],
  "Go": [
    "benchmark_test.go",
```

Note that enry's CLI **_doesn't need a git repository to work_**, which is intentionally different from the linguist.

## Java bindings


Generated Java bindings using a C shared library and JNI are available under [`java`](https://github.com/src-d/enry/blob/master/java) and published on Maven at [tech.sourced:enry-java](https://mvnrepository.com/artifact/tech.sourced/enry-java) for macOS and linux.


## Python bindings
Generated Python bindings using a C shared library and cffi are not available yet and are WIP under [src-d/enry#154](https://github.com/src-d/enry/issues/154).

Divergences from linguist
------------

The `enry` library is based on the data from `github/linguist` version **v7.5.1**.

As opposed to linguist, `enry` [CLI tool](#cli) does *not* require a full Git repository in the filesystem in order to report languages.

Parsing [linguist/samples](https://github.com/github/linguist/tree/master/samples) the following `enry` results are different from linguist:

* [Heuristics for ".es" extension](https://github.com/github/linguist/blob/e761f9b013e5b61161481fcb898b59721ee40e3d/lib/linguist/heuristics.yml#L103) in JavaScript could not be parsed, due to unsupported backreference in RE2 regexp engine.

* [Heuristics for ".rno" extension](https://github.com/github/linguist/blob/3a1bd3c3d3e741a8aaec4704f782e06f5cd2a00d/lib/linguist/heuristics.yml#L365) in RUNOFF could not be parsed, due to unsupported lookahead in RE2 regexp engine.

* As of [Linguist v5.3.2](https://github.com/github/linguist/releases/tag/v5.3.2) it is using [flex-based scanner in C for tokenization](https://github.com/github/linguist/pull/3846). Enry still uses [extract_token](https://github.com/github/linguist/pull/3846/files#diff-d5179df0b71620e3fac4535cd1368d15L60) regex-based algorithm. See [#193](https://github.com/src-d/enry/issues/193).

* Bayesian classifier can't distinguish "SQL" from "PLpgSQL. See [#194](https://github.com/src-d/enry/issues/194).

* Detection of [generated files](https://github.com/github/linguist/blob/bf95666fc15e49d556f2def4d0a85338423c25f3/lib/linguist/generated.rb#L53) is not supported yet.
 (Thus they are not excluded from CLI output). See [#213](https://github.com/src-d/enry/issues/213).

* XML detection strategy is not implemented. See [#192](https://github.com/src-d/enry/issues/192).

* Overriding languages and types though `.gitattributes` is not yet supported. See [#18](https://github.com/src-d/enry/issues/18).

* `enry` CLI output does NOT exclude `.gitignore`ed files and git submodules, as linguist does

In all the cases above that have an issue number - we plan to update enry to match Linguist behavior.


Benchmarks
------------

Enry's language detection has been compared with Linguist's one. In order to do that, Linguist's project directory [*linguist/samples*](https://github.com/github/linguist/tree/master/samples) was used as a set of files to run benchmarks against.

We got these results:

![histogram](benchmarks/histogram/distribution.png)

The histogram shows the number of files detected (y-axis) per time interval bucket (x-axis). As one can see, most of the files were detected faster by enry.

We found few cases where enry turns slower than linguist due to
Go regexp engine being slower than Ruby's, based on [oniguruma](https://github.com/kkos/oniguruma) library, written in C.

See [instructions](#misc) for running enry with oniguruma.


Why Enry?
------------

In the movie [My Fair Lady](https://en.wikipedia.org/wiki/My_Fair_Lady), [Professor Henry Higgins](http://www.imdb.com/character/ch0011719/?ref_=tt_cl_t2) is one of the main characters. Henry is a linguist and at the very beginning of the movie enjoys guessing the origin of people based on their accent.

"Enry Iggins" is how [Eliza Doolittle](http://www.imdb.com/character/ch0011720/?ref_=tt_cl_t1), [pronounces](https://www.youtube.com/watch?v=pwNKyTktDIE) the name of the Professor during the first half of the movie.

## Development

To build enry's CLI run:

    make build

this will generate a binary in the project's root directory called `enry`.

To run the tests:

    make test


### Sync with github/linguist upstream

*enry* re-uses parts of the original [github/linguist](https://github.com/github/linguist) to generate internal data structures.
In order to update to the latest release of linguist do:

```bash
$ git clone https://github.com/github/linguist.git .linguist
$ cd .linguist; git checkout <release-tag>; cd ..

# put the new release's commit sha in the generator_test.go (to re-generate .gold test fixtures)
# https://github.com/src-d/enry/blob/13d3d66d37a87f23a013246a1b0678c9ee3d524b/internal/code-generator/generator/generator_test.go#L18

$ make code-generate
```

To stay in sync, enry needs to be updated when a new release of the linguist includes changes to any of the following files:

* [languages.yml](https://github.com/github/linguist/blob/master/lib/linguist/languages.yml)
* [heuristics.yml](https://github.com/github/linguist/blob/master/lib/linguist/heuristics.yml)
* [vendor.yml](https://github.com/github/linguist/blob/master/lib/linguist/vendor.yml)
* [documentation.yml](https://github.com/github/linguist/blob/master/lib/linguist/documentation.yml)

There is no automation for detecting the changes in the linguist project, so this process above has to be done manually from time to time.

When submitting a pull request syncing up to a new release, please make sure it only contains the changes in
the generated files (in [data](https://github.com/src-d/enry/blob/master/data) subdirectory).

Separating all the necessary "manual" code changes to a different PR that includes some background description and an update to the documentation on ["divergences from linguist"](##divergences-from-linguist) is very much appreciated as it simplifies the maintenance (review/release notes/etc).



## Misc

<details>

### Benchmark

All benchmark scripts are in [*benchmarks*](https://github.com/src-d/enry/blob/master/benchmarks) directory.


#### Dependencies
As benchmarks depend on Ruby and Github-Linguist gem make sure you have:
 - Ruby (e.g using [`rbenv`](https://github.com/rbenv/rbenv)), [`bundler`](https://bundler.io/) installed
 - Docker
 - [native dependencies](https://github.com/github/linguist/#dependencies) installed
 - Build the gem `cd .linguist && bundle install && rake build_gem && cd -`
 - Install it `gem install --no-rdoc --no-ri --local .linguist/github-linguist-*.gem`


#### Quick benchmark
To run quicker benchmarks you can either:

    make benchmarks

to get average times for the main detection function and strategies for the whole samples set or:

    make benchmarks-samples

if you want to see measures per sample file.


#### Full benchmark
If you want to reproduce the same benchmarks as reported above:
 - Make sure all [dependencies](#benchmark-dependencies) are installed
 - Install [gnuplot](http://gnuplot.info) (in order to plot the histogram)
 - Run `ENRY_TEST_REPO="$PWD/.linguist" benchmarks/run.sh` (takes ~15h)

It will run the benchmarks for enry and linguist, parse the output, create csv files and plot the histogram.

### Faster regexp engine (optional)

[Oniguruma](https://github.com/kkos/oniguruma) is CRuby's regular expression engine.
It is very fast and performs better than the one built into Go runtime. *enry* supports swapping
between those two engines thanks to [rubex](https://github.com/moovweb/rubex) project.
The typical overall speedup from using Oniguruma is 1.5-2x. However, it requires CGo and the external shared library.
On macOS with [Homebrew](https://brew.sh/), it is:

```
brew install oniguruma
```

On Ubuntu, it is

```
sudo apt install libonig-dev
```

To build enry with Oniguruma regexps use the `oniguruma` build tag

```
go get -v -t --tags oniguruma ./...
```

and then rebuild the project.

</details>


License
------------

Apache License, Version 2.0. See [LICENSE](LICENSE)
