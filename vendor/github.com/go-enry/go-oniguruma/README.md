# go-oniguruma ![Test](https://github.com/go-enry/go-oniguruma/workflows/Test/badge.svg)

This repository is a fork of [moovweb/rubex](https://github.com/moovweb/rubex/tree/go1) - a simple regular expression library (based on [oniguruma](https://github.com/kkos/oniguruma)) that supports Ruby's regex syntax.

The _rubex_ was originally created by Zhigang Chen (zhigang.chen@moovweb.com or zhigangc@gmail.com). It implements all the public functions of Go's Regexp package, except LiteralPrefix.

By the benchmark tests in regexp, the library is 40% to 10X faster than Regexp on all but one test. Unlike Go's regexp, this library supports named capture groups and also allow `"\\1"` and `"\\k<name>"` in replacement strings.
The library calls the _oniguruma_ regex library for regex pattern searching. All replacement code is done in Go.

Install
-------

```sh
# linux (debian/ubuntu/...)
sudo apt-get install libonig-dev

# osx (homebrew)
brew install oniguruma

go get github.com/go-enry/go-oniguruma
```


License
-------
Apache License Version 2.0, see [LICENSE](LICENSE)
